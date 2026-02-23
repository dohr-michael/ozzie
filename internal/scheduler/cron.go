package scheduler

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// CronExpr wraps a parsed cron schedule.
type CronExpr struct {
	raw      string
	schedule cron.Schedule
}

// ParseCron parses a cron expression string.
// Supports standard 5-field (minute-based) cron expressions.
func ParseCron(expr string) (*CronExpr, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("parse cron %q: %w", expr, err)
	}
	return &CronExpr{raw: expr, schedule: schedule}, nil
}

// Next returns the next activation time after t.
func (c *CronExpr) Next(t time.Time) time.Time {
	return c.schedule.Next(t)
}

// Matches returns true if t falls within the same minute as a scheduled activation.
func (c *CronExpr) Matches(t time.Time) bool {
	// Truncate to minute precision
	truncated := t.Truncate(time.Minute)
	// The previous activation from "truncated + 1 minute" should equal truncated
	next := c.schedule.Next(truncated.Add(-time.Minute))
	return next.Equal(truncated)
}

// String returns the raw cron expression.
func (c *CronExpr) String() string {
	return c.raw
}
