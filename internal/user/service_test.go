package user_test

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"github.com/nstehr/canuckpunk/data/migrations"
	"github.com/nstehr/canuckpunk/internal/user"
)

const (
	testFingerprint = "SHA256:abc"
	testSurveyor    = "surveyor"
	testKey         = "ssh-ed25519 AAAA"
)

// newDB gives each test a migrated database of its own, exercising the real
// schema rather than a hand-written copy of it.
func newDB(t *testing.T) *sql.DB {
	t.Helper()

	database, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	goose.SetBaseFS(migrations.Embed)
	goose.SetLogger(goose.NopLogger())

	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("dialect: %v", err)
	}

	if err := goose.Up(database, "."); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return database
}

func TestUnknownCredentialHasNoAccounts(t *testing.T) {
	svc := user.NewService(newDB(t))

	got, err := svc.ForCredential(t.Context(), "SHA256:nobody")
	if err != nil {
		t.Fatalf("ForCredential: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("got %v, want none", got)
	}
}

func TestCreateThenLookUpByCredential(t *testing.T) {
	svc := user.NewService(newDB(t))
	fp := testFingerprint

	created, err := svc.Create(t.Context(), testSurveyor, fp, testKey)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.ForCredential(t.Context(), fp)
	if err != nil {
		t.Fatalf("ForCredential: %v", err)
	}

	if len(got) != 1 || got[0].Username != testSurveyor || got[0].ID != created.ID {
		t.Errorf("got %v, want the created account", got)
	}
}

// One key may hold several characters, and the menu order must be stable.
func TestOneCredentialManyAccounts(t *testing.T) {
	svc := user.NewService(newDB(t))
	fp := testFingerprint

	for _, name := range []string{testSurveyor, "auditor", "inspector"} {
		if _, err := svc.Create(t.Context(), name, fp, "ssh-ed25519 AAAA"); err != nil {
			t.Fatalf("Create(%s): %v", name, err)
		}
	}

	got, err := svc.ForCredential(t.Context(), fp)
	if err != nil {
		t.Fatalf("ForCredential: %v", err)
	}

	want := []string{testSurveyor, "auditor", "inspector"}
	if len(got) != len(want) {
		t.Fatalf("got %d accounts, want %d", len(got), len(want))
	}

	for i, w := range want {
		if got[i].Username != w {
			t.Errorf("account %d = %q, want %q", i, got[i].Username, w)
		}
	}
}

// Several keys may reach one account, so a player can sign in from any of
// their machines.
func TestManyCredentialsOneAccount(t *testing.T) {
	svc := user.NewService(newDB(t))

	created, err := svc.Create(t.Context(), testSurveyor, "SHA256:laptop", testKey)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.LinkCredential(t.Context(), created.ID, "SHA256:desktop", testKey+"B"); err != nil {
		t.Fatalf("LinkCredential: %v", err)
	}

	for _, fp := range []string{"SHA256:laptop", "SHA256:desktop"} {
		got, err := svc.ForCredential(t.Context(), fp)
		if err != nil {
			t.Fatalf("ForCredential(%s): %v", fp, err)
		}

		if len(got) != 1 || got[0].ID != created.ID {
			t.Errorf("%s reached %v, want the one account", fp, got)
		}
	}
}

func TestRelinkingTheSameCredentialIsANoop(t *testing.T) {
	svc := user.NewService(newDB(t))
	fp := testFingerprint

	created, err := svc.Create(t.Context(), testSurveyor, fp, testKey)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.LinkCredential(t.Context(), created.ID, fp, "ssh-ed25519 AAAA"); err != nil {
		t.Fatalf("LinkCredential: %v", err)
	}

	got, err := svc.ForCredential(t.Context(), fp)
	if err != nil {
		t.Fatalf("ForCredential: %v", err)
	}

	if len(got) != 1 {
		t.Errorf("got %d accounts, want 1 — re-linking duplicated the row", len(got))
	}
}

func TestDuplicateUsernameIsRejected(t *testing.T) {
	svc := user.NewService(newDB(t))

	if _, err := svc.Create(t.Context(), testSurveyor, "SHA256:a", testKey); err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err := svc.Create(t.Context(), testSurveyor, "SHA256:b", testKey)
	if !errors.Is(err, user.ErrUsernameTaken) {
		t.Fatalf("err = %v, want ErrUsernameTaken", err)
	}

	// The rejected attempt must not have left an orphaned key behind.
	got, err := svc.ForCredential(t.Context(), "SHA256:b")
	if err != nil {
		t.Fatalf("ForCredential: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("failed create left %v behind", got)
	}
}
