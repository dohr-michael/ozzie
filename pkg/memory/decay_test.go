package memory

import (
	"math"
	"testing"
	"time"
)

func TestApplyDecay_Normal_WithinGracePeriod(t *testing.T) {
	now := time.Now()
	lastUsed := now.Add(-3 * 24 * time.Hour) // 3 days ago
	result := ApplyDecay(0.8, lastUsed, now, ImportanceNormal)
	if result != 0.8 {
		t.Errorf("expected 0.8 (no decay within grace), got %f", result)
	}
}

func TestApplyDecay_Normal_ExactGraceBoundary(t *testing.T) {
	now := time.Now()
	lastUsed := now.Add(-7 * 24 * time.Hour) // exactly 7 days
	result := ApplyDecay(0.8, lastUsed, now, ImportanceNormal)
	if result != 0.8 {
		t.Errorf("expected 0.8 (at grace boundary), got %f", result)
	}
}

func TestApplyDecay_Normal_OneWeekAfterGrace(t *testing.T) {
	now := time.Now()
	lastUsed := now.Add(-14 * 24 * time.Hour) // 14 days = 7 grace + 7 idle
	result := ApplyDecay(0.8, lastUsed, now, ImportanceNormal)
	expected := 0.8 - 0.01*1.0 // 1 week after grace
	if math.Abs(result-expected) > 1e-9 {
		t.Errorf("expected %f, got %f", expected, result)
	}
}

func TestApplyDecay_Normal_SixMonths(t *testing.T) {
	now := time.Now()
	lastUsed := now.Add(-180 * 24 * time.Hour) // ~6 months
	result := ApplyDecay(0.6, lastUsed, now, ImportanceNormal)

	weeksIdle := float64((180-7)*24) / float64(7*24)
	expected := 0.6 - 0.01*weeksIdle
	if expected < 0.1 {
		expected = 0.1
	}
	if math.Abs(result-expected) > 1e-9 {
		t.Errorf("expected %f, got %f", expected, result)
	}
}

func TestApplyDecay_Normal_Floor(t *testing.T) {
	now := time.Now()
	lastUsed := now.Add(-365 * 24 * time.Hour) // 1 year
	result := ApplyDecay(0.3, lastUsed, now, ImportanceNormal)
	if result != 0.1 {
		t.Errorf("expected floor 0.1, got %f", result)
	}
}

func TestApplyDecay_Normal_AlreadyAtFloor(t *testing.T) {
	now := time.Now()
	lastUsed := now.Add(-30 * 24 * time.Hour)
	result := ApplyDecay(0.1, lastUsed, now, ImportanceNormal)
	if result != 0.1 {
		t.Errorf("expected floor 0.1, got %f", result)
	}
}

func TestApplyDecay_Core_NeverDecays(t *testing.T) {
	now := time.Now()
	lastUsed := now.Add(-365 * 24 * time.Hour) // 1 year idle
	result := ApplyDecay(0.9, lastUsed, now, ImportanceCore)
	if result != 0.9 {
		t.Errorf("core should never decay: expected 0.9, got %f", result)
	}
}

func TestApplyDecay_Important_SlowDecay(t *testing.T) {
	now := time.Now()
	// 60 days = 30 grace + 30 idle ≈ 4.28 weeks after grace
	lastUsed := now.Add(-60 * 24 * time.Hour)
	result := ApplyDecay(0.8, lastUsed, now, ImportanceImportant)

	weeksIdle := float64((60-30)*24) / float64(7*24) // ~4.28 weeks
	expected := 0.8 - 0.005*weeksIdle
	if expected < 0.3 {
		expected = 0.3
	}
	if math.Abs(result-expected) > 1e-9 {
		t.Errorf("expected %f, got %f", expected, result)
	}
	// Should be above normal's floor
	if result < 0.3 {
		t.Errorf("important floor is 0.3, got %f", result)
	}
}

func TestApplyDecay_Ephemeral_FastDecay(t *testing.T) {
	now := time.Now()
	// 8 days = 1 grace + 7 idle = 1 week after grace
	lastUsed := now.Add(-8 * 24 * time.Hour)
	result := ApplyDecay(0.8, lastUsed, now, ImportanceEphemeral)

	expected := 0.8 - 0.05*1.0 // 0.75
	if math.Abs(result-expected) > 1e-9 {
		t.Errorf("expected %f, got %f", expected, result)
	}
}

func TestApplyDecay_EmptyImportance_DefaultsToNormal(t *testing.T) {
	now := time.Now()
	lastUsed := now.Add(-14 * 24 * time.Hour)
	result := ApplyDecay(0.8, lastUsed, now, "")
	expected := ApplyDecay(0.8, lastUsed, now, ImportanceNormal)
	if result != expected {
		t.Errorf("empty importance should default to normal: expected %f, got %f", expected, result)
	}
}
