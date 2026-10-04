package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Run the real CLI in the fake SSH host, with its own HOME and PATH.
func TestRemoteTendrProcess(t *testing.T) {
	if os.Getenv("TENDR_REMOTE_TEST_PROCESS") != "1" {
		return
	}
	for index, arg := range os.Args {
		if arg == "--" {
			if err := run(os.Args[index+1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			os.Exit(0)
		}
	}
	os.Exit(1)
}

func TestRunRemoteStartAttach(t *testing.T) {
	for _, args := range [][]string{
		{"--remote", "workbox", "start", "--attach", "demo"},
		{"start", "--remote", "workbox", "--attach", "demo"},
		{"--remote=ssh://you@server:2222", "start", "--attach", "demo"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, logPath := remoteFixture(t)
			target := "workbox"
			if strings.HasPrefix(args[0], "--remote=") {
				target = strings.TrimPrefix(args[0], "--remote=")
			}
			t.Setenv("TENDR_TEST_TARGET", target)
			var stdout, stderr bytes.Buffer
			if err := run(args, strings.NewReader("hello\n"), &stdout, &stderr); err != nil {
				t.Fatalf("run() error = %v, stderr = %s", err, &stderr)
			}
			if stdout.String() != "attached:hello\n" || stderr.String() != "local-client\n" {
				t.Fatalf("stdout = %q, stderr = %q", &stdout, &stderr)
			}
			commands := readRemoteLog(t, logPath)
			before := strings.Index(commands, "before-start")
			workspace := strings.Index(commands, "demo|workspace create")
			after := strings.Index(commands, "after-start")
			attach := strings.Index(commands, "local-herdr:--remote "+target+" --session demo")
			if before < 0 || workspace < before || after < workspace || attach < after {
				t.Fatalf("unexpected startup order:\n%s", commands)
			}
		})
	}
}

func TestRunRemoteListAndStop(t *testing.T) {
	_, logPath := remoteFixture(t)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"list", "--remote", "workbox"}, nil, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "demo\n" {
		t.Fatalf("configured projects = %q", &stdout)
	}
	if err := run([]string{"--remote", "workbox", "start", "demo"}, nil, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"--remote", "workbox", "list", "--running"},
		{"--remote", "workbox", "__complete", "sessions"},
	} {
		stdout.Reset()
		if err := run(args, nil, &stdout, &stderr); err != nil {
			t.Fatal(err)
		}
		if stdout.String() != "default\ndemo\n" {
			t.Fatalf("running sessions = %q", &stdout)
		}
	}
	stdout.Reset()
	if err := run([]string{"--remote", "workbox", "__complete", "projects"}, nil, &stdout, &stderr); err != nil || stdout.String() != "demo\n" {
		t.Fatalf("project completions = %q, %v", &stdout, err)
	}
	if err := run([]string{"stop", "--remote", "workbox", "demo"}, nil, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	commands := readRemoteLog(t, logPath)
	before := strings.Index(commands, "before-stop")
	stop := strings.Index(commands, "unset|session stop demo --json")
	delete := strings.Index(commands, "unset|session delete demo --json")
	after := strings.Index(commands, "after-stop")
	if before < 0 || stop < before || delete < stop || after < delete {
		t.Fatalf("unexpected stop order:\n%s", commands)
	}
}

func TestRunRemoteValidatesAllConfigsBeforeStarting(t *testing.T) {
	home, logPath := remoteFixture(t)
	writeRemoteTestFile(t, filepath.Join(home, ".config", "tendr", "broken.yml"), "root: '~'\nworkspaces: []\n", 0o644)
	var stdout, stderr bytes.Buffer
	err := run([]string{"--remote", "workbox", "start", "demo", "broken"}, nil, &stdout, &stderr)
	if err == nil || !strings.Contains(stderr.String(), "invalid projects: broken") {
		t.Fatalf("error = %v, stderr = %q", err, &stderr)
	}
	if commands := readRemoteLog(t, logPath); strings.Contains(commands, "before-start") || strings.Contains(commands, "demo|server") {
		t.Fatalf("started a session before validation finished:\n%s", commands)
	}
}

func TestRunRemoteProjectNameStartingWithHyphen(t *testing.T) {
	home, logPath := remoteFixture(t)
	writeRemoteTestFile(t, filepath.Join(home, ".config", "tendr", "-demo.yml"), `root: '~'
workspaces:
  - label: main
    tabs:
      - label: shell
`, 0o644)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"start", "--remote", "workbox", "--", "-demo"}, nil, &stdout, &stderr); err != nil {
		t.Fatalf("error = %v, stderr = %q", err, &stderr)
	}
	if commands := readRemoteLog(t, logPath); !strings.Contains(commands, "-demo|server") {
		t.Fatalf("project name was parsed as a flag:\n%s", commands)
	}
}

func TestRunSavedMachineUsesSSHHostAndProjectSession(t *testing.T) {
	for _, selector := range []string{"Build machine", "ssh_build"} {
		t.Run(selector, func(t *testing.T) {
			_, logPath := remoteFixture(t)
			var stdout, stderr bytes.Buffer
			if err := run([]string{"start", "--machine", selector, "--attach", "demo"}, strings.NewReader("hello\n"), &stdout, &stderr); err != nil {
				t.Fatalf("start error = %v, stderr = %q", err, &stderr)
			}
			if stdout.String() != "attached:hello\n" {
				t.Fatalf("attach stdout = %q", &stdout)
			}
			stdout.Reset()
			if err := run([]string{"--machine", selector, "list"}, nil, &stdout, &stderr); err != nil || stdout.String() != "demo\n" {
				t.Fatalf("list output = %q, error = %v", &stdout, err)
			}
			stdout.Reset()
			if err := run([]string{"--machine", selector, "attach", "demo"}, strings.NewReader("again\n"), &stdout, &stderr); err != nil || stdout.String() != "attached:again\n" {
				t.Fatalf("attach output = %q, error = %v", &stdout, err)
			}
			stdout.Reset()
			if err := run([]string{"list", "--machine", selector, "--running"}, nil, &stdout, &stderr); err != nil || stdout.String() != "default\ndemo\n" {
				t.Fatalf("running sessions = %q, error = %v", &stdout, err)
			}
			stdout.Reset()
			if err := run([]string{"--machine", selector, "__complete", "projects"}, nil, &stdout, &stderr); err != nil || stdout.String() != "demo\n" {
				t.Fatalf("project completions = %q, error = %v", &stdout, err)
			}
			if err := run([]string{"stop", "--machine", selector, "demo"}, nil, &stdout, &stderr); err != nil {
				t.Fatal(err)
			}
			commands := readRemoteLog(t, logPath)
			if !strings.Contains(commands, "local-herdr:--remote workbox --session demo") || strings.Contains(commands, "saved-session") {
				t.Fatalf("selected the profile session instead of the project:\n%s", commands)
			}
		})
	}
}

func TestRunRejectsUnavailableMachinesWithoutSSH(t *testing.T) {
	for _, machines := range []string{
		`[]`, `not json`,
		`[{"id":"ssh_build","label":"Build machine","target":"workbox","enabled":false}]`,
		`[{"id":"ssh_build","label":"Build machine","target":"host;command","enabled":true}]`,
	} {
		t.Run(machines, func(t *testing.T) {
			_, logPath := remoteFixture(t)
			t.Setenv("TENDR_TEST_MACHINES", machines)
			var stdout, stderr bytes.Buffer
			if err := run([]string{"--machine", "Build machine", "start", "demo"}, nil, &stdout, &stderr); err == nil {
				t.Fatal("accepted unavailable machine")
			}
			if commands := readRemoteLog(t, logPath); strings.Contains(commands, "ssh:") || strings.Contains(commands, "--session") {
				t.Fatalf("connected despite catalog error:\n%s", commands)
			}
		})
	}
}

func TestRunCompletesEnabledMachines(t *testing.T) {
	_, _ = remoteFixture(t)
	t.Setenv("TENDR_TEST_MACHINES", `[
  {"id":"ssh_build","label":"Build machine","target":"workbox","enabled":true},
  {"id":"ssh_disabled","label":"Disabled","target":"offline","enabled":false},
  {"id":"ssh_one","label":"Duplicate","target":"one","enabled":true},
  {"id":"ssh_two","label":"Duplicate","target":"two","enabled":false},
  {"id":"ssh_hidden","label":"Hidden","target":"hidden","enabled":false},
  {"id":"ssh_three","label":"ssh_hidden","target":"three","enabled":true}
]`)
	var stdout, stderr bytes.Buffer
	if err := run([]string{"__complete", "machines"}, nil, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if got, want := stdout.String(), "Build machine\nssh_build\nssh_one\nssh_three\n"; got != want {
		t.Fatalf("machine completions = %q, want %q", got, want)
	}
}

func TestRunRemoteFailedStartDoesNotAttach(t *testing.T) {
	_, logPath := remoteFixture(t)
	var stdout, stderr bytes.Buffer
	err := run([]string{"--remote", "workbox", "start", "--attach", "missing"}, nil, &stdout, &stderr)
	if err == nil || !strings.Contains(stderr.String(), "invalid projects: missing") {
		t.Fatalf("error = %v, stderr = %q", err, &stderr)
	}
	if commands := readRemoteLog(t, logPath); strings.Contains(commands, "local-herdr:") {
		t.Fatalf("attached after failed startup:\n%s", commands)
	}
}

func TestRunRemoteAttachChecksSessionStatus(t *testing.T) {
	for _, name := range []string{"default", "missing", "saved"} {
		t.Run(name, func(t *testing.T) {
			home, logPath := remoteFixture(t)
			// Attachment only needs remote Herdr, even without remote Tendr.
			if err := os.Remove(filepath.Join(home, "bin", "tendr")); err != nil {
				t.Fatal(err)
			}
			t.Setenv("TENDR_TEST_SESSIONS", `{"sessions":[{"name":"default","running":true},{"name":"saved","running":false}]}`)
			var stdout, stderr bytes.Buffer
			err := run([]string{"attach", "--remote", "workbox", name}, strings.NewReader("hello\n"), &stdout, &stderr)
			commands := readRemoteLog(t, logPath)
			if name == "default" {
				if err != nil || stdout.String() != "attached:hello\n" {
					t.Fatalf("error = %v, stdout = %q", err, &stdout)
				}
			} else {
				want := "does not exist"
				if name == "saved" {
					want = "is not running"
				}
				if err == nil || !strings.Contains(err.Error(), want) || strings.Contains(commands, "local-herdr:") {
					t.Fatalf("error = %v, commands:\n%s", err, commands)
				}
			}
		})
	}
}

func TestRunRejectsInvalidRemoteArguments(t *testing.T) {
	for _, args := range [][]string{
		{"--remote=", "list"}, {"start", "--remote=", "demo"},
		{"attach", "--remote=", "default"}, {"stop", "--remote=", "demo"},
		{"list", "--remote="}, {"--remote", "-oProxyCommand=bad", "list"},
		{"--remote", "host;command", "list"}, {"--remote", "two hosts", "list"},
		{"--remote", "workbox", "start", "../bad"},
		{"--remote", "workbox", "stop", "default"},
		{"--remote", "workbox", "attach", "../bad"},
		{"--remote", "workbox", "start", "--attach", "one", "two"},
		{"--machine=", "list"}, {"start", "--machine=", "demo"},
		{"attach", "--machine=", "default"}, {"stop", "--machine=", "demo"},
		{"--remote", "workbox", "--machine", "Build machine", "list"},
		{"--remote", "workbox", "start", "--machine", "Build machine", "demo"},
		{"--machine", "Build machine", "list", "--remote", "workbox"},
	} {
		var stdout, stderr bytes.Buffer
		if err := run(args, nil, &stdout, &stderr); err == nil {
			t.Fatalf("run(%q) succeeded", args)
		}
	}
}

func remoteFixture(t *testing.T) (string, string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SHELL", "/bin/sh")
	t.Setenv("HERDR_SESSION", "local-ambient")
	t.Setenv("HERDR_SOCKET_PATH", "/local/ambient.sock")
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	remoteBin := filepath.Join(home, "bin")
	localBin := t.TempDir()
	logPath := filepath.Join(home, "commands.log")
	for _, dir := range []string{remoteBin, filepath.Join(home, ".config", "tendr")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeRemoteTestFile(t, filepath.Join(home, ".config", "tendr", "demo.yml"), `root: '~'
before_start:
  - test "$PWD" = "$TENDR_TEST_REMOTE_HOME" && test "$HERDR_SESSION" = demo && test -z "${HERDR_SOCKET_PATH-}" && printf 'before-start\n' >> "$TENDR_TEST_LOG"
after_start:
  - printf 'after-start\n' >> "$TENDR_TEST_LOG"
before_stop:
  - printf 'before-stop\n' >> "$TENDR_TEST_LOG"
after_stop:
  - printf 'after-stop\n' >> "$TENDR_TEST_LOG"
workspaces:
  - label: main
    tabs:
      - label: shell
`, 0o644)
	writeRemoteTestFile(t, filepath.Join(localBin, "ssh"), `#!/bin/sh
test "$1" = -T || exit 9
shift
if [ "$1" = -o ]; then
  test "$2" = BatchMode=yes && test "$3" = -o && test "$4" = ConnectTimeout=5 || exit 9
  shift 4
  printf 'completion-ssh\n' >> "$TENDR_TEST_LOG"
fi
test "$#" = 3 && test "$1" = -- && test "$2" = "$TENDR_TEST_TARGET" || exit 9
test -z "${HERDR_SESSION-}" && test -z "${HERDR_SOCKET_PATH-}" || exit 10
printf 'ssh:%s\n' "$2" >> "$TENDR_TEST_LOG"
env HOME="$TENDR_TEST_REMOTE_HOME" PATH="$TENDR_TEST_REMOTE_BIN:$PATH" HERDR_SESSION=remote-ambient HERDR_SOCKET_PATH=/remote/ambient.sock /bin/sh -c "$3"
`, 0o755)
	writeRemoteTestFile(t, filepath.Join(remoteBin, "tendr"), `#!/bin/sh
exec "$TENDR_TEST_BINARY" -test.run='^TestRemoteTendrProcess$' -- "$@"
`, 0o755)
	writeRemoteTestFile(t, filepath.Join(remoteBin, "herdr"), `#!/bin/sh
test -z "${HERDR_SOCKET_PATH-}" || exit 11
printf '%s|%s\n' "${HERDR_SESSION-unset}" "$*" >> "$TENDR_TEST_LOG"
case "$*" in
  "session list --json")
    test -z "${HERDR_SESSION-}" || exit 12
    if [ -n "${TENDR_TEST_SESSIONS-}" ]; then
      printf '%s\n' "$TENDR_TEST_SESSIONS"
    elif [ -f "$TENDR_TEST_STATE" ]; then
      printf '%s\n' '{"sessions":[{"name":"demo","running":true},{"name":"default","running":true}]}'
    else
      printf '%s\n' '{"sessions":[]}'
    fi
    ;;
  "server") : > "$TENDR_TEST_STATE" ;;
  "status server --json")
    if [ -f "$TENDR_TEST_STATE" ]; then
      printf '%s\n' '{"running":true}'
    else
      printf '%s\n' '{"running":false}'
    fi
    ;;
  workspace\ create*)
    printf '%s\n' '{"result":{"workspace":{"workspace_id":"w1"},"tab":{"tab_id":"t1"},"root_pane":{"pane_id":"p1"}}}'
    ;;
  "session stop demo --json") rm "$TENDR_TEST_STATE" ;;
  *) printf '%s\n' '{"result":{"type":"ok"}}' ;;
esac
`, 0o755)
	writeRemoteTestFile(t, filepath.Join(localBin, "herdr"), `#!/bin/sh
test -z "${HERDR_SESSION-}" && test -z "${HERDR_SOCKET_PATH-}" || exit 13
if [ "$*" = "machine list --json" ]; then
  printf 'catalog-read\n' >> "$TENDR_TEST_LOG"
  printf '%s\n' "$TENDR_TEST_MACHINES"
  exit 0
fi
printf 'local-herdr:%s\n' "$*" >> "$TENDR_TEST_LOG"
test "$1" = --remote && test "$2" = "$TENDR_TEST_TARGET" && test "$3" = --session || exit 14
IFS= read -r input
printf 'attached:%s\n' "$input"
printf 'local-client\n' >&2
`, 0o755)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", localBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TENDR_REMOTE_TEST_PROCESS", "1")
	t.Setenv("TENDR_TEST_BINARY", binary)
	t.Setenv("TENDR_TEST_REMOTE_HOME", home)
	t.Setenv("TENDR_TEST_REMOTE_BIN", remoteBin)
	t.Setenv("TENDR_TEST_TARGET", "workbox")
	t.Setenv("TENDR_TEST_MACHINES", `[{"id":"ssh_build","label":"Build machine","target":"workbox","session":"saved-session","enabled":true,"selected":false}]`)
	t.Setenv("TENDR_TEST_LOG", logPath)
	t.Setenv("TENDR_TEST_STATE", filepath.Join(home, "running"))
	return home, logPath
}

func writeRemoteTestFile(t *testing.T, path, contents string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}
}

func readRemoteLog(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}
