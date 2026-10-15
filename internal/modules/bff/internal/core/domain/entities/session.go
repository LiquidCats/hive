package entities

import (
	"time"

	"github.com/LiquidCats/hive/internal/kernel/values"
)

// Session is a signed-in browser; only the hash of its cookie token is stored.
type Session struct {
	TokenHash string
	UserID    values.UserID
	ExpiresAt time.Time
}
