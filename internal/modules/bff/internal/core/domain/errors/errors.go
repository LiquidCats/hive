package errors

import "github.com/rotisserie/eris"

var (
	ErrInvalidUsername  = eris.New("invalid username")
	ErrUsernameTaken    = eris.New("username taken")
	ErrUserNotFound     = eris.New("user not found")
	ErrApprovalPending  = eris.New("user awaiting approval or disabled")
	ErrCannotManageSelf = eris.New("cannot manage self")
	ErrUserDisabled     = eris.New("target user awaiting approval or disabled")

	ErrUnauthenticated = eris.New("unauthenticated")
	ErrForbidden       = eris.New("forbidden")

	ErrCeremonyNotFound = eris.New("ceremony not found or expired")
	ErrVerification     = eris.New("passkey verification failed")

	ErrRepoNotFound            = eris.New("repository not found")
	ErrRepoURLTaken            = eris.New("repository url taken")
	ErrInvalidRepoName         = eris.New("invalid repository name")
	ErrInvalidRepoURL          = eris.New("invalid repository url")
	ErrInvalidRepoCredentials  = eris.New("credentials do not match the repository url")
	ErrRepoCredentialsRequired = eris.New("changing the repository url requires new credentials")
	// ErrRepoAccessDenied and ErrRepoUnreachable wrap the git error for the log; clients get a fixed message.
	ErrRepoAccessDenied = eris.New("repository not found or credentials rejected")
	ErrRepoUnreachable  = eris.New("could not connect to the repository")
)
