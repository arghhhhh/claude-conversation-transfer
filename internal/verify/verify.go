// Package verify reads every .jsonl line back as JSON and counts turn types.
// A successful import isn't proven by Claude Code showing the session in its
// list (sidecar records survive most corruption) — only by every line still
// parsing cleanly.
package verify

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type FileReport struct {
	Path           string `json:"path"`
	OKLines        int    `json:"ok_lines"`
	BadLines       int    `json:"bad_lines"`
	UserTurns      int    `json:"user_turns"`
	AssistantTurns int    `json:"assistant_turns"`
	FirstError     string `json:"first_error,omitempty"`
}

type Report struct {
	Files    []FileReport `json:"files"`
	TotalOK  int          `json:"total_ok"`
	TotalBad int          `json:"total_bad"`
}

// Walk scans root for .jsonl files and verifies each one. Returns the
// aggregated report. The caller decides how to react to TotalBad > 0.
func Walk(root string) (Report, error) {
	var rep Report
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".jsonl") {
			return nil
		}
		fr, err := verifyFile(path)
		if err != nil {
			return err
		}
		rep.Files = append(rep.Files, fr)
		rep.TotalOK += fr.OKLines
		rep.TotalBad += fr.BadLines
		return nil
	})
	return rep, err
}

func verifyFile(path string) (FileReport, error) {
	fr := FileReport{Path: path}
	f, err := os.Open(path)
	if err != nil {
		return fr, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var v map[string]any
		if err := json.Unmarshal(line, &v); err != nil {
			fr.BadLines++
			if fr.FirstError == "" {
				fr.FirstError = err.Error()
			}
			continue
		}
		fr.OKLines++
		if t, ok := v["type"].(string); ok {
			switch t {
			case "user":
				fr.UserTurns++
			case "assistant":
				fr.AssistantTurns++
			}
		}
	}
	if err := sc.Err(); err != nil {
		return fr, err
	}
	return fr, nil
}
