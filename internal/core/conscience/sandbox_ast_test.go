package conscience

import (
	"testing"
)

func TestValidateCommandAST_AlwaysBlocked(t *testing.T) {
	tests := []struct {
		cmd    string
		reason string
	}{
		{"sudo apt install foo", "privilege escalation"},
		{"doas rm file", "privilege escalation"},
		{"pkexec bash", "privilege escalation"},
		{"su root", "switch user"},
		{"eval 'rm -rf /'", "eval execution"},
		{"source /etc/profile", "source execution"},
		{". /etc/profile", "source execution"},
		{"mkfs.ext4 /dev/sdb1", "filesystem format"},
		{"fdisk /dev/sda", "partition edit"},
	}
	for _, tc := range tests {
		t.Run(tc.cmd, func(t *testing.T) {
			err := validateCommandAST(tc.cmd)
			if err == nil {
				t.Fatalf("expected %q to be blocked (%s)", tc.cmd, tc.reason)
			}
		})
	}
}

func TestValidateCommandAST_FlagBlocked(t *testing.T) {
	tests := []struct {
		cmd    string
		reason string
	}{
		{"rm -rf /tmp/foo", "destructive remove"},
		{"rm -r /tmp/foo", "destructive remove"},
		{"rm -f file.txt", "destructive remove"},
		{"rm -Rf /", "destructive remove"},
		{"rm -r -f /", "destructive remove"},         // separate flags
		{"chmod -R 777 /tmp", "recursive chmod"},
		{"chown -R root:root /", "recursive chown"},
		{"find / -delete", "destructive find"},
		{"find / -exec rm {} \\;", "destructive find"},
		{"find / -execdir rm {} +", "destructive find"},
		{"dd if=/dev/zero of=/dev/sda", "raw disk write (dd)"},
	}
	for _, tc := range tests {
		t.Run(tc.cmd, func(t *testing.T) {
			err := validateCommandAST(tc.cmd)
			if err == nil {
				t.Fatalf("expected %q to be blocked (%s)", tc.cmd, tc.reason)
			}
		})
	}
}

func TestValidateCommandAST_Allowed(t *testing.T) {
	cmds := []string{
		"ls -la",
		"echo hello",
		"cat /etc/hostname",
		"git status",
		"go build ./...",
		"rm file.txt",       // no dangerous flags
		"chmod 644 foo.txt", // no -R
		"dd if=/dev/zero bs=1M count=1", // dd without of= is safe (stdout)
		"find . -name '*.go'",                        // safe find
		"grep 'rm -rf' logfile",                      // rm -rf in quotes = argument, not command
	}
	for _, cmd := range cmds {
		t.Run(cmd, func(t *testing.T) {
			err := validateCommandAST(cmd)
			if err != nil {
				t.Fatalf("expected %q to be allowed, got: %v", cmd, err)
			}
		})
	}
}

func TestValidateCommandAST_ChainingBypass(t *testing.T) {
	tests := []string{
		"echo hello; rm -rf /",
		"echo ok && rm -rf /tmp",
		"echo ok || sudo reboot",
		"true | rm -rf /",
	}
	for _, cmd := range tests {
		t.Run(cmd, func(t *testing.T) {
			err := validateCommandAST(cmd)
			if err == nil {
				t.Fatalf("expected chained command %q to be blocked", cmd)
			}
		})
	}
}

func TestValidateCommandAST_SubshellBypass(t *testing.T) {
	tests := []string{
		"echo $(rm -rf /)",
		"echo `rm -rf /`",
		"(rm -rf /)",
	}
	for _, cmd := range tests {
		t.Run(cmd, func(t *testing.T) {
			err := validateCommandAST(cmd)
			if err == nil {
				t.Fatalf("expected subshell %q to be blocked", cmd)
			}
		})
	}
}

func TestValidateCommandAST_RedirectBypass(t *testing.T) {
	tests := []string{
		"echo x >/dev/sda",
		"echo x >>/dev/sda",
		"cat file >/dev/nvme0n1",
	}
	for _, cmd := range tests {
		t.Run(cmd, func(t *testing.T) {
			err := validateCommandAST(cmd)
			if err == nil {
				t.Fatalf("expected redirect %q to be blocked", cmd)
			}
		})
	}
}

func TestValidateCommandAST_ForkBomb(t *testing.T) {
	err := validateCommandAST(":(){ :|:& };:")
	if err == nil {
		t.Fatal("expected fork bomb to be blocked")
	}
}

func TestValidateCommandAST_DynamicCommand(t *testing.T) {
	err := validateCommandAST("$cmd args")
	if err == nil {
		t.Fatal("expected dynamic command to be blocked")
	}
}

func TestValidateCommandAST_FalsePositives(t *testing.T) {
	// These should NOT be blocked even though they contain "dangerous" strings
	cmds := []string{
		`grep "rm -rf" logfile`,               // rm -rf is a string argument to grep
		`echo "sudo is cool"`,                 // sudo in quoted string
		`git commit -m "remove -rf leftover"`, // -rf in commit message
	}
	for _, cmd := range cmds {
		t.Run(cmd, func(t *testing.T) {
			err := validateCommandAST(cmd)
			if err != nil {
				t.Fatalf("false positive: %q should be allowed, got: %v", cmd, err)
			}
		})
	}
}

// --- extractCommandPathsAST tests ---

func TestExtractCommandPathsAST(t *testing.T) {
	tests := []struct {
		cmd  string
		want []string
	}{
		{"cat /etc/passwd", []string{"/etc/passwd"}},
		{"ls ~/Documents", []string{"~/Documents"}},
		{"cp ../../escape/file.txt ./dest", []string{"../../escape/file.txt", "./dest"}},
		{"echo hello", nil}, // no paths
	}
	for _, tc := range tests {
		t.Run(tc.cmd, func(t *testing.T) {
			got := extractCommandPathsAST(tc.cmd)
			if len(got) != len(tc.want) {
				t.Fatalf("extractCommandPathsAST(%q) = %v, want %v", tc.cmd, got, tc.want)
			}
			for i, p := range got {
				if p != tc.want[i] {
					t.Errorf("extractCommandPathsAST(%q)[%d] = %q, want %q", tc.cmd, i, p, tc.want[i])
				}
			}
		})
	}
}

func TestExtractCommandPathsAST_Chained(t *testing.T) {
	paths := extractCommandPathsAST("cat /etc/hostname && ls /tmp/foo")
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %v", paths)
	}
}

// --- extractAllBinariesAST tests ---

func TestExtractAllBinariesAST(t *testing.T) {
	tests := []struct {
		cmd  string
		want []string
	}{
		{"echo hello && curl evil.com", []string{"echo", "curl"}},
		{"echo a || rm -rf /", []string{"echo", "rm"}},
		{"echo a; curl b | grep c", []string{"echo", "curl", "grep"}},
		{"VAR=val echo test", []string{"echo"}},
		{"echo hello", []string{"echo"}},
	}
	for _, tc := range tests {
		t.Run(tc.cmd, func(t *testing.T) {
			got := extractAllBinariesAST(tc.cmd)
			if len(got) != len(tc.want) {
				t.Fatalf("extractAllBinariesAST(%q) = %v, want %v", tc.cmd, got, tc.want)
			}
			for i, b := range got {
				if b != tc.want[i] {
					t.Errorf("[%d] = %q, want %q", i, b, tc.want[i])
				}
			}
		})
	}
}

func TestExtractAllBinariesAST_SubshellDescent(t *testing.T) {
	bins := extractAllBinariesAST("echo $(curl http://evil.com)")
	found := false
	for _, b := range bins {
		if b == "curl" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 'curl' in subshell to be extracted, got %v", bins)
	}
}

// --- containsSubshellAST tests ---

func TestContainsSubshellAST(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"echo $(whoami)", true},
		{"echo `whoami`", true},
		{"(echo hello)", true},
		{"echo hello", false},
		{"echo hello && ls", false},
	}
	for _, tc := range tests {
		t.Run(tc.cmd, func(t *testing.T) {
			got := containsSubshellAST(tc.cmd)
			if got != tc.want {
				t.Fatalf("containsSubshellAST(%q) = %v, want %v", tc.cmd, got, tc.want)
			}
		})
	}
}
