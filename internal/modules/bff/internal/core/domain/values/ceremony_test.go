package values_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/values"
)

type CeremonyTypeSuite struct {
	suite.Suite
}

func TestCeremonyTypeSuite(t *testing.T) {
	suite.Run(t, new(CeremonyTypeSuite))
}

func (s *CeremonyTypeSuite) TestTextRoundTrip() {
	for _, want := range []values.CeremonyType{values.CeremonyTypeRegistration, values.CeremonyTypeLogin} {
		s.Run(want.String(), func() {
			var got values.CeremonyType

			s.Require().NoError(got.UnmarshalText([]byte(want.String())))
			s.Equal(want, got)
		})
	}
}

func (s *CeremonyTypeSuite) TestUnmarshalText_Unknown() {
	var got values.CeremonyType

	s.ErrorIs(got.UnmarshalText([]byte("logout")), values.ErrInvalidCeremonyType)
}

func (s *CeremonyTypeSuite) TestString_Unknown() {
	s.Equal("unknown", values.CeremonyType(99).String())
}
