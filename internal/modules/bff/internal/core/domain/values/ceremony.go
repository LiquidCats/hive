package values

import (
	"encoding"

	"github.com/rotisserie/eris"
)

var ErrInvalidCeremonyType = eris.New("invalid ceremony type")

var _ encoding.TextUnmarshaler = (*CeremonyType)(nil)

type CeremonyType int

const (
	CeremonyTypeRegistration CeremonyType = iota
	CeremonyTypeLogin
)

var mapTextToCeremony = map[string]CeremonyType{
	"registration": CeremonyTypeRegistration,
	"login":        CeremonyTypeLogin,
}

var mapCeremonyToText = map[CeremonyType]string{
	CeremonyTypeRegistration: "registration",
	CeremonyTypeLogin:        "login",
}

// UnmarshalText implements [encoding.TextUnmarshaler].
func (t *CeremonyType) UnmarshalText(text []byte) error {
	if ceremony, ok := mapTextToCeremony[string(text)]; ok {
		*t = ceremony

		return nil
	}

	return ErrInvalidCeremonyType
}

func (t CeremonyType) String() string {
	if text, ok := mapCeremonyToText[t]; ok {
		return text
	}

	return "unknown"
}
