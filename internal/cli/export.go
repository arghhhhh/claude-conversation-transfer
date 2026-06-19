package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/arghhhhh/claude-conversation-transfer/internal/archive"
	"github.com/arghhhhh/claude-conversation-transfer/internal/encode"
)

type ExportOpts struct {
	CWD     string
	OutDir  string
	Now     func() time.Time // overridable for tests
	Home    string           // overridable for tests
}

func RunExport(opts ExportOpts) (ExportReport, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	projects, err := ProjectsDir(opts.Home)
	if err != nil {
		return ExportReport{}, err
	}
	enc := encode.Encode(opts.CWD)
	src := filepath.Join(projects, enc)
	if _, err := os.Stat(src); err != nil {
		return ExportReport{}, fmt.Errorf("no project folder for cwd %s (looked for %s): %w", opts.CWD, src, err)
	}
	ts := opts.Now().Format("20060102-150405")
	name := fmt.Sprintf("claude-convo-export-%s-%s.zip", enc, ts)
	outPath := filepath.Join(opts.OutDir, name)
	if _, err := archive.Create(src, outPath); err != nil {
		return ExportReport{}, err
	}
	info, err := os.Stat(outPath)
	if err != nil {
		return ExportReport{}, err
	}
	jcount, hasMem := surveyTree(src)
	return ExportReport{
		Archive:    outPath,
		SizeBytes:  info.Size(),
		JSONLFiles: jcount,
		HasMemory:  hasMem,
		Source:     src,
	}, nil
}

func surveyTree(root string) (int, bool) {
	jcount := 0
	hasMem := false
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "memory" {
				hasMem = true
			}
			return nil
		}
		if filepath.Ext(d.Name()) == ".jsonl" {
			jcount++
		}
		return nil
	})
	return jcount, hasMem
}
