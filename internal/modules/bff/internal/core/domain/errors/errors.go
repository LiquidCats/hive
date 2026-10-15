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
)
