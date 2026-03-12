package scheduler

import (
	"testing"
	"time"
)

func TestParseCron_Valid(t *testing.T) {
	expr, err := ParseCron("*/5 * * * *")
	if err != nil {
		t.Fatalf("ParseCron: %v", err)
	}
	if expr.String() != "*/5 * * * *" {
		t.Fatalf("expected raw %q, got %q", "*/5 * * * *", expr.String())
	}
}

func TestParseCron_Invalid(t *testing.T) {
	_, err := ParseCron("not a cron")
	if err == nil {
		t.Fatal("expected error for invalid cron expression")
	}
}

func TestCronExpr_Next(t *testing.T) {
	expr, err := ParseCron("0 12 * * *") // every day at noon
	if err != nil {
		t.Fatalf("ParseCron: %v", err)
	}

	base := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	next := expr.Next(base)

	expected := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Fatalf("expected next %v, got %v", expected, next)
	}
}

func TestCronExpr_Matches(t *testing.T) {
	expr, err := ParseCron("30 14 * * *") // daily at 14:30
	if err != nil {
		t.Fatalf("ParseCron: %v", err)
	}

	match := time.Date(2025, 6, 15, 14, 30, 45, 0, time.UTC)
	if !expr.Matches(match) {
		t.Fatal("expected Matches to return true for 14:30")
	}

	noMatch := time.Date(2025, 6, 15, 14, 31, 0, 0, time.UTC)
	if expr.Matches(noMatch) {
		t.Fatal("expected Matches to return false for 14:31")
	}
}

func TestCronExpr_EveryFiveMinutes(t *testing.T) {
	expr, err := ParseCron("*/5 * * * *")
	if err != nil {
		t.Fatalf("ParseCron: %v", err)
	}

	at0 := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	at5 := time.Date(2025, 1, 1, 10, 5, 0, 0, time.UTC)
	at3 := time.Date(2025, 1, 1, 10, 3, 0, 0, time.UTC)

	if !expr.Matches(at0) {
		t.Fatal("expected match at :00")
	}
	if !expr.Matches(at5) {
		t.Fatal("expected match at :05")
	}
	if expr.Matches(at3) {
		t.Fatal("expected no match at :03")
	}
}
