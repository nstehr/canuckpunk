package narratives_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nstehr/canuckpunk/narratives"
)

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// The whole point: prose is re-read, so a writer can revise against a running
// server.
func TestEditsAreVisibleWithoutRestarting(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "onboarding/welcome.md", "first draft")

	store, err := narratives.FromDir(dir)
	if err != nil {
		t.Fatalf("FromDir: %v", err)
	}

	got, err := store.Read("onboarding/welcome.md")
	if err != nil || got != "first draft" {
		t.Fatalf("Read = %q, %v", got, err)
	}

	writeFile(t, dir, "onboarding/welcome.md", "second draft")

	if got, _ := store.Read("onboarding/welcome.md"); got != "second draft" {
		t.Errorf("Read = %q, want the edited text — prose is being cached", got)
	}
}

// A binary on its own still has to run, so the embedded copy is the fallback.
func TestOpenFallsBackToTheEmbeddedCopy(t *testing.T) {
	store := narratives.Open(filepath.Join(t.TempDir(), "does-not-exist"))

	if store.Dir() != "" {
		t.Errorf("Dir = %q, want empty for the embedded copy", store.Dir())
	}

	got, err := store.Read("onboarding/welcome.md")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if !strings.Contains(got, "Canuckpunk") {
		t.Errorf("embedded welcome looks wrong: %q", got[:min(60, len(got))])
	}
}

func TestOpenPrefersTheDirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "onboarding/welcome.md", "from disk")

	store := narratives.Open(dir)
	if store.Dir() != dir {
		t.Errorf("Dir = %q, want %q", store.Dir(), dir)
	}

	if got, _ := store.Read("onboarding/welcome.md"); got != "from disk" {
		t.Errorf("Read = %q, want the file on disk", got)
	}
}

// Names are joined onto a directory, so a traversal must not escape it.
func TestReadCannotEscapeTheDirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "onboarding/welcome.md", "inside")

	secret := filepath.Join(filepath.Dir(dir), "secret.md")
	if err := os.WriteFile(secret, []byte("outside"), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	store, err := narratives.FromDir(dir)
	if err != nil {
		t.Fatalf("FromDir: %v", err)
	}

	for _, name := range []string{
		"../secret.md",
		"onboarding/../../secret.md",
		secret,
	} {
		if got, err := store.Read(name); err == nil {
			t.Errorf("Read(%q) escaped the tree and returned %q", name, got)
		}
	}
}

func TestMissingNarrativeIsAnError(t *testing.T) {
	store := narratives.Embedded()

	if _, err := store.Read("onboarding/nope.md"); err == nil {
		t.Error("missing narrative did not error")
	}
}
