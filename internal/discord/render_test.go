package discord

import (
	"strings"
	"testing"

	"github.com/disgoorg/disgo/discord"

	"github.com/nstehr/canuckpunk/internal/menu"
)

// A passage longer than Discord's limit has to arrive whole, in order, and
// without a sentence torn across the seam.
func TestChunkKeepsProseIntact(t *testing.T) {
	para := strings.Repeat("word ", 100) // ~500 chars
	src := strings.TrimSpace(strings.Repeat(para+"\n\n", 12))

	parts := chunk(src)
	if len(parts) < 2 {
		t.Fatalf("expected several messages, got %d", len(parts))
	}

	for i, p := range parts {
		if n := len([]rune(p)); n > maxMessageRunes {
			t.Errorf("message %d is %d runes, over the %d limit", i, n, maxMessageRunes)
		}
	}

	// Every word survives, in order.
	want := strings.Fields(src)
	got := strings.Fields(strings.Join(parts, " "))

	if len(got) != len(want) {
		t.Fatalf("got %d words, want %d — prose was lost", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("word %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// A single paragraph with no break still has to be split somewhere.
func TestChunkSplitsAnUnbrokenParagraph(t *testing.T) {
	src := strings.Repeat("x", maxMessageRunes*2+50)

	parts := chunk(src)
	if len(parts) < 2 {
		t.Fatalf("expected a split, got %d part(s)", len(parts))
	}

	for i, p := range parts {
		if n := len([]rune(p)); n > maxMessageRunes {
			t.Errorf("part %d is %d runes, over the limit", i, n)
		}
	}

	if joined := strings.Join(parts, ""); len(joined) != len(src) {
		t.Errorf("joined length %d, want %d", len(joined), len(src))
	}
}

func TestChunkOnEmpty(t *testing.T) {
	if got := chunk("   \n\n  "); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

// The id a button carries has to survive the round trip, whatever is in it.
func TestChoiceIDRoundTrip(t *testing.T) {
	for _, id := range []string{
		"create-user",
		"continue:7",
		"weird/id with spaces",
		"unicode-é",
	} {
		custom := choiceCustomID(id)

		if len(custom) > 100 {
			t.Errorf("custom_id for %q is %d chars, over Discord's 100", id, len(custom))
		}

		if got := choiceID(strings.TrimPrefix(custom, choiceRoute)); got != id {
			t.Errorf("round trip of %q gave %q", id, got)
		}

		if got := choiceFromValue(custom); got != id {
			t.Errorf("select round trip of %q gave %q", id, got)
		}
	}
}

func TestComponentsUsesButtonsThenSelect(t *testing.T) {
	set := func(n int) menu.Set {
		out := make(menu.Set, 0, n)
		for i := range n {
			out = append(out, menu.Choice{ID: string(rune('a' + i)), Label: "Option"})
		}

		return out
	}

	if got := components(nil); got != nil {
		t.Error("no choices should render no components")
	}

	rows := components(set(maxButtons))
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}

	if _, ok := rows[0].(discord.ActionRowComponent); !ok {
		t.Errorf("expected an action row, got %T", rows[0])
	}

	// Above a row's worth, a select menu carries them instead.
	rows = components(set(maxButtons + 1))
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
}

// Discord caps a select at 25; going over must not send an invalid payload.
func TestComponentsCapsSelect(t *testing.T) {
	many := make(menu.Set, 0, 40)
	for i := range 40 {
		many = append(many, menu.Choice{ID: string(rune(i)), Label: "Option"})
	}

	rows := components(many)
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
}

// Buttons belong under the text that explains them, not on the first of five
// messages the player has to scroll back to.
func TestMessagesPutChoicesOnTheLast(t *testing.T) {
	long := strings.TrimSpace(strings.Repeat(strings.Repeat("word ", 100)+"\n\n", 6))
	s := screen{
		text:    long,
		choices: menu.Set{{ID: "create-user", Label: "Create New User"}},
	}

	msgs := messages(s)
	if len(msgs) < 2 {
		t.Fatalf("expected several messages, got %d", len(msgs))
	}

	for i, m := range msgs[:len(msgs)-1] {
		if len(m.Components) != 0 {
			t.Errorf("message %d carries components", i)
		}
	}

	if len(msgs[len(msgs)-1].Components) == 0 {
		t.Error("the last message has no components")
	}
}

func TestMessagesOnEmptyScreen(t *testing.T) {
	if got := messages(screen{}); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}
