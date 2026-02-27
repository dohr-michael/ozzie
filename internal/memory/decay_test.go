package memory

import (
	"math"
	"testing"
	"time"
)

func TestApplyDecay_WithinGracePeriod(t *testing.T) {
	now := time.Now()
	lastUsed := now.Add(-3 * 24 * time.Hour) // 3 days ago
	result := ApplyDecay(0.8, lastUsed, now)
	if result != 0.8 {
		t.Errorf("expected 0.8 (no decay within grace), got %f", result)
	}
}

func TestApplyDecay_ExactGraceBoundary(t *testing.T) {
	now := time.Now()
	lastUsed := now.Add(-decayGracePeriod) // exactly 7 days
	result := ApplyDecay(0.8, lastUsed, now)
	if result != 0.8 {
		t.Errorf("expected 0.8 (at grace boundary), got %f", result)
	}
}

func TestApplyDecay_OneWeekAfterGrace(t *testing.T) {
	now := time.Now()
	lastUsed := now.Add(-14 * 24 * time.Hour) // 14 days = 7 grace + 7 idle
	result := ApplyDecay(0.8, lastUsed, now)
	expected := 0.8 - 0.01*1.0 // 1 week after grace
	if math.Abs(result-expected) > 1e-9 {
		t.Errorf("expected %f, got %f", expected, result)
	}
}

func TestApplyDecay_SixMonths(t *testing.T) {
	now := time.Now()
	lastUsed := now.Add(-180 * 24 * time.Hour) // ~6 months
	result := ApplyDecay(0.6, lastUsed, now)

	weeksIdle := float64((180-7)*24) / float64(7*24)
	expected := 0.6 - 0.01*weeksIdle
	if expected < decayFloor {
		expected = decayFloor
	}
	if math.Abs(result-expected) > 1e-9 {
		t.Errorf("expected %f, got %f", expected, result)
	}
}

func TestApplyDecay_Floor(t *testing.T) {
	now := time.Now()
	lastUsed := now.Add(-365 * 24 * time.Hour) // 1 year
	result := ApplyDecay(0.3, lastUsed, now)
	if result != decayFloor {
		t.Errorf("expected floor %f, got %f", decayFloor, result)
	}
}

func TestApplyDecay_AlreadyAtFloor(t *testing.T) {
	now := time.Now()
	lastUsed := now.Add(-30 * 24 * time.Hour)
	result := ApplyDecay(decayFloor, lastUsed, now)
	if result != decayFloor {
		t.Errorf("expected floor %f, got %f", decayFloor, result)
	}
}
