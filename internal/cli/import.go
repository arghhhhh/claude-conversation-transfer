package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/arghhhhh/claude-conversation-transfer/internal/archive"
	"github.com/arghhhhh/claude-conversation-transfer/internal/encode"
	"github.com/arghhhhh/claude-conversation-transfer/internal/rewrite"
	"github.com/arghhhhh/claude-conversation-transfer/internal/verify"
)

type ImportOpts struct {
	Zip       string
	TargetCWD string
	Now       func() time.Time
	Home      string
	HostOS    encode.OS // overridable for cross-platform tests
}

func RunImport(opts ImportOpts) (ImportReport, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return ImportReport{}, err
		}
		opts.Home = h
	}
	if opts.HostOS == encode.Unknown {
		if runtime.GOOS == "windows" {
			opts.HostOS = encode.Windows
		} else {
			opts.HostOS = encode.POSIX
		}
	}

	// 1. Detect source CWD/OS.
	srcCWD, _ := archive.PeekCWD(opts.Zip)
	var srcOS encode.OS
	if srcCWD != "" {
		srcOS = encode.DetectOS(srcCWD)
	}
	if srcCWD == "" {
		// Fallback: parse from filename.
		base := filepath.Base(opts.Zip)
		enc := stripExportPrefix(base)
		if enc == "" {
			return ImportReport{}, fmt.Errorf("cannot derive source cwd from archive name %q", base)
		}
		cwd, os2, err := encode.DecodeFilenameHint(enc)
		if err != nil {
			return ImportReport{}, fmt.Errorf("cannot decode source cwd from filename: %w", err)
		}
		srcCWD = cwd
		srcOS = os2
	}

	// 2. Compute target folder.
	tgtEnc := encode.Encode(opts.TargetCWD)
	target := filepath.Join(opts.Home, ".claude", "projects", tgtEnc)

	// 3. Backup any existing folder.
	backup := ""
	if _, err := os.Stat(target); err == nil {
		ts := opts.Now().Format("20060102-150405")
		backup = target + ".bak-" + ts
		if err := os.Rename(target, backup); err != nil {
			return ImportReport{}, fmt.Errorf("backup failed: %w", err)
		}
	}

	// 4. Extract.
	if err := os.MkdirAll(target, 0o755); err != nil {
		return ImportReport{}, err
	}
	if err := archive.Extract(opts.Zip, target); err != nil {
		return ImportReport{}, err
	}

	// 5. Rewrite if needed.
	filesRewritten := 0
	tailConv := 0
	direction := ""
	needRewrite := srcCWD != opts.TargetCWD
	if needRewrite {
		err := filepath.WalkDir(target, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			if !strings.EqualFold(filepath.Ext(d.Name()), ".jsonl") {
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			out, changed, n := rewrite.Apply(data, srcCWD, srcOS, opts.TargetCWD, opts.HostOS)
			if changed {
				if err := os.WriteFile(p, out, 0o644); err != nil {
					return err
				}
				filesRewritten++
			}
			tailConv += n
			return nil
		})
		if err != nil {
			return ImportReport{}, err
		}
		direction = rewrite.DirectionLabel(srcOS, opts.HostOS)
	}

	// 6. Verify.
	vrep, err := verify.Walk(target)
	if err != nil {
		return ImportReport{}, err
	}

	jcount, hasMem := surveyTree(target)

	rep := ImportReport{
		Target:          target,
		JSONLFiles:      jcount,
		HasMemory:       hasMem,
		Backup:          backup,
		SourceCWD:       srcCWD,
		SourceOS:        srcOS.String(),
		TargetCWD:       opts.TargetCWD,
		TargetOS:        opts.HostOS.String(),
		FilesRewritten:  filesRewritten,
		TailConversions: tailConv,
		TailDirection:   direction,
		OutOfCWDIgnored: "References outside the project CWD were not rewritten and may still 404 on this machine.",
		Verification:    vrep,
	}
	return rep, nil
}

// stripExportPrefix removes the export filename prefix and trailing timestamp.
var (
	exportFilenameRE = regexp.MustCompile(`(?i)^claude[-_]convo[-_]export[-_](.*?)[-_]\d{8}([-_]\d{6})?\.zip$`)
)

func stripExportPrefix(name string) string {
	m := exportFilenameRE.FindStringSubmatch(name)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}
