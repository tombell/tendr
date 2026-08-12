package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	oldVersion, oldCommit := Version, Commit
	Version, Commit = "1.2.3", "abc12345"
	t.Cleanup(func() { Version, Commit = oldVersion, oldCommit })

	var stdout, stderr bytes.Buffer
	if err := run([]string{"--version"}, nil, &stdout, &stderr); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got, want := stdout.String(), "tendr 1.2.3 (abc12345)\n"; got != want {
		t.Fatalf("run() output = %q, want %q", got, want)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"launch"}, nil, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "not a known command") {
		t.Fatalf("run() error = %v", err)
	}
}

func TestRunRejectsListArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"list", "extra"}, nil, &stdout, &stderr)
	if err == nil || err.Error() != "usage: tendr list [--running]" {
		t.Fatalf("run() error = %v", err)
	}
}

func TestRunListsRunningSessions(t *testing.T) {
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

	var stdout, stderr bytes.Buffer
	if err := run([]string{"list", "--running"}, nil, &stdout, &stderr); err != nil {
		t.Fatalf("run(list --running) error = %v", err)
	}
	if got, want := stdout.String(), "alpha\nzeta\n"; got != want {
		t.Fatalf("run(list --running) = %q, want %q", got, want)
	}
}

func TestRunCompletion(t *testing.T) {
	for _, shell := range []string{"bash", "fish", "zsh"} {
		var stdout, stderr bytes.Buffer
		if err := run([]string{"completion", shell}, nil, &stdout, &stderr); err != nil {
			t.Fatalf("run(completion %s) error = %v", shell, err)
		}
		if !strings.Contains(stdout.String(), "tendr __complete sessions") {
			t.Fatalf("run(completion %s) output does not complete running sessions", shell)
		}
		runningFlag := "--running"
		if shell == "fish" {
			runningFlag = "-l running"
		}
		if !strings.Contains(stdout.String(), runningFlag) {
			t.Fatalf("run(completion %s) output does not complete list --running", shell)
		}
		if stderr.Len() != 0 {
			t.Fatalf("run(completion %s) stderr = %q", shell, stderr.String())
		}
	}
}

func TestRunRejectsInvalidCompletionArguments(t *testing.T) {
	for _, args := range [][]string{{"completion"}, {"completion", "bash", "extra"}} {
		var stdout, stderr bytes.Buffer
		err := run(args, nil, &stdout, &stderr)
		if err == nil || err.Error() != "usage: tendr completion <bash|fish|zsh>" {
			t.Fatalf("run(%q) error = %v", args, err)
		}
	}

	var stdout, stderr bytes.Buffer
	err := run([]string{"completion", "nushell"}, nil, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "unsupported shell") {
		t.Fatalf("run(completion nushell) error = %v", err)
	}
}

func TestRunCompletesRunningSessionsOnly(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "herdr")
	contents := `#!/bin/sh
if [ "$*" != "session list --json" ]; then
  printf 'unexpected arguments: %s\n' "$*" >&2
  exit 1
fi
printf '%s\n' '{"sessions":[{"name":"running","running":true},{"name":"stopped","running":false}]}'
`
	if err := os.WriteFile(script, []byte(contents), 0o755); err != nil {
		t.Fatalf("WriteFile(fake herdr) error = %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var stdout, stderr bytes.Buffer
	if err := run([]string{"__complete", "sessions"}, nil, &stdout, &stderr); err != nil {
		t.Fatalf("run(__complete sessions) error = %v", err)
	}
	if got, want := stdout.String(), "running\n"; got != want {
		t.Fatalf("run(__complete sessions) = %q, want %q", got, want)
	}
}

func TestRunAttachConnectsStandardIO(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "herdr")
	contents := `#!/bin/sh
case "$*" in
  "session list --json")
    printf '%s\n' '{"sessions":[{"name":"demo","running":true}]}'
    ;;
  "session attach demo")
    IFS= read -r input
    printf 'attached:%s\n' "$input"
    printf 'notice:demo\n' >&2
    ;;
  *)
    printf 'unexpected arguments: %s\n' "$*" >&2
    exit 1
    ;;
esac
`
	if err := os.WriteFile(script, []byte(contents), 0o755); err != nil {
		t.Fatalf("WriteFile(fake herdr) error = %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var stdout, stderr bytes.Buffer
	if err := run([]string{"attach", "demo"}, strings.NewReader("hello\n"), &stdout, &stderr); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got, want := stdout.String(), "attached:hello\n"; got != want {
		t.Fatalf("run() stdout = %q, want %q", got, want)
	}
	if got, want := stderr.String(), "notice:demo\n"; got != want {
		t.Fatalf("run() stderr = %q, want %q", got, want)
	}
}

func TestRunStartAttachConnectsAfterStartupFinishes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	projectsDir := filepath.Join(home, ".config", "tendr")
	if err := os.MkdirAll(projectsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	config := `root: .
after_start:
  - printf 'after-start\n' >> "$FAKE_HERDR_LOG"
workspaces:
  - label: main
    tabs:
      - label: shell
`
	if err := os.WriteFile(filepath.Join(projectsDir, "demo.yml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "herdr")
	logPath := filepath.Join(dir, "commands.log")
	statePath := filepath.Join(dir, "running")
	contents := `#!/bin/sh
printf '%s|%s\n' "${HERDR_SESSION-unset}" "$*" >> "$FAKE_HERDR_LOG"
case "$*" in
  "session list --json")
    if [ -f "$FAKE_HERDR_STATE" ]; then
      printf '%s\n' '{"sessions":[{"name":"demo","running":true}]}'
    else
      printf '%s\n' '{"sessions":[]}'
    fi
    ;;
  "server")
    : > "$FAKE_HERDR_STATE"
    ;;
  "status server --json")
    printf '%s\n' '{"running":true}'
    ;;
  workspace\ create*)
    printf '%s\n' '{"result":{"workspace":{"workspace_id":"workspace-1"},"tab":{"tab_id":"tab-1"},"root_pane":{"pane_id":"pane-1"}}}'
    ;;
  "session attach demo")
    IFS= read -r input
    printf 'attached:%s\n' "$input"
    printf 'notice:demo\n' >&2
    ;;
  *)
    printf '%s\n' '{"result":{"type":"ok"}}'
    ;;
esac
`
	if err := os.WriteFile(script, []byte(contents), 0o755); err != nil {
		t.Fatalf("WriteFile(fake herdr) error = %v", err)
	}
	t.Setenv("FAKE_HERDR_LOG", logPath)
	t.Setenv("FAKE_HERDR_STATE", statePath)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var stdout, stderr bytes.Buffer
	if err := run([]string{"start", "--attach", "demo"}, strings.NewReader("hello\n"), &stdout, &stderr); err != nil {
		t.Fatalf("run(start --attach demo) error = %v", err)
	}
	if got, want := stdout.String(), "attached:hello\n"; got != want {
		t.Fatalf("run(start --attach demo) stdout = %q, want %q", got, want)
	}
	if got, want := stderr.String(), "notice:demo\n"; got != want {
		t.Fatalf("run(start --attach demo) stderr = %q, want %q", got, want)
	}

	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	commands := string(log)
	focus := strings.Index(commands, "workspace focus workspace-1")
	afterStart := strings.Index(commands, "after-start")
	attach := strings.Index(commands, "session attach demo")
	if focus < 0 || afterStart < focus || attach < afterStart {
		t.Fatalf("attach did not happen after startup finished:\n%s", commands)
	}
}

func TestRunStartAttachRejectsMultipleProjects(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"start", "--attach", "one", "two"}, nil, &stdout, &stderr)
	if err == nil || err.Error() != "--attach requires exactly one project" {
		t.Fatalf("run(start --attach one two) error = %v", err)
	}
}

func TestRunAttachRejectsMissingSession(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "herdr")
	contents := `#!/bin/sh
if [ "$*" != "session list --json" ]; then
  printf 'unexpected arguments: %s\n' "$*" >&2
  exit 1
fi
printf '%s\n' '{"sessions":[]}'
`
	if err := os.WriteFile(script, []byte(contents), 0o755); err != nil {
		t.Fatalf("WriteFile(fake herdr) error = %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var stdout, stderr bytes.Buffer
	err := run([]string{"attach", "missing"}, nil, &stdout, &stderr)
	if err == nil || err.Error() != `session "missing" does not exist` {
		t.Fatalf("run(attach missing) error = %v", err)
	}
}

func TestRunRejectsInvalidAttachArguments(t *testing.T) {
	for _, args := range [][]string{{"attach"}, {"attach", "one", "two"}} {
		var stdout, stderr bytes.Buffer
		err := run(args, nil, &stdout, &stderr)
		if err == nil || err.Error() != "usage: tendr attach <name>" {
			t.Fatalf("run(%q) error = %v", args, err)
		}
	}
}
