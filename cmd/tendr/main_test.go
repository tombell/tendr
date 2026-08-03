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
	if err == nil || err.Error() != "usage: tendr list" {
		t.Fatalf("run() error = %v", err)
	}
}

func TestRunAttachConnectsStandardIO(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "herdr")
	contents := `#!/bin/sh
if [ "$*" != "session attach demo" ]; then
  printf 'unexpected arguments: %s\n' "$*" >&2
  exit 1
fi
IFS= read -r input
printf 'attached:%s\n' "$input"
printf 'notice:demo\n' >&2
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

func TestRunRejectsInvalidAttachArguments(t *testing.T) {
	for _, args := range [][]string{{"attach"}, {"attach", "one", "two"}} {
		var stdout, stderr bytes.Buffer
		err := run(args, nil, &stdout, &stderr)
		if err == nil || err.Error() != "usage: tendr attach <name>" {
			t.Fatalf("run(%q) error = %v", args, err)
		}
	}
}
