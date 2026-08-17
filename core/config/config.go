package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	dirName     = "git-rewind"
	fileName    = "config.json"
	dirPerm     = 0o700
	filePerm    = 0o600
	maxFileSize = 64 * 1024
)

// Config holds the user preferences git-rewind remembers between runs.
type Config struct {
	Language string `json:"language"`
}

// Path returns the configuration file's location inside the OS user configuration directory.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locating the user configuration directory: %w", err)
	}
	return filepath.Join(dir, dirName, fileName), nil
}

// Load reads the configuration file, returning defaults when it does not exist yet.
func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	return LoadFrom(path)
}

// LoadFrom reads a configuration file by path, returning defaults when it does not exist.
func LoadFrom(path string) (Config, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // G304: the path is derived from os.UserConfigDir or supplied by a test.
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("reading %s: %w", path, err)
	}
	if len(raw) > maxFileSize {
		return Config{}, fmt.Errorf("reading %s: file is larger than %d bytes", path, maxFileSize)
	}

	var c Config
	if err := json.Unmarshal(raw, &c); err != nil {
		return Config{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	return c, nil
}

// Save writes the configuration file, creating its directory when needed.
func Save(c Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	return SaveTo(path, c)
}

// SaveTo writes a configuration file to path, creating its directory when needed.
func SaveTo(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}

	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding the configuration: %w", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), filePerm); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
