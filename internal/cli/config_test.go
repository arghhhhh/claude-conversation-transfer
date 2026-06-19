package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectsDirDefault(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	got, err := ProjectsDir("/h")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/h", ".claude", "projects")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestProjectsDirRespectsEnvVar(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	got, err := ProjectsDir("/ignored")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "projects")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRunImportRespectsEnvVar(t *testing.T) {
	homeA := t.TempDir()
	tgtConfig := t.TempDir() // simulates CLAUDE_CONFIG_DIR on target
	outDir := t.TempDir()
	cwd := `/home/joss/proj`
	buildProject(t, homeA, cwd)
	exp, err := RunExport(ExportOpts{CWD: cwd, OutDir: outDir, Now: fixedTime, Home: homeA})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", tgtConfig)
	_, err = RunImport(ImportOpts{
		Zip:       exp.Archive,
		TargetCWD: cwd,
		Now:       fixedTime,
		Home:      t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	// Project should land under $CLAUDE_CONFIG_DIR/projects, not the Home.
	expected := filepath.Join(tgtConfig, "projects", "-home-joss-proj")
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected import under %s: %v", expected, err)
	}
}
