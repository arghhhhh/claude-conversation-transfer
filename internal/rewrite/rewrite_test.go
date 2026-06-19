package rewrite

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/arghhhhh/claude-conversation-transfer/internal/encode"
)

// TestWindowsToPOSIX verifies the prefix swap and tail conversion across the
// full path including a nested file reference.
func TestWindowsToPOSIX(t *testing.T) {
	src := `C:\Users\joss\Desktop\Projects\Mine`
	tgt := `/Users/joss/work/Mine`
	// What appears inside a .jsonl JSON string for a nested file on Windows:
	// "C:\\Users\\joss\\Desktop\\Projects\\Mine\\sub\\file.txt"
	line := `{"cwd":"C:\\Users\\joss\\Desktop\\Projects\\Mine","file":"C:\\Users\\joss\\Desktop\\Projects\\Mine\\sub\\file.txt"}`
	out, changed, n := Apply([]byte(line), src, encode.Windows, tgt, encode.POSIX)
	if !changed {
		t.Fatal("expected change")
	}
	if !strings.Contains(string(out), `"cwd":"/Users/joss/work/Mine"`) {
		t.Fatalf("cwd not rewritten: %s", out)
	}
	if !strings.Contains(string(out), `"file":"/Users/joss/work/Mine/sub/file.txt"`) {
		t.Fatalf("tail not converted: %s", out)
	}
	if n == 0 {
		t.Fatal("expected tail conversions > 0")
	}
	// Must remain valid JSON.
	var v map[string]any
	if err := json.Unmarshal(out, &v); err != nil {
		t.Fatalf("invalid JSON after rewrite: %v\n%s", err, out)
	}
}

// TestPOSIXToWindows verifies the symmetric direction.
func TestPOSIXToWindows(t *testing.T) {
	src := `/Users/joss/work/Mine`
	tgt := `C:\Users\joss\Desktop\Projects\Mine`
	line := `{"cwd":"/Users/joss/work/Mine","file":"/Users/joss/work/Mine/sub/file.txt"}`
	out, changed, n := Apply([]byte(line), src, encode.POSIX, tgt, encode.Windows)
	if !changed {
		t.Fatal("expected change")
	}
	if !strings.Contains(string(out), `"cwd":"C:\\Users\\joss\\Desktop\\Projects\\Mine"`) {
		t.Fatalf("cwd not rewritten: %s", out)
	}
	if !strings.Contains(string(out), `"file":"C:\\Users\\joss\\Desktop\\Projects\\Mine\\sub\\file.txt"`) {
		t.Fatalf("tail not converted: %s", out)
	}
	if n == 0 {
		t.Fatal("expected tail conversions > 0")
	}
	var v map[string]any
	if err := json.Unmarshal(out, &v); err != nil {
		t.Fatalf("invalid JSON after rewrite: %v\n%s", err, out)
	}
}

// TestTailScopingNotOverEager makes sure separators outside path tokens are
// left alone — e.g. a URL or code block in message content should not have
// its slashes flipped.
func TestTailScopingNotOverEager(t *testing.T) {
	src := `/Users/joss/work/Mine`
	tgt := `C:\Users\joss\proj`
	line := `{"msg":"see https://example.com/a/b and /Users/joss/work/Mine/foo bar"}`
	out, _, _ := Apply([]byte(line), src, encode.POSIX, tgt, encode.Windows)
	s := string(out)
	if !strings.Contains(s, `https://example.com/a/b`) {
		t.Fatalf("URL slashes were rewritten: %s", s)
	}
	if !strings.Contains(s, `C:\\Users\\joss\\proj\\foo`) {
		t.Fatalf("path tail not converted: %s", s)
	}
}

// TestNoOpSameOS confirms we do nothing when source and target match.
func TestNoOpSameOS(t *testing.T) {
	src := `C:\a\b`
	tgt := `C:\a\b`
	line := `{"cwd":"C:\\a\\b"}`
	out, changed, n := Apply([]byte(line), src, encode.Windows, tgt, encode.Windows)
	if changed || n != 0 {
		t.Fatalf("expected no change, got %s changed=%v n=%d", out, changed, n)
	}
}

// TestSameOSDifferentPath rewrites prefix but does not touch tail separators.
func TestSameOSDifferentPath(t *testing.T) {
	src := `C:\a\b`
	tgt := `C:\x\y`
	line := `{"file":"C:\\a\\b\\sub\\f.txt"}`
	out, changed, _ := Apply([]byte(line), src, encode.Windows, tgt, encode.Windows)
	if !changed {
		t.Fatal("expected change")
	}
	if !strings.Contains(string(out), `C:\\x\\y\\sub\\f.txt`) {
		t.Fatalf("got %s", out)
	}
}

// TestRegressionEscapedQuoteAfterPath guards the escape-corruption bug: a
// command line that embeds a path written with forward slashes and ends in an
// escaped quote (e.g. a quoted shell argument `cd ".../dir"`) must survive the
// Windows->POSIX rewrite. The historic single-backslash tail pass rewrote the
// `\` of the `\"` escape into `/`, un-escaping the quote and corrupting the
// record. The path here uses '/' (as the user typed it on Windows), so only
// the prefix swap should change anything and the JSON must still parse.
func TestRegressionEscapedQuoteAfterPath(t *testing.T) {
	src := `C:\Users\joss\Desktop\Projects\Mine\claude-project-transfer`
	tgt := `/Users/joss/Desktop/Projects/Mine/claude-conversations-transfer`
	// As stored in JSONL: a tool command string containing a forward-slash
	// path that the source already swapped to the target, ending in \" (an
	// escaped quote that closes the shell argument).
	line := `{"type":"assistant","command":"cd \"` + escForJSON(tgt) + `/_mermaid_test\" && echo \"done\""}`
	// Sanity: the constructed input must itself be valid JSON.
	var probe any
	if err := json.Unmarshal([]byte(line), &probe); err != nil {
		t.Fatalf("test input is not valid JSON: %v\n%s", err, line)
	}
	out, _, _ := Apply([]byte(line), src, encode.Windows, tgt, encode.POSIX)
	var v any
	if err := json.Unmarshal(out, &v); err != nil {
		t.Fatalf("invalid JSON after rewrite (escape corrupted): %v\nout: %s", err, out)
	}
	if !strings.Contains(string(out), `_mermaid_test\"`) {
		t.Fatalf("escaped quote after path was mangled: %s", out)
	}
}

// escForJSON doubles backslashes so a literal path can be embedded inside a Go
// string that is itself a JSON document under test.
func escForJSON(s string) string {
	return strings.ReplaceAll(s, `\`, `\\`)
}

// TestRegressionValidJSONAfterRewrite is the explicit guard for the
// backslash-transport bug: after rewriting, every JSON record must parse.
func TestRegressionValidJSONAfterRewrite(t *testing.T) {
	src := `C:\Users\joss\Desktop\Projects\Mine`
	tgt := `/Users/joss/work/Mine`
	// A realistic-shaped record with several embedded path forms.
	lines := []string{
		`{"type":"user","cwd":"C:\\Users\\joss\\Desktop\\Projects\\Mine","content":"hi"}`,
		`{"type":"assistant","toolUseResult":{"file_path":"C:\\Users\\joss\\Desktop\\Projects\\Mine\\src\\main.go"}}`,
		`{"type":"user","message":"check C:\\Users\\joss\\Desktop\\Projects\\Mine\\a\\b\\c.txt please"}`,
	}
	for _, l := range lines {
		out, _, _ := Apply([]byte(l), src, encode.Windows, tgt, encode.POSIX)
		var v any
		if err := json.Unmarshal(out, &v); err != nil {
			t.Fatalf("invalid JSON after rewrite: %v\nin:  %s\nout: %s", err, l, out)
		}
	}
}
