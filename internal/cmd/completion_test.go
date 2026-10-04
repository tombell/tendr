package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletionQueriesSelectedRemoteHost(t *testing.T) {
	tests := []struct {
		words []string
		query string
		want  string
	}{
		{[]string{"tendr", "--remote", "workbox", "start", ""}, "--remote workbox __complete projects", "remote-project"},
		{[]string{"tendr", "start", "--remote", "workbox", "--attach", ""}, "--remote workbox __complete projects", "remote-project"},
		{[]string{"tendr", "attach", "--remote", "workbox", ""}, "--remote workbox __complete sessions", "remote-session"},
		{[]string{"tendr", "--remote=workbox", "stop", ""}, "--remote workbox __complete projects", "remote-project"},
		{[]string{"tendr", "--remote", "start", "attach", ""}, "--remote start __complete sessions", "remote-session"},
		{[]string{"tendr", "start", "--remote", ""}, "", ""},
		{[]string{"tendr", "--machine", "Build machine", "start", ""}, "--machine Build machine __complete projects", "remote-project"},
		{[]string{"tendr", "attach", "--machine", "ssh_build", ""}, "--machine ssh_build __complete sessions", "remote-session"},
		{[]string{"tendr", "--machine=start", "stop", ""}, "--machine start __complete projects", "remote-project"},
		{[]string{"tendr", "--machine", ""}, "__complete machines", "ssh_build"},
		{[]string{"tendr", "start", "--machine", ""}, "__complete machines", "ssh_build"},
	}
	for _, shell := range []string{"bash", "zsh", "fish"} {
		for _, test := range tests {
			t.Run(shell+"/"+strings.Join(test.words, " "), func(t *testing.T) {
				binary, err := exec.LookPath(shell)
				if err != nil {
					t.Skipf("%s unavailable: %v", shell, err)
				}
				dir := t.TempDir()
				logPath := filepath.Join(dir, "query.log")
				fakeTendr := `#!/bin/sh
if [ "$1" = --machine ]; then
  test "$#" = 4 || exit 9
fi
printf '%s\n' "$*" > "$TENDR_COMPLETION_LOG"
case "$*" in
  "__complete machines") printf 'Build machine\nssh_build\n' ;;
  *"__complete sessions") printf 'remote-session\n' ;;
  *"__complete projects") printf 'remote-project\n' ;;
esac
`
				if err := os.WriteFile(filepath.Join(dir, "tendr"), []byte(fakeTendr), 0o755); err != nil {
					t.Fatal(err)
				}
				t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
				t.Setenv("TENDR_COMPLETION_LOG", logPath)
				var script bytes.Buffer
				if err := New(nil, nil, &script, nil).Completion(shell); err != nil {
					t.Fatal(err)
				}
				var quoted []string
				for _, word := range test.words {
					quoted = append(quoted, "'"+strings.ReplaceAll(word, "'", "'\"'\"'")+"'")
				}
				var args []string
				switch shell {
				case "bash":
					fmt.Fprintf(&script, "\nCOMP_WORDS=(%s)\nCOMP_CWORD=%d\n_tendr\nprintf '%%s\\n' \"${COMPREPLY[@]}\"\n", strings.Join(quoted, " "), len(test.words)-1)
					args = []string{"--noprofile", "--norc"}
				case "zsh":
					// Capture candidates without starting an interactive completion UI.
					prefix := "compdef() { :; }\ncompadd() { shift; printf '%s\\n' \"$@\"; }\n"
					contents := script.String()
					script.Reset()
					script.WriteString(prefix)
					script.WriteString(contents)
					fmt.Fprintf(&script, "\nwords=(%s)\nCURRENT=%d\n_tendr\n", strings.Join(quoted, " "), len(test.words))
					args = []string{"-f"}
				case "fish":
					line := strings.Join(quoted[:len(quoted)-1], " ") + " " + test.words[len(test.words)-1]
					fmt.Fprintf(&script, "\ncomplete -C '%s'\n", strings.ReplaceAll(line, "'", "'\"'\"'"))
					args = []string{"--no-config"}
				}
				command := exec.Command(binary, args...)
				command.Stdin = &script
				output, err := command.CombinedOutput()
				if err != nil {
					t.Fatalf("completion failed: %v\n%s", err, output)
				}
				if test.want != "" && !strings.Contains(string(output), test.want) {
					t.Fatalf("completion candidates = %q, want %q", output, test.want)
				}
				query, err := os.ReadFile(logPath)
				if test.query == "" {
					if !os.IsNotExist(err) {
						t.Fatalf("queried sessions while completing an SSH target: %q, %v", query, err)
					}
				} else if err != nil || strings.TrimSpace(string(query)) != test.query {
					t.Fatalf("completion query = %q, %v, want %q", query, err, test.query)
				}
			})
		}
	}
}

func TestCompletionScriptsHaveValidSyntax(t *testing.T) {
	tests := []struct {
		shell       string
		command     string
		want        string
		attachFlag  string
		runningFlag string
	}{
		{shell: "bash", command: "bash", want: "__complete sessions", attachFlag: "--attach", runningFlag: "--running"},
		{shell: "fish", command: "fish", want: "__complete sessions", attachFlag: "-l attach", runningFlag: "-l running"},
		{shell: "zsh", command: "zsh", want: "__complete sessions", attachFlag: "--attach", runningFlag: "--running"},
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
			if !strings.Contains(output.String(), test.attachFlag) {
				t.Fatalf("Completion(%q) does not include the start --attach flag", test.shell)
			}
			if !strings.Contains(output.String(), test.runningFlag) {
				t.Fatalf("Completion(%q) does not include the list --running flag", test.shell)
			}
			if !strings.Contains(output.String(), "fish") {
				t.Fatalf("Completion(%q) does not include fish as a completion target", test.shell)
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
	err := New(nil, nil, &bytes.Buffer{}, nil).Completion("nushell")
	if err == nil || err.Error() != `unsupported shell "nushell" (supported: bash, fish, zsh)` {
		t.Fatalf("Completion(\"nushell\") error = %v", err)
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
