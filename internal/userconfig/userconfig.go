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
	"regexp"

	"go.yaml.in/yaml/v3"
)

// EnvUser overrides the `user` setting.
const EnvUser = "KARDBOARD_USER"

var userRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// Config is the content of the user's config.yaml.
type Config struct {
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

// User returns the user's own short name, from KARDBOARD_USER or the
// settings file. It is the name of their directory in a per-person column.
func User() (string, error) {
	name := os.Getenv(EnvUser)
	if name == "" {
		cfg, err := Load()
		if err != nil {
			return "", err
		}
		name = cfg.User
	}
	if name == "" {
		path, _ := Path()
		return "", fmt.Errorf("no user set: add `user: <name>` to %s or set %s", path, EnvUser)
	}
	if !userRe.MatchString(name) {
		return "", fmt.Errorf("user %q is not a valid name (a-z, 0-9, -)", name)
	}
	return name, nil
}
