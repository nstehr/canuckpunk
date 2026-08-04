package discord

import "strings"

// writer is what a state writes into. Implementing state.Hinter is how a state
// says what kind of answer it wants next.
type writer struct {
	buf  strings.Builder
	hint string
}

func (w *writer) Write(p []byte) (int, error) { return w.buf.Write(p) }

func (w *writer) Hint(kind string) { w.hint = kind }

func (w *writer) text() string { return w.buf.String() }
