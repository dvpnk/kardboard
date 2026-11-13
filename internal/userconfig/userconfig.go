// Package userconfig reads the per-user settings in
// ~/.config/kardboard/config.yaml.
package userconfig

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// EnvUser overrides the `user` setting, see package people.
const EnvUser = "KARDBOARD_USER"

// Config is the content of the user's config.yaml.
type Config struct {
	// User is the user's name in every board, unless a project says
	// otherwise; see people.Me.
	User   string   `yaml:"user"`
	Boards []string `yaml:"boards"`
	Scan   []string `yaml:"scan"`
	Hidden []string `yaml:"hidden"`
}

// Path returns where the settings live. It honours XDG_CONFIG_HOME and
// otherwise uses ~/.config on every platform.
func Path() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "kardboard", "config.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "kardboard", "config.yaml"), nil
}

// Load reads the settings. A missing file is an empty configuration.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Config{}, nil
		}
		return nil, err
	}
	cfg := &Config{}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}
