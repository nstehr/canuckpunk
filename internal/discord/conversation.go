package discord

import (
	"sync"
	"time"

	"github.com/nstehr/canuckpunk/internal/menu"
	"github.com/nstehr/canuckpunk/internal/session"
	"github.com/nstehr/canuckpunk/internal/state"
)

// A DM is the Discord equivalent of an SSH connection, except nothing tells us
// when it ends. Conversations are therefore evicted on age rather than on
// disconnect, and a player whose conversation has gone simply starts again.
const conversationTTL = 2 * time.Hour

// conversation is one player's position in the flow. The mutex matters:
// Discord delivers interactions concurrently, and state.Machine is documented
// as single-goroutine.
type conversation struct {
	mu      sync.Mutex
	machine *state.Machine
	session session.UserSession
	touched time.Time

	// What the player is currently being offered. Discord messages outlive
	// the screen they were sent for, so a button is only honoured while its
	// choice is still on the table.
	offered menu.Set
}

// offers reports whether a choice is one the conversation is currently
// waiting on.
func (c *conversation) offers(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, ok := c.offered.ByID(id)

	return ok
}

// conversations is the set of players currently mid-flow, keyed by Discord
// user id. It holds no Discord types so it can be tested on its own.
type conversations struct {
	mu    sync.Mutex
	items map[string]*conversation
	ttl   time.Duration
	now   func() time.Time
	start func() *state.Machine
}

func newConversations(ttl time.Duration, start func() *state.Machine) *conversations {
	return &conversations{
		items: map[string]*conversation{},
		ttl:   ttl,
		now:   time.Now,
		start: start,
	}
}

// get returns the player's conversation, beginning a new one when there is
// none. A stale button click therefore restarts the flow instead of failing.
func (cs *conversations) get(id string, us session.UserSession) *conversation {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.sweepLocked()

	c, ok := cs.items[id]
	if !ok {
		c = &conversation{machine: cs.start(), session: us}
		cs.items[id] = c
	}

	c.touched = cs.now()

	return c
}

// fresh discards any conversation in progress and starts over.
func (cs *conversations) fresh(id string, us session.UserSession) *conversation {
	cs.drop(id)

	return cs.get(id, us)
}

func (cs *conversations) drop(id string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	delete(cs.items, id)
}

func (cs *conversations) len() int {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	return len(cs.items)
}

func (cs *conversations) sweepLocked() {
	cutoff := cs.now().Add(-cs.ttl)
	for id, c := range cs.items {
		if c.touched.Before(cutoff) {
			delete(cs.items, id)
		}
	}
}
