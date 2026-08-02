package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	oldVersion, oldCommit := Version, Commit
	Version, Commit = "1.2.3", "abc12345"
	t.Cleanup(func() { Version, Commit = oldVersion, oldCommit })

	var stdout, stderr bytes.Buffer
	if err := run([]string{"--version"}, &stdout, &stderr); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if got, want := stdout.String(), "tendr 1.2.3 (abc12345)\n"; got != want {
		t.Fatalf("run() output = %q, want %q", got, want)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"launch"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "not a known command") {
		t.Fatalf("run() error = %v", err)
	}
}

func TestRunRejectsListArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"list", "extra"}, &stdout, &stderr)
	if err == nil || err.Error() != "usage: tendr list" {
		t.Fatalf("run() error = %v", err)
	}
}
