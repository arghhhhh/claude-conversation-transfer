package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/arghhhhh/claude-conversation-transfer/internal/encode"
)

type RenameOpts struct {
	FromCWD   string    // current project CWD (source)
	ToCWD     string    // new project CWD (target)
	RenameDir bool      // also move the on-disk project directory (opt-in)
	Now       func() time.Time
	Home      string    // overridable for tests
	HostOS    encode.OS // overridable for cross-platform tests
}

// DirRenameError marks the failure of the optional on-disk directory move so
// the caller can map it to a distinct exit code. Nothing under
// ~/.claude/projects/ has been mutated when this is returned.
type DirRenameError struct{ Err error }

func (e *DirRenameError) Error() string { return "directory rename failed: " + e.Err.Error() }
func (e *DirRenameError) Unwrap() error { return e.Err }

// RunRename migrates a project's Claude-side data from FromCWD's encoded
// folder to ToCWD's, composing the tested export+import path (via a temp zip)
// rather than duplicating rewrite logic. On a clean verify it deletes the old
// encoded folder. With RenameDir, it also moves the working directory on disk.
//
// A rename is single-machine, so the projects directory is shared between the
// old and new encoded folders (one Home / one CLAUDE_CONFIG_DIR).
func RunRename(opts RenameOpts) (RenameReport, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.FromCWD == opts.ToCWD {
		return RenameReport{}, fmt.Errorf("source and target CWD are identical (%s); nothing to rename", opts.FromCWD)
	}
	projects, err := ProjectsDir(opts.Home)
	if err != nil {
		return RenameReport{}, err
	}
	oldProj := filepath.Join(projects, encode.Encode(opts.FromCWD))
	if _, err := os.Stat(oldProj); err != nil {
		return RenameReport{}, fmt.Errorf("no project folder for source cwd %s (looked for %s): %w", opts.FromCWD, oldProj, err)
	}

	// 1. Export the old project to a throwaway zip.
	tmp, err := os.MkdirTemp("", "ccr-rename-")
	if err != nil {
		return RenameReport{}, err
	}
	defer os.RemoveAll(tmp)
	exp, err := RunExport(ExportOpts{CWD: opts.FromCWD, OutDir: tmp, Now: opts.Now, Home: opts.Home})
	if err != nil {
		return RenameReport{}, err
	}

	// 2. Optionally move the working directory on disk. Do this AFTER a
	//    successful export (so we still have the data) but BEFORE deleting
	//    anything — if it fails, the Claude-side state is untouched.
	dirRenamed := false
	if opts.RenameDir {
		if err := renameProjectDir(opts.FromCWD, opts.ToCWD); err != nil {
			return RenameReport{}, &DirRenameError{Err: err}
		}
		dirRenamed = true
	}

	// 3. Import into the new encoded folder, rewriting paths and verifying.
	imp, err := RunImport(ImportOpts{
		Zip:       exp.Archive,
		TargetCWD: opts.ToCWD,
		Now:       opts.Now,
		Home:      opts.Home,
		HostOS:    opts.HostOS,
	})
	if err != nil {
		return RenameReport{}, err
	}

	rep := RenameReport{
		OldProject:              oldProj,
		NewProject:              imp.Target,
		OldCWD:                  opts.FromCWD,
		NewCWD:                  opts.ToCWD,
		JSONLFiles:              imp.JSONLFiles,
		HasMemory:               imp.HasMemory,
		DirRenamed:              dirRenamed,
		PreexistingTargetBackup: imp.Backup,
		FilesRewritten:          imp.FilesRewritten,
		TailConversions:         imp.TailConversions,
		TailDirection:           imp.TailDirection,
		Verification:            imp.Verification,
	}

	// 4. Delete the old encoded folder ONLY on a clean verify, and never if it
	//    resolved to the same folder we just wrote into.
	if imp.Verification.TotalBad == 0 &&
		filepath.Clean(oldProj) != filepath.Clean(imp.Target) {
		if err := os.RemoveAll(oldProj); err != nil {
			return rep, fmt.Errorf("migration verified but removing old project folder failed: %w", err)
		}
		rep.OldDataDeleted = true
	}
	return rep, nil
}

// renameProjectDir moves the on-disk working directory from -> to. It refuses
// to clobber an existing target and requires the target's parent to exist. On
// Windows a directory cannot be renamed while it is a live process's CWD, so
// if we are ourselves sitting inside `from` we first step out to its parent to
// release the handle (harmless no-op otherwise).
func renameProjectDir(from, to string) error {
	if _, err := os.Stat(to); err == nil {
		return fmt.Errorf("target directory already exists: %s", to)
	}
	if parent := filepath.Dir(to); parent != "" && parent != to {
		if _, err := os.Stat(parent); err != nil {
			return fmt.Errorf("target parent directory does not exist: %s", parent)
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		if cwd == from || strings.HasPrefix(cwd, from+string(os.PathSeparator)) {
			_ = os.Chdir(filepath.Dir(from))
		}
	}
	return os.Rename(from, to)
}
