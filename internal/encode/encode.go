package encode

import (
	"errors"
	"strings"
)

type OS int

const (
	Unknown OS = iota
	Windows
	POSIX
)

func (o OS) String() string {
	switch o {
	case Windows:
		return "windows"
	case POSIX:
		return "posix"
	}
	return "unknown"
}

// DetectOS classifies a path string by its shape.
//   - "C:\..." or "C:/..." => Windows
//   - "/..." => POSIX
func DetectOS(p string) OS {
	if len(p) >= 2 && isLetter(p[0]) && p[1] == ':' {
		return Windows
	}
	if strings.HasPrefix(p, "/") {
		return POSIX
	}
	if strings.Contains(p, "\\") {
		return Windows
	}
	return Unknown
}

func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// Encode produces the project-folder name Claude Code uses for a CWD.
// Windows: every ':' '\' '/' becomes '-'.
// POSIX:   every '/' becomes '-'.
func Encode(cwd string) string {
	os := DetectOS(cwd)
	r := strings.NewReplacer
	switch os {
	case Windows:
		return r(":", "-", "\\", "-", "/", "-").Replace(cwd)
	default:
		return strings.ReplaceAll(cwd, "/", "-")
	}
}

// DecodeFilenameHint parses an encoded folder name back into a CWD. This is a
// best-effort fallback used only when the source cwd cannot be read directly
// from inside the zip; separator drift in the filename can make it lossy.
func DecodeFilenameHint(enc string) (string, OS, error) {
	if enc == "" {
		return "", Unknown, errors.New("empty encoded name")
	}
	// Normalize underscores to hyphens.
	enc = strings.ReplaceAll(enc, "_", "-")
	// Drive-letter: <L>--rest  =>  <L>:\rest-with-dashes-as-backslashes
	if len(enc) >= 3 && isLetter(enc[0]) && enc[1] == '-' && enc[2] == '-' {
		drive := string(enc[0]) + ":\\"
		rest := strings.ReplaceAll(enc[3:], "-", "\\")
		return drive + rest, Windows, nil
	}
	// Single-dash drive variant (some collapsers): <L>-rest
	if len(enc) >= 2 && isLetter(enc[0]) && enc[1] == '-' {
		drive := string(enc[0]) + ":\\"
		rest := strings.ReplaceAll(enc[2:], "-", "\\")
		return drive + rest, Windows, nil
	}
	// POSIX: leading '-' means leading '/'.
	if strings.HasPrefix(enc, "-") {
		return "/" + strings.ReplaceAll(enc[1:], "-", "/"), POSIX, nil
	}
	return "", Unknown, errors.New("cannot decode: " + enc)
}
