package cmd

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const ProjectsDir = "~/.config/tendr"

var ErrNotImplemented = errors.New("not implemented")

type App struct {
	logger *log.Logger
	stdout io.Writer
}

func New(logger *log.Logger, stdout io.Writer) App {
	return App{logger: logger, stdout: stdout}
}

func (a App) List() error {
	dir, err := expandHome(ProjectsDir)
	if err != nil {
		return fmt.Errorf("resolve projects directory: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read projects directory: %w", err)
	}

	var projects []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yml" {
			continue
		}
		projects = append(projects, strings.TrimSuffix(entry.Name(), ".yml"))
	}
	sort.Strings(projects)
	for _, project := range projects {
		fmt.Fprintln(a.stdout, project)
	}
	return nil
}

func (a App) Start(projects []string) error {
	if len(projects) == 0 {
		return errors.New("usage: tendr start <project names...>")
	}
	return ErrNotImplemented
}

func (a App) Stop(projects []string) error {
	if len(projects) == 0 {
		return errors.New("usage: tendr stop <project names...>")
	}
	return ErrNotImplemented
}

func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
}
