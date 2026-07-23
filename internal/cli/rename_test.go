package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/arghhhhh/claude-conversation-transfer/internal/encode"
)

// TestRenameWindowsSameMachine mirrors the motivating scenario: a project
// folder is renamed on one Windows machine (pod-design -> unfold-museum-pod-design).
// The Claude-side data migrates to the new encoded folder, embedded paths are
// rewritten, verification passes, and the old encoded folder is deleted.
func TestRenameWindowsSameMachine(t *testing.T) {
	home := t.TempDir()
	src := `C:\Users\joss\Desktop\museum\pod-design`
	tgt := `C:\Users\joss\Desktop\museum\unfold-museum-pod-design`
	buildProject(t, home, src)

	oldProj := filepath.Join(home, ".claude", "projects", encode.Encode(src))
	expU, expA := countTurns(t, oldProj)

	rep, err := RunRename(RenameOpts{
		FromCWD: src, ToCWD: tgt, Home: home, HostOS: encode.Windows, Now: fixedTime,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Verification.TotalBad != 0 {
		t.Fatalf("bad lines after rename: %+v", rep.Verification)
	}
	if !rep.OldDataDeleted {
		t.Fatal("expected old data to be deleted after clean verify")
	}
	if _, err := os.Stat(oldProj); !os.IsNotExist(err) {
		t.Fatalf("old project folder should be gone, stat err=%v", err)
	}
	newProj := filepath.Join(home, ".claude", "projects", encode.Encode(tgt))
	if rep.NewProject != newProj {
		t.Fatalf("new project: got %q want %q", rep.NewProject, newProj)
	}
	gotU, gotA := countTurns(t, newProj)
	if gotU != expU || gotA != expA {
		t.Fatalf("turn count mismatch: want u=%d a=%d got u=%d a=%d", expU, expA, gotU, gotA)
	}
	if rep.DirRenamed {
		t.Fatal("dir should not have been renamed without --rename-dir")
	}
	if rep.PreexistingTargetBackup != "" {
		t.Fatalf("unexpected preexisting backup: %s", rep.PreexistingTargetBackup)
	}
	// New encoded path must appear (rewritten), old must be gone from content.
	data, _ := os.ReadFile(filepath.Join(newProj, "sess1.jsonl"))
	if !strings.Contains(string(data), `unfold-museum-pod-design`) {
		t.Fatalf("new path not present after rewrite: %s", data)
	}
	if strings.Contains(string(data), `museum\\pod-design`) || strings.Contains(string(data), `museum/pod-design`) {
		t.Fatalf("old path still present after rewrite: %s", data)
	}
}

// TestRenameCrossOSRoundTrip exercises the historical backslash-transport path
// through rename: a Windows-shaped source migrated with a POSIX host still
// verifies and rewrites tail separators. (Same-machine cross-OS is unusual but
// keeps the round-trip guard equivalent to import's fixtures.)
func TestRenameCrossOSRoundTrip(t *testing.T) {
	home := t.TempDir()
	src := `C:\Users\joss\Desktop\Projects\Mine\proj`
	tgt := `/Users/joss/work/Mine/renamed`
	buildProject(t, home, src)
	oldProj := filepath.Join(home, ".claude", "projects", encode.Encode(src))
	expU, expA := countTurns(t, oldProj)

	rep, err := RunRename(RenameOpts{
		FromCWD: src, ToCWD: tgt, Home: home, HostOS: encode.POSIX, Now: fixedTime,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Verification.TotalBad != 0 {
		t.Fatalf("bad lines: %+v", rep.Verification)
	}
	if rep.TailDirection != `\->/` {
		t.Fatalf("direction: %q", rep.TailDirection)
	}
	newProj := filepath.Join(home, ".claude", "projects", encode.Encode(tgt))
	gotU, gotA := countTurns(t, newProj)
	if gotU != expU || gotA != expA {
		t.Fatalf("turn mismatch u=%d/%d a=%d/%d", gotU, expU, gotA, expA)
	}
	sub, _ := os.ReadFile(filepath.Join(newProj, "sess1", "subagents", "agent-1.jsonl"))
	if !strings.Contains(string(sub), `/Users/joss/work/Mine/renamed/sub/file.txt`) {
		t.Fatalf("subagent tail not rewritten: %s", sub)
	}
}

// TestRenamePreexistingTargetBackup: a project folder already exists at the new
// encoded path. Import backs it up; rename surfaces the backup and never
// deletes it, while still completing the migration and deleting the old folder.
func TestRenamePreexistingTargetBackup(t *testing.T) {
	home := t.TempDir()
	src := `/home/joss/pod-design`
	tgt := `/home/joss/unfold-museum-pod-design`
	buildProject(t, home, src)

	// Pre-existing target folder with a stale session.
	tgtProj := filepath.Join(home, ".claude", "projects", encode.Encode(tgt))
	if err := os.MkdirAll(tgtProj, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(tgtProj, "stale.jsonl"), "{}\n")

	rep, err := RunRename(RenameOpts{
		FromCWD: src, ToCWD: tgt, Home: home, HostOS: encode.POSIX, Now: fixedTime,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rep.PreexistingTargetBackup == "" {
		t.Fatal("expected preexisting target backup to be reported")
	}
	if _, err := os.Stat(filepath.Join(rep.PreexistingTargetBackup, "stale.jsonl")); err != nil {
		t.Fatalf("backup missing stale.jsonl: %v", err)
	}
	if !rep.OldDataDeleted {
		t.Fatal("old folder should still be deleted on clean verify")
	}
	oldProj := filepath.Join(home, ".claude", "projects", encode.Encode(src))
	if _, err := os.Stat(oldProj); !os.IsNotExist(err) {
		t.Fatalf("old project should be gone: %v", err)
	}
}

// TestRenameWithDirRename moves a real on-disk project directory via
// --rename-dir, using OS-native temp paths so encode/rewrite match the host.
func TestRenameWithDirRename(t *testing.T) {
	home := t.TempDir()
	workspace := t.TempDir()
	fromDir := filepath.Join(workspace, "pod-design")
	toDir := filepath.Join(workspace, "unfold-museum-pod-design")
	if err := os.MkdirAll(fromDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A file inside the working dir, to prove the whole directory moved.
	mustWrite(t, filepath.Join(fromDir, "marker.txt"), "hi\n")

	buildProject(t, home, fromDir)

	hostOS := encode.POSIX
	if runtime.GOOS == "windows" {
		hostOS = encode.Windows
	}
	rep, err := RunRename(RenameOpts{
		FromCWD: fromDir, ToCWD: toDir, RenameDir: true,
		Home: home, HostOS: hostOS, Now: fixedTime,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.DirRenamed {
		t.Fatal("expected DirRenamed=true")
	}
	if _, err := os.Stat(fromDir); !os.IsNotExist(err) {
		t.Fatalf("source dir should be gone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(toDir, "marker.txt")); err != nil {
		t.Fatalf("marker.txt missing in moved dir: %v", err)
	}
	if !rep.OldDataDeleted {
		t.Fatal("expected old Claude data deleted")
	}
}

// TestRenameIdenticalCWDRejected: refusing a no-op keeps us from deleting the
// folder we just wrote into.
func TestRenameIdenticalCWDRejected(t *testing.T) {
	home := t.TempDir()
	cwd := `/home/joss/proj`
	buildProject(t, home, cwd)
	_, err := RunRename(RenameOpts{FromCWD: cwd, ToCWD: cwd, Home: home, HostOS: encode.POSIX, Now: fixedTime})
	if err == nil {
		t.Fatal("expected error for identical source/target CWD")
	}
}

// TestRenameMissingSource: no project folder for the source CWD is an error,
// not a silent success.
func TestRenameMissingSource(t *testing.T) {
	home := t.TempDir()
	_, err := RunRename(RenameOpts{
		FromCWD: `/home/joss/does-not-exist`, ToCWD: `/home/joss/whatever`,
		Home: home, HostOS: encode.POSIX, Now: fixedTime,
	})
	if err == nil {
		t.Fatal("expected error for missing source project folder")
	}
}
