// Package session models a connected user independently of how they
// connected, so one state machine can serve any front end.
package session

import "time"

// Client is how the user connected.
type Client string

// Known clients.
const (
	ClientSSH     Client = "ssh"
	ClientDiscord Client = "discord"
	ClientLocal   Client = "local"
)

// UserSession is identity and provenance only. How the user is rendered
// belongs to the front end; the ssh.Session, connection, and PTY stay with
// the adapter that built this.
type UserSession struct {
	ID       string
	Username string
	Client   Client

	Credential Credential

	RemoteAddr  string
	ConnectedAt time.Time
}

// Credential is what the user proved to get here. For SSH that is a key; for
// another front end it might be an identity provider subject. Accounts are
// looked up by ID; Material is whatever was presented, kept so linking an
// account can record it.
type Credential struct {
	ID       string
	Material string
}

// Name is the username, or "stranger" when the client did not supply one.
func (u UserSession) Name() string {
	if u.Username == "" {
		return "stranger"
	}

	return u.Username
}
