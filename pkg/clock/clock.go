package clock

import "time"

type Clock struct {
}

func New() *Clock {
	return &Clock{}
}

func (c *Clock) Now() time.Time {
	return time.Now()
}

func (c *Clock) Until(t time.Time) time.Duration {
	return time.Until(t)
}

func (c *Clock) Since(t time.Time) time.Duration {
	return time.Since(t)
}
