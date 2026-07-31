// Package menu is a set of labelled choices a player can take, either by
// selecting one or by typing its label. It carries no game vocabulary of its
// own: the labels and actions come from whoever builds the Set.
package menu

import (
	"context"
	"strings"
)

// Action is what taking a Choice does. Values are defined by the caller, so
// this package never needs to know what any of them mean.
type Action string

// Choice is one option.
//
// ID is the stable selector: a front end with real controls sends it back
// untouched, so rewording a Label never breaks a link. Label is display text,
// and doubles as what a text front end matches on. Value carries whatever the
// action needs — a record id, a direction, a room.
type Choice struct {
	ID     string
	Label  string
	Action Action
	Value  string
}

// Set is the choices offered on one screen.
type Set []Choice

// Resolve takes whatever a front end sends back: an ID from a control, or a
// label a player typed. Front ends that have real controls should send the ID.
func (s Set) Resolve(input string) (Choice, bool) {
	if c, ok := s.ByID(strings.TrimSpace(input)); ok {
		return c, true
	}

	return s.Match(input)
}

// ByID selects exactly, with no normalising: an ID is machine-supplied.
func (s Set) ByID(id string) (Choice, bool) {
	if id == "" {
		return Choice{}, false
	}

	for _, c := range s {
		if c.ID == id {
			return c, true
		}
	}

	return Choice{}, false
}

// Match finds the choice a player typed. Matching is case-insensitive and
// ignores surrounding space, because the label is meant to be retyped by hand.
func (s Set) Match(input string) (Choice, bool) {
	want := strings.ToLower(strings.TrimSpace(input))
	if want == "" {
		return Choice{}, false
	}

	for _, c := range s {
		if strings.ToLower(c.Label) == want {
			return c, true
		}
	}

	return Choice{}, false
}

// Labels lists the choices in order, for a front end that only renders text.
func (s Set) Labels() []string {
	out := make([]string, 0, len(s))
	for _, c := range s {
		out = append(out, c.Label)
	}

	return out
}

// Source supplies the choices for a screen. A front end takes one of these
// instead of any particular game package.
type Source interface {
	Choices(ctx context.Context) (Set, error)
}
