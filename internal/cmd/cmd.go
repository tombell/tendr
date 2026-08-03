package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tombell/tendr/internal/config"
	"github.com/tombell/tendr/internal/herdr"
	"github.com/tombell/tendr/internal/manager"
)

const ProjectsDir = "~/.config/tendr"

type App struct {
	logger *log.Logger
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func New(logger *log.Logger, stdin io.Reader, stdout, stderr io.Writer) App {
	return App{logger: logger, stdin: stdin, stdout: stdout, stderr: stderr}
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

	loaded, err := loadProjects(projects)
	if err != nil {
		return err
	}
	client := herdr.New("", a.logger)
	lifecycle := manager.New(client, manager.NewDefaultShell(a.logger))
	for index, cfg := range loaded {
		if err := lifecycle.Start(context.Background(), projects[index], cfg); err != nil {
			return fmt.Errorf("start project %q: %w", projects[index], err)
		}
	}
	return nil
}

func (a App) Attach(name string) error {
	if !validProjectName(name) {
		return fmt.Errorf("invalid session name %q", name)
	}

	client := herdr.New("", a.logger)
	return client.AttachSession(context.Background(), name, a.stdin, a.stdout, a.stderr)
}

func (a App) Stop(projects []string) error {
	if len(projects) == 0 {
		return errors.New("usage: tendr stop <project names...>")
	}

	loaded, err := loadProjects(projects)
	if err != nil {
		return err
	}
	client := herdr.New("", a.logger)
	lifecycle := manager.New(client, manager.NewDefaultShell(a.logger))
	for index, cfg := range loaded {
		if err := lifecycle.Stop(context.Background(), projects[index], cfg); err != nil {
			return fmt.Errorf("stop project %q: %w", projects[index], err)
		}
	}
	return nil
}

func loadProjects(projects []string) ([]*config.Config, error) {
	dir, err := expandHome(ProjectsDir)
	if err != nil {
		return nil, fmt.Errorf("resolve projects directory: %w", err)
	}

	configs := make([]*config.Config, 0, len(projects))
	var invalid []string
	for _, project := range projects {
		if !validProjectName(project) {
			invalid = append(invalid, fmt.Sprintf("%s (invalid project name)", project))
			continue
		}
		cfg, err := config.Load(filepath.Join(dir, project+".yml"))
		if err != nil {
			invalid = append(invalid, fmt.Sprintf("%s (%v)", project, err))
			continue
		}
		configs = append(configs, cfg)
	}
	if len(invalid) > 0 {
		return nil, fmt.Errorf("invalid projects: %s", strings.Join(invalid, ", "))
	}
	return configs, nil
}

func validProjectName(project string) bool {
	return project != "" && filepath.Base(project) == project
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
