package cli

import (
	"errors"
	"os"
	"path/filepath"
)

// ProjectsDir returns the absolute path to the Claude Code projects directory
// for a given installation. The precedence matches Claude Code itself:
//
//  1. $CLAUDE_CONFIG_DIR/projects  if CLAUDE_CONFIG_DIR is set
//  2. <home>/.claude/projects      otherwise
//
// `home` may be empty, in which case os.UserHomeDir() is consulted.
func ProjectsDir(home string) (string, error) {
	if v := os.Getenv("CLAUDE_CONFIG_DIR"); v != "" {
		return filepath.Join(v, "projects"), nil
	}
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		home = h
	}
	if home == "" {
		return "", errors.New("cannot resolve home directory")
	}
	return filepath.Join(home, ".claude", "projects"), nil
}
