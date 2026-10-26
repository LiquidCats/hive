package values_test

import (
	"testing"
	"uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/values"
)

func TestCursor_RoundTrip(t *testing.T) {
	t.Parallel()

	for _, want := range []values.Cursor{
		{ID: uuid.NewV7()},
		{ID: uuid.NewV7(), Before: true},
	} {
		got, err := values.ParseCursor(want.String())
		require.NoError(t, err)
		assert.Equal(t, want, got)
	}
}

func TestParseCursor_Invalid(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"", "not-a-uuid", "after:", "after:nope", "around:" + uuid.NewV7().String(), uuid.NewV7().String()} {
		_, err := values.ParseCursor(in)
		assert.ErrorIs(t, err, values.ErrInvalidCursor, in)
	}
}
