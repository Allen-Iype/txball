// Package score persists the high score.
package score

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Path returns where the high score lives: $XDG_CONFIG_HOME/txball/highscore,
// or ~/.config/txball/highscore when that isn't set. (os.UserConfigDir is
// not used: on macOS it points into ~/Library.)
func Path() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if !filepath.IsAbs(dir) { // unset, or relative and so invalid per the spec
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("find home directory: %w", err)
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "txball", "highscore"), nil
}

// Load reads the high score. A missing file is a score of zero, not an
// error.
func Load(path string) (int, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%s: not a score: %q", path, strings.TrimSpace(string(data)))
	}
	return n, nil
}

// Save writes the high score, creating the directory if need be. The file
// is replaced atomically, so a crash can't leave it half-written.
func Save(path string, score int) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".highscore-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // a no-op once renamed
	if _, err := fmt.Fprintf(tmp, "%d\n", score); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
