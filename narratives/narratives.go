// Package narratives holds the game's prose. It is markdown so the front end
// can render it richly; the state machine only ever passes it through.
package narratives

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// DefaultDir is where a Store looks before falling back to the embedded copy.
const DefaultDir = "narratives"

//go:embed all:onboarding
var embedded embed.FS

// Store reads prose. Nothing is cached: a Store backed by a directory picks up
// edits on the next read, so prose can be revised against a running server.
type Store struct {
	fsys fs.FS
	dir  string
}

// Open reads from dir when it exists, and from the copy built into the binary
// otherwise, so a deployment can ship as one file while a writer works against
// the tree. Dir reports which one it got.
func Open(dir string) *Store {
	if dir != "" {
		if s, err := FromDir(dir); err == nil {
			return s
		}
	}

	return Embedded()
}

// FromDir reads from a directory. Reads are confined to it, so a name can
// never reach outside the prose tree.
func FromDir(dir string) (*Store, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open narratives dir %q: %w", dir, err)
	}

	return &Store{fsys: root.FS(), dir: dir}, nil
}

// Embedded reads the copy built into the binary.
func Embedded() *Store {
	return &Store{fsys: embedded}
}

// Dir is the directory being read, or "" when reading the embedded copy.
func (s *Store) Dir() string { return s.dir }

// Check reports every name that cannot be read. Reading prose from disk turns
// a rename into a runtime failure, so call this at startup rather than leaving
// it for the first player to reach that screen. All misses are reported at
// once, so a rename is fixed in one pass.
func (s *Store) Check(names ...string) error {
	var missing []string

	for _, name := range names {
		if _, err := s.Read(name); err != nil {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing narratives: %s", strings.Join(missing, ", "))
	}

	return nil
}

// Read returns the named markdown, e.g. "onboarding/orientation.md".
func (s *Store) Read(name string) (string, error) {
	b, err := fs.ReadFile(s.fsys, name)
	if err != nil {
		return "", fmt.Errorf("read narrative %q: %w", name, err)
	}

	return string(b), nil
}
