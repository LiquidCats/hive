package errors

import "github.com/rotisserie/eris"

var (
	ErrInvalidUsername   = eris.New("invalid username")
	ErrUsernameTaken     = eris.New("username taken")
	ErrUserNotFound      = eris.New("user not found")
	ErrApprovalPending   = eris.New("user awaiting approval or disabled")
	ErrCannotDisableSelf = eris.New("cannot disable self")

	ErrUnauthenticated = eris.New("unauthenticated")

	ErrCeremonyNotFound = eris.New("ceremony not found or expired")
	ErrVerification     = eris.New("passkey verification failed")
)
