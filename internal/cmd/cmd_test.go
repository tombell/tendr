package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestListPrintsSortedYAMLProjectsOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "tendr")
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"zeta.yml", "alpha.yml", "notes.yaml", "README"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var output bytes.Buffer
	if err := New(nil, nil, &output, nil).List(); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got, want := strings.Fields(output.String()), []string{"alpha", "zeta"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("List() = %#v, want %#v", got, want)
	}
}

func TestListMissingDirectoryIsEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var output bytes.Buffer
	if err := New(nil, nil, &output, nil).List(); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("List() output = %q", output.String())
	}
}

func TestLoadProjectsRejectsAllBeforeReturningAny(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "tendr")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	valid := `root: /tmp
workspaces:
  - label: valid
    tabs:
      - label: shell
`
	invalid := `root: /tmp
workspaces:
  - label: invalid
    tabs:
      - name: tm-style-field
`
	if err := os.WriteFile(filepath.Join(dir, "valid.yml"), []byte(valid), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "invalid.yml"), []byte(invalid), 0o644); err != nil {
		t.Fatal(err)
	}

	configs, err := loadProjects([]string{"valid", "invalid"})
	if err == nil || !strings.Contains(err.Error(), "invalid projects: invalid") {
		t.Fatalf("loadProjects() error = %v", err)
	}
	if configs != nil {
		t.Fatalf("loadProjects() configs = %#v, want nil", configs)
	}
}

func TestLoadProjectsRejectsPathTraversal(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if _, err := loadProjects([]string{"../elsewhere"}); err == nil || !strings.Contains(err.Error(), "invalid project name") {
		t.Fatalf("loadProjects() error = %v", err)
	}
}
