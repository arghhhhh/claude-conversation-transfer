package verify

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWalk(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "a.jsonl")
	bad := filepath.Join(dir, "b.jsonl")
	os.WriteFile(good, []byte(`{"type":"user"}`+"\n"+`{"type":"assistant"}`+"\n"), 0o644)
	os.WriteFile(bad, []byte(`{"type":"user"`+"\n"), 0o644)
	rep, err := Walk(dir)
	if err != nil {
		t.Fatal(err)
	}
	if rep.TotalOK != 2 || rep.TotalBad != 1 {
		t.Fatalf("got %+v", rep)
	}
}
