package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletionScriptsHaveValidSyntax(t *testing.T) {
	tests := []struct {
		shell   string
		command string
		want    string
	}{
		{shell: "bash", command: "bash", want: "tendr __complete sessions"},
		{shell: "zsh", command: "zsh", want: "tendr __complete sessions"},
	}

	for _, test := range tests {
		t.Run(test.shell, func(t *testing.T) {
			var output bytes.Buffer
			if err := New(nil, nil, &output, nil).Completion(test.shell); err != nil {
				t.Fatalf("Completion(%q) error = %v", test.shell, err)
			}
			if !strings.Contains(output.String(), test.want) {
				t.Fatalf("Completion(%q) does not contain %q", test.shell, test.want)
			}
			if !strings.Contains(output.String(), "--attach") {
				t.Fatalf("Completion(%q) does not include the start --attach flag", test.shell)
			}
			if !strings.Contains(output.String(), "--running") {
				t.Fatalf("Completion(%q) does not include the list --running flag", test.shell)
			}

			binary, err := exec.LookPath(test.command)
			if err != nil {
				t.Skipf("%s unavailable: %v", test.command, err)
			}
			check := exec.Command(binary, "-n")
			check.Stdin = strings.NewReader(output.String())
			if result, err := check.CombinedOutput(); err != nil {
				t.Fatalf("%s syntax check failed: %v\n%s", test.shell, err, result)
			}
		})
	}
}

func TestCompletionRejectsUnsupportedShell(t *testing.T) {
	err := New(nil, nil, &bytes.Buffer{}, nil).Completion("fish")
	if err == nil || err.Error() != `unsupported shell "fish" (supported: bash, zsh)` {
		t.Fatalf("Completion(\"fish\") error = %v", err)
	}
}

func TestListRunningSessionsPrintsSortedRunningSessionsOnly(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "herdr")
	contents := `#!/bin/sh
if [ "$*" != "session list --json" ]; then
  printf 'unexpected arguments: %s\n' "$*" >&2
  exit 1
fi
printf '%s\n' '{"sessions":[{"name":"zeta","running":true},{"name":"saved","running":false},{"name":"alpha","running":true}]}'
`
	if err := os.WriteFile(script, []byte(contents), 0o755); err != nil {
		t.Fatalf("WriteFile(fake herdr) error = %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var output bytes.Buffer
	if err := New(nil, nil, &output, nil).ListRunningSessions(); err != nil {
		t.Fatalf("ListRunningSessions() error = %v", err)
	}
	if got, want := output.String(), "alpha\nzeta\n"; got != want {
		t.Fatalf("ListRunningSessions() = %q, want %q", got, want)
	}
}
