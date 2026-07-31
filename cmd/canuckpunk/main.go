// Command canuckpunk serves the terminal UI over SSH.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/log/v2"
	"charm.land/ssh"
	"charm.land/wish/v2"
	"charm.land/wish/v2/activeterm"
	"charm.land/wish/v2/bubbletea"
	"charm.land/wish/v2/logging"
	gossh "golang.org/x/crypto/ssh"
	_ "modernc.org/sqlite"

	"github.com/nstehr/canuckpunk/internal/onboarding"
	"github.com/nstehr/canuckpunk/internal/session"
	"github.com/nstehr/canuckpunk/internal/state"
	"github.com/nstehr/canuckpunk/internal/tui"
	"github.com/nstehr/canuckpunk/internal/user"
	"github.com/nstehr/canuckpunk/narratives"
)

const (
	host           = "localhost"
	port           = "1867"
	defaultDBPath  = "canuckpunk.db"
	shutdownWindow = 30 * time.Second
)

// Env overrides, so prose and data can live outside the working directory.
const (
	envDBPath     = "CANUCKPUNK_DB"
	envNarratives = "CANUCKPUNK_NARRATIVES"
)

func main() {
	if err := run(); err != nil {
		log.Error("canuckpunk failed to start", "error", err)
		os.Exit(1)
	}
}

func run() error {
	database, err := openDatabase(context.Background())
	if err != nil {
		return err
	}

	defer func() { _ = database.Close() }()

	prose, err := openNarratives()
	if err != nil {
		return err
	}

	users := user.NewService(database)

	s, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort(host, port)),
		wish.WithHostKeyPath(".ssh/id_ed25519"),
		// Every key is accepted: the key is an identity to look up, not a
		// credential to check against an allow list.
		wish.WithPublicKeyAuth(func(ssh.Context, ssh.PublicKey) bool { return true }),
		wish.WithMiddleware(
			bubbletea.Middleware(teaHandler(users, prose)),
			activeterm.Middleware(), // Bubble Tea apps usually require a PTY.
			logging.Middleware(),
		),
	)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	log.Info("Starting SSH server", "host", host, "port", port)
	// A listen failure has to reach the caller, so it travels back here rather
	// than only being logged from the goroutine.
	serveErr := make(chan error, 1)

	go func() {
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			serveErr <- fmt.Errorf("serve: %w", err)
			done <- nil

			return
		}

		serveErr <- nil
	}()

	<-done
	log.Info("Stopping SSH server")

	ctx, cancel := context.WithTimeout(context.Background(), shutdownWindow)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		return fmt.Errorf("stop server: %w", err)
	}

	select {
	case err := <-serveErr:
		return err
	default:
		return nil
	}
}

func openDatabase(ctx context.Context) (*sql.DB, error) {
	path := os.Getenv(envDBPath)
	if path == "" {
		path = defaultDBPath
	}

	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}

	if err := database.Ping(); err != nil {
		_ = database.Close()

		return nil, fmt.Errorf("reach database %s: %w", path, err)
	}

	if err := ensureSchema(ctx, database); err != nil {
		_ = database.Close()

		return nil, fmt.Errorf("database %s: %w", path, err)
	}

	return database, nil
}

// Prose is read from disk at request time, so a missing file would otherwise
// only surface when a player reached that screen.
func openNarratives() (*narratives.Store, error) {
	dir := os.Getenv(envNarratives)
	if dir == "" {
		dir = narratives.DefaultDir
	}

	prose := narratives.Open(dir)
	if err := prose.Check(onboarding.RequiredNarratives()...); err != nil {
		return nil, fmt.Errorf("narratives: %w", err)
	}

	if prose.Dir() == "" {
		log.Info("Reading narratives built into the binary", "looked_in", dir)
	} else {
		log.Info("Reading narratives from disk", "dir", prose.Dir())
	}

	return prose, nil
}

// SQLite creates an empty file on open and Ping succeeds against it, so a
// database that was never migrated looks healthy at startup and only fails
// once a player is already connected. Check for the schema instead.
func ensureSchema(ctx context.Context, database *sql.DB) error {
	const q = `SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'users'`

	var tables int
	if err := database.QueryRowContext(ctx, q).Scan(&tables); err != nil {
		return fmt.Errorf("check schema: %w", err)
	}

	if tables == 0 {
		return errors.New("no schema found: run `make migrate` first")
	}

	return nil
}

func teaHandler(users *user.Service, prose onboarding.Narratives) bubbletea.Handler {
	return func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
		pty, _, _ := s.Pty()
		us := newUserSession(s)
		ctx := session.NewContext(s.Context(), us)
		sm := state.New(onboarding.Start(users, prose))
		opts := tui.Options{
			Display: tui.Display{
				Term:   pty.Term,
				Width:  pty.Window.Width,
				Height: pty.Window.Height,
			},
			Choices: onboarding.NewChooser(users),
		}

		return tui.New(ctx, us, sm, opts), []tea.ProgramOption{}
	}
}

// The boundary: everything downstream sees a session.UserSession, so another
// front end needs only its own adapter.
func newUserSession(s ssh.Session) session.UserSession {
	us := session.UserSession{
		ID:          s.Context().SessionID(),
		Username:    s.User(),
		Client:      session.ClientSSH,
		RemoteAddr:  s.RemoteAddr().String(),
		ConnectedAt: time.Now(),
	}

	// The one place a credential is known to be an SSH key.
	if key := s.PublicKey(); key != nil {
		us.Credential = session.Credential{
			ID:       gossh.FingerprintSHA256(key),
			Material: string(gossh.MarshalAuthorizedKey(key)),
		}
	}

	return us
}
