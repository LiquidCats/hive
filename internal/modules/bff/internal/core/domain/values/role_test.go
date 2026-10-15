package values_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LiquidCats/hive/internal/modules/bff/internal/core/domain/values"
)

func TestRole_UnmarshalText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		text    string
		want    values.Role
		wantErr error
	}{
		{name: "user", text: "user", want: values.RoleUser},
		{name: "admin", text: "admin", want: values.RoleAdmin},
		{name: "owner", text: "owner", want: values.RoleOwner},
		{name: "unknown", text: "root", wantErr: values.ErrInvalidRole},
		{name: "case sensitive", text: "Admin", wantErr: values.ErrInvalidRole},
		{name: "empty", text: "", wantErr: values.ErrInvalidRole},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := values.RoleAdmin // must stay untouched on error

			err := got.UnmarshalText([]byte(tt.text))

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, values.RoleAdmin, got)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.text, got.String())
		})
	}
}

func TestRole_String_Unknown(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "unknown", values.Role(99).String())
}

func TestRole_Order(t *testing.T) {
	t.Parallel()

	// RequireSession compares roles with <, and an unset role must be the least privileged.
	var zero values.Role

	assert.Equal(t, values.RoleUser, zero)
	assert.Less(t, values.RoleUser, values.RoleAdmin)
	assert.Less(t, values.RoleAdmin, values.RoleOwner)
}
