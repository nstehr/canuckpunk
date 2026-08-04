// Package user resolves credentials to player accounts. They are stored as SSH
// key fingerprints today; nothing above this package needs to know that.
package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/nstehr/canuckpunk/internal/db"
)

// ErrUsernameTaken means the name is already registered.
var ErrUsernameTaken = errors.New("username already taken")

// User is a player account. Email is optional and blank when unset.
type User struct {
	ID       int64
	Username string
	Email    string
}

// NewAccount is what it takes to register one. Email may be blank; the rest
// may not.
type NewAccount struct {
	Username string
	Email    string

	// CredentialID is what the client proved, and what the account is found
	// by afterwards. Material is kept as presented.
	CredentialID string
	Material     string
}

// NormaliseEmail lowercases and trims, so an address entered as "A@B.CA"
// finds the account created with "a@b.ca". Storage keeps this form.
func NormaliseEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Service reads and writes player accounts.
type Service struct {
	database *sql.DB
	queries  *db.Queries
}

// NewService returns a Service backed by database.
func NewService(database *sql.DB) *Service {
	return &Service{
		database: database,
		queries:  db.New(database),
	}
}

// ForCredential returns every account reachable from one credential. An
// unknown one yields no accounts rather than an error: that is the new-player
// case.
func (s *Service) ForCredential(ctx context.Context, credentialID string) ([]User, error) {
	rows, err := s.queries.ListUsersByFingerprint(ctx, credentialID)
	if err != nil {
		return nil, fmt.Errorf("list users by fingerprint: %w", err)
	}

	return toUsers(rows), nil
}

// ForEmail returns every account registered under an address. It is how a
// client that cannot present the original credential — a web or chat sign-in —
// finds the accounts a person already holds.
//
// An address here is a claim, not a proof. Verify ownership before letting it
// grant access to anything it returns.
func (s *Service) ForEmail(ctx context.Context, email string) ([]User, error) {
	email = NormaliseEmail(email)
	if email == "" {
		return nil, nil
	}

	rows, err := s.queries.ListUsersByEmail(ctx, sql.NullString{String: email, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list users by email: %w", err)
	}

	return toUsers(rows), nil
}

func toUsers(rows []db.User) []User {
	users := make([]User, 0, len(rows))
	for _, r := range rows {
		users = append(users, User{ID: r.ID, Username: r.Username, Email: r.Email.String})
	}

	return users
}

// Create registers an account and links it to the credential in one
// transaction, so an account can never exist that nobody can sign in to.
func (s *Service) Create(ctx context.Context, account NewAccount) (User, error) {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("begin: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	q := s.queries.WithTx(tx)

	email := NormaliseEmail(account.Email)

	row, err := q.CreateUser(ctx, db.CreateUserParams{
		Username: account.Username,
		Email:    sql.NullString{String: email, Valid: email != ""},
	})
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, ErrUsernameTaken
		}

		return User{}, fmt.Errorf("create user: %w", err)
	}

	err = q.LinkKeyToUser(ctx, db.LinkKeyToUserParams{
		UserID:      row.ID,
		Fingerprint: account.CredentialID,
		PublicKey:   account.Material,
	})
	if err != nil {
		return User{}, fmt.Errorf("link key: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("commit: %w", err)
	}

	return User{ID: row.ID, Username: row.Username, Email: email}, nil
}

// LinkCredential attaches another credential to an existing account, so a
// player can sign in from more than one machine. Re-linking is a no-op.
func (s *Service) LinkCredential(ctx context.Context, userID int64, credentialID, material string) error {
	err := s.queries.LinkKeyToUser(ctx, db.LinkKeyToUserParams{
		UserID:      userID,
		Fingerprint: credentialID,
		PublicKey:   material,
	})
	if err != nil {
		return fmt.Errorf("link key: %w", err)
	}

	return nil
}

// sqlite reports constraint failures as a driver-specific error, so match on
// the message rather than depending on the driver's error type.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
