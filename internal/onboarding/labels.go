package onboarding

import (
	"strconv"

	"github.com/nstehr/canuckpunk/internal/menu"
)

// What the onboarding screen offers. Labels are shown in the sidebar and may
// be typed verbatim; keep them in the game's voice, since players read and
// retype them.
const (
	LabelCreateUser   = "Create New User"
	LabelStartNewGame = "Start New Game"
	continuePrefix    = "Continue as "
)

// What the onboarding states branch on. Actions are internal, so they stay
// stable even when the labels above are reworded.
const (
	ActionNewUser  menu.Action = "new-user"
	ActionContinue menu.Action = "continue"
)

// Selectors a front end with real controls sends back. Unlike labels these are
// never shown, never translated, and never reworded.
const (
	IDCreateUser     = "create-user"
	IDStartNewGame   = "start-new-game"
	idContinuePrefix = "continue:"
)

// The prose this flow reads. Named once so the startup check and the states
// cannot disagree about what has to exist. Exported because the words inside
// belong to whoever is writing them: anything that needs to know what a screen
// says should read the file, not hardcode the text.
const (
	NarrativeWelcome     = "onboarding/welcome.md"
	NarrativeOrientation = "onboarding/orientation.md"
)

// RequiredNarratives lists the prose this flow cannot run without, for a
// caller that wants to verify it before serving.
func RequiredNarratives() []string {
	return []string{NarrativeWelcome, NarrativeOrientation}
}

// What each state is waiting for, so a front end can say so at its input.
const (
	HintName  = "name"
	HintEmail = "email"
)

// MaxUsername bounds a name so every front end can render it. Discord caps a
// button label at 80 characters and "Continue as " eats into that, which is
// the tightest constraint any client puts on it.
const MaxUsername = 40

// SkipEmail is what a player types to decline the optional address. Enter on
// an empty line cannot mean this: a blank submission never reaches a state.
const SkipEmail = "skip"

// ContinueID is the selector for resuming a particular account.
func ContinueID(userID int64) string {
	return idContinuePrefix + strconv.FormatInt(userID, 10)
}

// ContinueLabel is the label for resuming an existing account.
func ContinueLabel(username string) string {
	return continuePrefix + username
}
