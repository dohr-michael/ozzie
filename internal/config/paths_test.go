package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOzziePath_Default(t *testing.T) {
	t.Setenv("OZZIE_PATH", "")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	got := OzziePath()
	want := filepath.Join(home, ".ozzie")
	if got != want {
		t.Errorf("OzziePath() = %q, want %q", got, want)
	}
}

func TestOzziePath_EnvOverride(t *testing.T) {
	t.Setenv("OZZIE_PATH", "/tmp/custom-ozzie")
	t.Setenv("OZZIE_HOME", "")

	got := OzziePath()
	want := "/tmp/custom-ozzie"
	if got != want {
		t.Errorf("OzziePath() = %q, want %q", got, want)
	}
}

func TestOzziePath_HomeAlias(t *testing.T) {
	t.Setenv("OZZIE_PATH", "")
	t.Setenv("OZZIE_HOME", "/tmp/home-ozzie")

	got := OzziePath()
	want := "/tmp/home-ozzie"
	if got != want {
		t.Errorf("OzziePath() = %q, want %q", got, want)
	}
}

func TestOzziePath_PathTakesPriority(t *testing.T) {
	t.Setenv("OZZIE_PATH", "/tmp/path-ozzie")
	t.Setenv("OZZIE_HOME", "/tmp/home-ozzie")

	got := OzziePath()
	want := "/tmp/path-ozzie"
	if got != want {
		t.Errorf("OzziePath() = %q, want %q (OZZIE_PATH should take priority over OZZIE_HOME)", got, want)
	}
}

func TestConfigPath(t *testing.T) {
	t.Setenv("OZZIE_PATH", "/tmp/test-ozzie")

	got := ConfigPath()
	want := "/tmp/test-ozzie/config.jsonc"
	if got != want {
		t.Errorf("ConfigPath() = %q, want %q", got, want)
	}
}

func TestDotenvPath(t *testing.T) {
	t.Setenv("OZZIE_PATH", "/tmp/test-ozzie")

	got := DotenvPath()
	want := "/tmp/test-ozzie/.env"
	if got != want {
		t.Errorf("DotenvPath() = %q, want %q", got, want)
	}
}
