package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DirectionRight = "right"
	DirectionDown  = "down"
)

type Pane struct {
	Direction string   `yaml:"direction"`
	Ratio     *float64 `yaml:"ratio,omitempty"`
	Root      string   `yaml:"root,omitempty"`
	Commands  []string `yaml:"commands,omitempty"`
}

type Tab struct {
	Label    string   `yaml:"label"`
	Root     string   `yaml:"root,omitempty"`
	Commands []string `yaml:"commands,omitempty"`
	Panes    []Pane   `yaml:"panes,omitempty"`
}

type Workspace struct {
	Label       string   `yaml:"label"`
	Root        string   `yaml:"root,omitempty"`
	BeforeStart []string `yaml:"before_start,omitempty"`
	AfterStart  []string `yaml:"after_start,omitempty"`
	Tabs        []Tab    `yaml:"tabs"`
}

type Config struct {
	Remote      string      `yaml:"remote,omitempty"`
	Root        string      `yaml:"root"`
	BeforeStart []string    `yaml:"before_start,omitempty"`
	AfterStart  []string    `yaml:"after_start,omitempty"`
	BeforeStop  []string    `yaml:"before_stop,omitempty"`
	AfterStop   []string    `yaml:"after_stop,omitempty"`
	Workspaces  []Workspace `yaml:"workspaces"`
}

func Load(name string) (*Config, error) {
	path, err := expandHome(name)
	if err != nil {
		return nil, fmt.Errorf("expand config path: %w", err)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)

	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, fmt.Errorf("decode config: %w", err)
		}
		return nil, errors.New("decode config: multiple YAML documents are not allowed")
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := cfg.resolvePaths(filepath.Dir(path)); err != nil {
		return nil, fmt.Errorf("resolve paths: %w", err)
	}
	return &cfg, nil
}

func (c Config) Validate() error {
	var problems []string
	if strings.TrimSpace(c.Root) == "" {
		problems = append(problems, "root is required")
	}
	validateCommands(&problems, "before_start", c.BeforeStart)
	validateCommands(&problems, "after_start", c.AfterStart)
	validateCommands(&problems, "before_stop", c.BeforeStop)
	validateCommands(&problems, "after_stop", c.AfterStop)
	if len(c.Workspaces) == 0 {
		problems = append(problems, "workspaces must contain at least one workspace")
	}

	workspaceLabels := make(map[string]struct{}, len(c.Workspaces))
	for wi, workspace := range c.Workspaces {
		workspacePath := fmt.Sprintf("workspaces[%d]", wi)
		validateLabel(&problems, workspaceLabels, workspacePath, workspace.Label)
		validateCommands(&problems, workspacePath+".before_start", workspace.BeforeStart)
		validateCommands(&problems, workspacePath+".after_start", workspace.AfterStart)
		if len(workspace.Tabs) == 0 {
			problems = append(problems, workspacePath+".tabs must contain at least one tab")
		}

		tabLabels := make(map[string]struct{}, len(workspace.Tabs))
		for ti, tab := range workspace.Tabs {
			tabPath := fmt.Sprintf("%s.tabs[%d]", workspacePath, ti)
			validateLabel(&problems, tabLabels, tabPath, tab.Label)
			validateCommands(&problems, tabPath+".commands", tab.Commands)
			for pi, pane := range tab.Panes {
				panePath := fmt.Sprintf("%s.panes[%d]", tabPath, pi)
				if pane.Direction != DirectionRight && pane.Direction != DirectionDown {
					problems = append(problems, panePath+".direction must be right or down")
				}
				if pane.Ratio != nil && (*pane.Ratio <= 0 || *pane.Ratio >= 1) {
					problems = append(problems, panePath+".ratio must be greater than 0 and less than 1")
				}
				validateCommands(&problems, panePath+".commands", pane.Commands)
			}
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("invalid config: %s", strings.Join(problems, "; "))
	}
	return nil
}

func validateLabel(problems *[]string, labels map[string]struct{}, path, label string) {
	label = strings.TrimSpace(label)
	if label == "" {
		*problems = append(*problems, path+".label is required")
		return
	}
	if _, exists := labels[label]; exists {
		*problems = append(*problems, path+".label must be unique")
		return
	}
	labels[label] = struct{}{}
}

func validateCommands(problems *[]string, path string, commands []string) {
	for i, command := range commands {
		if strings.TrimSpace(command) == "" {
			*problems = append(*problems, fmt.Sprintf("%s[%d] must not be empty", path, i))
		}
	}
}

func (c *Config) resolvePaths(configDir string) error {
	// Remote roots belong to the remote host. In particular, do not expand ~
	// using the local user's home directory.
	if c.Remote != "" {
		configDir = ""
	}
	root, err := resolvePath(configDir, c.Root)
	if err != nil {
		return fmt.Errorf("root: %w", err)
	}
	c.Root = root

	for wi := range c.Workspaces {
		workspace := &c.Workspaces[wi]
		workspace.Root, err = resolvePath(c.Root, workspace.Root)
		if err != nil {
			return fmt.Errorf("workspaces[%d].root: %w", wi, err)
		}
		for ti := range workspace.Tabs {
			tab := &workspace.Tabs[ti]
			tab.Root, err = resolvePath(workspace.Root, tab.Root)
			if err != nil {
				return fmt.Errorf("workspaces[%d].tabs[%d].root: %w", wi, ti, err)
			}
			for pi := range tab.Panes {
				pane := &tab.Panes[pi]
				pane.Root, err = resolvePath(tab.Root, pane.Root)
				if err != nil {
					return fmt.Errorf("workspaces[%d].tabs[%d].panes[%d].root: %w", wi, ti, pi, err)
				}
			}
		}
	}
	return nil
}

func resolvePath(parent, child string) (string, error) {
	if child == "" {
		return filepath.Clean(parent), nil
	}
	if parent == "" {
		if strings.HasPrefix(child, "~") {
			return "", fmt.Errorf("remote roots must be absolute (got %q)", child)
		}
		if !filepath.IsAbs(child) {
			return "", fmt.Errorf("remote project root must be absolute (got %q)", child)
		}
		return filepath.Clean(child), nil
	}
	expanded, err := expandHome(child)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(expanded) {
		return filepath.Clean(expanded), nil
	}
	return filepath.Clean(filepath.Join(parent, expanded)), nil
}

func expandHome(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	if strings.HasPrefix(path, "~") {
		return "", fmt.Errorf("unsupported home path %q", path)
	}
	return path, nil
}
