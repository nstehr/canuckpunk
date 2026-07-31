// Command canuckpunk serves the terminal UI over SSH.
package main

import (
	"context"
	"errors"
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

	"github.com/nstehr/canuckpunk/internal/session"
	"github.com/nstehr/canuckpunk/internal/state"
	"github.com/nstehr/canuckpunk/internal/ui"
)

const (
	host = "localhost"
	port = "1867"
)

func main() {
	s, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort(host, port)),
		wish.WithHostKeyPath(".ssh/id_ed25519"),
		wish.WithMiddleware(
			bubbletea.Middleware(teaHandler),
			activeterm.Middleware(), // Bubble Tea apps usually require a PTY.
			logging.Middleware(),
		),
	)
	if err != nil {
		log.Error("Could not start server", "error", err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	log.Info("Starting SSH server", "host", host, "port", port)
	go func() {
		if err = s.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Error("Could not start server", "error", err)
			done <- nil
		}
	}()

	<-done
	log.Info("Stopping SSH server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer func() { cancel() }()
	if err := s.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		log.Error("Could not stop server", "error", err)
	}
}

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	pty, _, _ := s.Pty()
	us := newUserSession(s)
	ctx := session.NewContext(s.Context(), us)
	sm := state.New(state.Welcome)
	opts := ui.Options{
		Display: ui.Display{
			Term:   pty.Term,
			Width:  pty.Window.Width,
			Height: pty.Window.Height,
		},
	}

	return ui.New(ctx, us, sm, opts), []tea.ProgramOption{}
}

// The only place that knows about ssh.Session; another front end needs just
// its own adapter.
func newUserSession(s ssh.Session) session.UserSession {
	return session.UserSession{
		ID:          s.Context().SessionID(),
		Username:    s.User(),
		Client:      session.ClientSSH,
		RemoteAddr:  s.RemoteAddr().String(),
		ConnectedAt: time.Now(),
	}
}
