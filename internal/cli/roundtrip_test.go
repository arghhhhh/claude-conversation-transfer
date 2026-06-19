package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/arghhhhh/claude-conversation-transfer/internal/encode"
)

// fixedTime is used so export filenames are deterministic.
func fixedTime() time.Time { return time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC) }

// buildProject creates a fake ~/.claude/projects/<enc> tree under home and
// returns the home path. The source CWD is `srcCWD` (so embedded paths in
// the .jsonl files match what Claude Code would have written).
func buildProject(t *testing.T, home, srcCWD string) {
	t.Helper()
	enc := encode.Encode(srcCWD)
	proj := filepath.Join(home, ".claude", "projects", enc)
	if err := os.MkdirAll(filepath.Join(proj, "memory"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(proj, "sess1", "subagents"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Top-level session.
	mustWrite(t, filepath.Join(proj, "sess1.jsonl"), buildLines(srcCWD))
	// Subagent transcript.
	mustWrite(t, filepath.Join(proj, "sess1", "subagents", "agent-1.jsonl"), buildLines(srcCWD))
	// Memory files (must be left alone by import).
	mustWrite(t, filepath.Join(proj, "memory", "MEMORY.md"), "- [Foo](foo.md) — bar\n")
	mustWrite(t, filepath.Join(proj, "memory", "foo.md"), "remember this\n")
}

func mustWrite(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

// buildLines returns three JSONL records with the source CWD embedded the
// way Claude Code stores it. We hand-construct the JSON so we can control
// the precise escape forms.
func buildLines(cwd string) string {
	mk := func(rec map[string]any) string {
		b, _ := json.Marshal(rec)
		return string(b) + "\n"
	}
	nested := cwd
	if strings.Contains(cwd, `\`) {
		nested = cwd + `\sub\file.txt`
	} else {
		nested = cwd + `/sub/file.txt`
	}
	return mk(map[string]any{"type": "user", "cwd": cwd, "content": "hi"}) +
		mk(map[string]any{"type": "assistant", "cwd": cwd, "toolUseResult": map[string]any{"file_path": nested}}) +
		mk(map[string]any{"type": "user", "cwd": cwd, "content": "see " + nested + " please"})
}

func countTurns(t *testing.T, root string) (int, int) {
	t.Helper()
	u, a := 0, 0
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if filepath.Ext(d.Name()) != ".jsonl" {
			return nil
		}
		data, _ := os.ReadFile(p)
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var v map[string]any
			if err := json.Unmarshal([]byte(line), &v); err != nil {
				t.Fatalf("%s: invalid JSON after import: %v\n%s", p, err, line)
			}
			if t2, ok := v["type"].(string); ok {
				if t2 == "user" {
					u++
				} else if t2 == "assistant" {
					a++
				}
			}
		}
		return nil
	})
	return u, a
}

// TestRoundTripWindowsToPOSIX exports a Windows-shaped project, imports it
// onto a "POSIX host" pointed at a different CWD, then asserts user/assistant
// turn counts match the original and the rewrite happened.
func TestRoundTripWindowsToPOSIX(t *testing.T) {
	homeA := t.TempDir() // source machine
	homeB := t.TempDir() // target machine
	outDir := t.TempDir()

	srcCWD := `C:\Users\joss\Desktop\Projects\Mine\proj`
	tgtCWD := `/Users/joss/work/Mine/proj`
	buildProject(t, homeA, srcCWD)

	expU, expA := countTurns(t, filepath.Join(homeA, ".claude", "projects", encode.Encode(srcCWD)))

	exp, err := RunExport(ExportOpts{CWD: srcCWD, OutDir: outDir, Now: fixedTime, Home: homeA})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(exp.Archive, "claude-convo-export-C--Users-joss-Desktop-Projects-Mine-proj-20260619-120000.zip") {
		t.Fatalf("filename: %s", exp.Archive)
	}

	imp, err := RunImport(ImportOpts{
		Zip:       exp.Archive,
		TargetCWD: tgtCWD,
		Now:       fixedTime,
		Home:      homeB,
		HostOS:    encode.POSIX,
	})
	if err != nil {
		t.Fatal(err)
	}
	if imp.Verification.TotalBad != 0 {
		t.Fatalf("bad lines after import: %+v", imp.Verification)
	}
	gotU, gotA := countTurns(t, imp.Target)
	if gotU != expU || gotA != expA {
		t.Fatalf("turn count mismatch: want u=%d a=%d got u=%d a=%d", expU, expA, gotU, gotA)
	}
	if imp.FilesRewritten == 0 {
		t.Fatal("expected at least one .jsonl rewritten")
	}
	if imp.TailDirection != `\->/` {
		t.Fatalf("direction: %q", imp.TailDirection)
	}
	// Memory files were imported and untouched.
	mem, err := os.ReadFile(filepath.Join(imp.Target, "memory", "MEMORY.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mem), "Foo") {
		t.Fatalf("memory not preserved: %s", mem)
	}
	// Confirm subagent transcript was rewritten too.
	sub, err := os.ReadFile(filepath.Join(imp.Target, "sess1", "subagents", "agent-1.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sub), `/Users/joss/work/Mine/proj/sub/file.txt`) {
		t.Fatalf("subagent not rewritten: %s", sub)
	}
}

func TestRoundTripPOSIXToWindows(t *testing.T) {
	homeA := t.TempDir()
	homeB := t.TempDir()
	outDir := t.TempDir()

	srcCWD := `/Users/joss/work/Mine/proj`
	tgtCWD := `C:\Users\joss\Desktop\Projects\Mine\proj`
	buildProject(t, homeA, srcCWD)
	expU, expA := countTurns(t, filepath.Join(homeA, ".claude", "projects", encode.Encode(srcCWD)))

	exp, err := RunExport(ExportOpts{CWD: srcCWD, OutDir: outDir, Now: fixedTime, Home: homeA})
	if err != nil {
		t.Fatal(err)
	}
	imp, err := RunImport(ImportOpts{
		Zip:       exp.Archive,
		TargetCWD: tgtCWD,
		Now:       fixedTime,
		Home:      homeB,
		HostOS:    encode.Windows,
	})
	if err != nil {
		t.Fatal(err)
	}
	if imp.Verification.TotalBad != 0 {
		t.Fatalf("bad lines: %+v", imp.Verification)
	}
	gotU, gotA := countTurns(t, imp.Target)
	if gotU != expU || gotA != expA {
		t.Fatalf("turn mismatch u=%d/%d a=%d/%d", gotU, expU, gotA, expA)
	}
	// Confirm Windows-escaped form is present in the JSON.
	data, _ := os.ReadFile(filepath.Join(imp.Target, "sess1.jsonl"))
	if !strings.Contains(string(data), `C:\\Users\\joss\\Desktop\\Projects\\Mine\\proj\\sub\\file.txt`) {
		t.Fatalf("Windows-escaped form missing: %s", data)
	}
}

func TestSameOSNoOp(t *testing.T) {
	homeA := t.TempDir()
	homeB := t.TempDir()
	outDir := t.TempDir()
	cwd := `/home/joss/proj`
	buildProject(t, homeA, cwd)
	exp, err := RunExport(ExportOpts{CWD: cwd, OutDir: outDir, Now: fixedTime, Home: homeA})
	if err != nil {
		t.Fatal(err)
	}
	imp, err := RunImport(ImportOpts{
		Zip:       exp.Archive,
		TargetCWD: cwd,
		Now:       fixedTime,
		Home:      homeB,
		HostOS:    encode.POSIX,
	})
	if err != nil {
		t.Fatal(err)
	}
	if imp.FilesRewritten != 0 {
		t.Fatalf("expected no rewrites, got %d", imp.FilesRewritten)
	}
	if imp.Verification.TotalBad != 0 {
		t.Fatal("verification failed")
	}
}

func TestBackupOnExistingTarget(t *testing.T) {
	homeA := t.TempDir()
	homeB := t.TempDir()
	outDir := t.TempDir()
	cwd := `/home/joss/proj`
	buildProject(t, homeA, cwd)
	// Pre-existing target.
	tgt := filepath.Join(homeB, ".claude", "projects", encode.Encode(cwd))
	os.MkdirAll(tgt, 0o755)
	mustWrite(t, filepath.Join(tgt, "stale.jsonl"), "{}\n")

	exp, err := RunExport(ExportOpts{CWD: cwd, OutDir: outDir, Now: fixedTime, Home: homeA})
	if err != nil {
		t.Fatal(err)
	}
	imp, err := RunImport(ImportOpts{
		Zip:       exp.Archive,
		TargetCWD: cwd,
		Now:       fixedTime,
		Home:      homeB,
		HostOS:    encode.POSIX,
	})
	if err != nil {
		t.Fatal(err)
	}
	if imp.Backup == "" {
		t.Fatal("expected backup path to be set")
	}
	if _, err := os.Stat(filepath.Join(imp.Backup, "stale.jsonl")); err != nil {
		t.Fatalf("backup missing stale.jsonl: %v", err)
	}
}

func TestUnderscorePrefixToleranceFilenameFallback(t *testing.T) {
	// Build a project with no .jsonl cwd to force the filename fallback path.
	homeA := t.TempDir()
	homeB := t.TempDir()
	outDir := t.TempDir()
	cwd := `/home/joss/proj`
	enc := encode.Encode(cwd)
	proj := filepath.Join(homeA, ".claude", "projects", enc)
	os.MkdirAll(proj, 0o755)
	// A .jsonl without any cwd field.
	mustWrite(t, filepath.Join(proj, "sess.jsonl"), `{"type":"user","content":"hi"}`+"\n")

	exp, err := RunExport(ExportOpts{CWD: cwd, OutDir: outDir, Now: fixedTime, Home: homeA})
	if err != nil {
		t.Fatal(err)
	}
	// Rename the archive to the underscore variant.
	underscored := strings.ReplaceAll(filepath.Base(exp.Archive), "-", "_")
	newPath := filepath.Join(filepath.Dir(exp.Archive), underscored)
	if err := os.Rename(exp.Archive, newPath); err != nil {
		t.Fatal(err)
	}
	imp, err := RunImport(ImportOpts{
		Zip:       newPath,
		TargetCWD: cwd,
		Now:       fixedTime,
		Home:      homeB,
		HostOS:    encode.POSIX,
	})
	if err != nil {
		t.Fatalf("underscore variant rejected: %v", err)
	}
	if imp.SourceCWD != cwd {
		t.Fatalf("source cwd: got %q want %q", imp.SourceCWD, cwd)
	}
}
