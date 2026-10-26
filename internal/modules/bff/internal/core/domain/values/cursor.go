package values

import (
	"strings"
	"uuid"

	"github.com/rotisserie/eris"
)

var ErrInvalidCursor = eris.New("invalid cursor")

// Cursor positions a page next to an item: the page after ID, or the one before it.
// The zero value is the first page.
type Cursor struct {
	ID     uuid.UUID
	Before bool
}

// Cursors point at the neighbouring pages; nil where there is none.
type Cursors struct {
	Next, Prev *Cursor
}

// ParseCursor reads the "after:<uuid>" or "before:<uuid>" form [Cursor.String] produces.
func ParseCursor(s string) (Cursor, error) {
	dir, id, ok := strings.Cut(s, ":")
	if !ok || (dir != "after" && dir != "before") {
		return Cursor{}, ErrInvalidCursor
	}

	parsed, err := uuid.Parse(id)
	if err != nil {
		return Cursor{}, ErrInvalidCursor
	}

	return Cursor{ID: parsed, Before: dir == "before"}, nil
}

func (c Cursor) String() string {
	if c.Before {
		return "before:" + c.ID.String()
	}

	return "after:" + c.ID.String()
}
