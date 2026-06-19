// Package rewrite performs cross-OS path rewriting on .jsonl session files.
//
// All rewriting is done in-process on raw bytes. We never shell out — that
// avoids a known regression where Windows path literals (`C:\\...`) lose a
// backslash in an argv layer and produce invalid JSON escapes (\U, \j, \D)
// that silently drop every record containing the path.
package rewrite

import (
	"bytes"

	"github.com/arghhhhh/claude-conversation-transfer/internal/encode"
)

type Result struct {
	FilesModified  int
	TailConversions int
	Direction      string // "\\->/" or "/->\\" or "" if no tail step
}

// rewriteForms holds the two byte forms of a CWD that may appear in JSONL.
type rewriteForms struct {
	raw  []byte // separators as the source OS uses them, no JSON escaping
	jsonEsc []byte // backslashes doubled (only meaningful on Windows source)
}

func sourceForms(srcCWD string, srcOS encode.OS) rewriteForms {
	if srcOS == encode.Windows {
		return rewriteForms{
			raw:     []byte(srcCWD),
			jsonEsc: []byte(jsonEscapeBackslashes(srcCWD)),
		}
	}
	return rewriteForms{raw: []byte(srcCWD)}
}

// targetForm returns how the target CWD should appear inside a JSON string.
// For Windows targets that means doubled backslashes; for POSIX it's the
// path itself.
func targetForm(tgtCWD string, tgtOS encode.OS) []byte {
	if tgtOS == encode.Windows {
		return []byte(jsonEscapeBackslashes(tgtCWD))
	}
	return []byte(tgtCWD)
}

func jsonEscapeBackslashes(s string) string {
	// Each '\' becomes '\\' inside a JSON string literal.
	return string(bytes.ReplaceAll([]byte(s), []byte(`\`), []byte(`\\`)))
}

// Apply rewrites the bytes of one .jsonl file. It returns the new content,
// whether anything changed, and the number of tail-separator conversions.
//
// Steps:
//  1. Replace the source CWD prefix (both raw and JSON-escaped forms when
//     the source is Windows) with the target form.
//  2. Walk each occurrence of the target prefix and translate any leftover
//     source-style separators in the path tail into target-style. Stop at
//     the first non-path-token byte (quote, whitespace, JSON delimiter).
func Apply(data []byte, srcCWD string, srcOS encode.OS, tgtCWD string, tgtOS encode.OS) ([]byte, bool, int) {
	src := sourceForms(srcCWD, srcOS)
	tgt := targetForm(tgtCWD, tgtOS)

	orig := data
	out := data
	// JSON-escaped form first — longer, must precede the raw form so the
	// raw replacement doesn't eat half of it on Windows-source files.
	if len(src.jsonEsc) > 0 && !bytes.Equal(src.jsonEsc, src.raw) {
		out = bytes.ReplaceAll(out, src.jsonEsc, tgt)
	}
	out = bytes.ReplaceAll(out, src.raw, tgt)
	changed := !bytes.Equal(out, orig)

	if srcOS == tgtOS {
		// Same OS: nothing to translate in the tail.
		return out, changed, 0
	}

	var fromSep, toSep []byte
	if srcOS == encode.Windows && tgtOS == encode.POSIX {
		// Tail still has JSON-escaped backslashes (\\) — and maybe raw \.
		fromSep, toSep = []byte(`\\`), []byte(`/`)
	} else {
		// POSIX -> Windows: forward slashes in tail -> JSON-escaped backslash.
		fromSep, toSep = []byte(`/`), []byte(`\\`)
	}
	out2, n := convertTails(out, tgt, fromSep, toSep, false)
	if n > 0 {
		changed = true
	}
	// For Windows -> POSIX we also handle raw single backslashes in the tail
	// (defensive; the file shouldn't have them inside JSON strings, but we
	// don't want one to break a path). This pass is escape-aware: a single
	// backslash that introduces a JSON escape sequence (\", \\, \n, \uXXXX,
	// ...) is NOT a path separator — it's the syntax of the JSON string
	// itself — so we must leave it intact. Rewriting it corrupts the record
	// (e.g. a quoted path token like ".../dir\" becomes ".../dir/" and the
	// now-unescaped quote terminates the string early).
	if srcOS == encode.Windows && tgtOS == encode.POSIX {
		out3, n2 := convertTails(out2, tgt, []byte(`\`), []byte(`/`), true)
		return out3, changed || n2 > 0, n + n2
	}
	return out2, changed, n
}

// convertTails walks every occurrence of `prefix`, then translates fromSep
// to toSep until the path token ends.
//
// When skipJSONEscapes is true, a backslash that introduces a JSON escape
// sequence (\", \\, \/, \b, \f, \n, \r, \t, \uXXXX) is copied through verbatim
// along with the character it escapes, and is never treated as a separator.
// This is required for the single-backslash Windows->POSIX pass: in valid
// JSONL a real path separator is always doubled (\\) and handled earlier, so
// any lone backslash that remains belongs to the JSON string syntax, not the
// path.
func convertTails(data, prefix, fromSep, toSep []byte, skipJSONEscapes bool) ([]byte, int) {
	if len(prefix) == 0 {
		return data, 0
	}
	var out bytes.Buffer
	out.Grow(len(data))
	i := 0
	conversions := 0
	for i < len(data) {
		idx := bytes.Index(data[i:], prefix)
		if idx < 0 {
			out.Write(data[i:])
			break
		}
		out.Write(data[i : i+idx+len(prefix)])
		i += idx + len(prefix)
		for i < len(data) {
			if isPathTokenEnd(data[i]) {
				break
			}
			if skipJSONEscapes && data[i] == '\\' && i+1 < len(data) && isJSONEscapeChar(data[i+1]) {
				// Preserve the JSON escape sequence intact (both bytes).
				out.WriteByte(data[i])
				out.WriteByte(data[i+1])
				i += 2
				continue
			}
			if i+len(fromSep) <= len(data) && bytes.Equal(data[i:i+len(fromSep)], fromSep) {
				out.Write(toSep)
				i += len(fromSep)
				conversions++
				continue
			}
			out.WriteByte(data[i])
			i++
		}
	}
	return out.Bytes(), conversions
}

// isJSONEscapeChar reports whether b is a character that, when preceded by a
// backslash inside a JSON string, forms a valid escape sequence. A backslash
// followed by one of these is JSON syntax, not a path separator.
func isJSONEscapeChar(b byte) bool {
	switch b {
	case '"', '\\', '/', 'b', 'f', 'n', 'r', 't', 'u':
		return true
	}
	return false
}

// isPathTokenEnd returns true for bytes that cannot appear mid-path inside a
// .jsonl record. We err on the side of stopping early — broad rewrites
// across message content would corrupt code blocks and URLs.
func isPathTokenEnd(b byte) bool {
	switch b {
	case '"', '\t', '\n', '\r', ' ', ',', ';', ']', '}', '(', ')',
		'<', '>', '|', '*', '?', '\'', '`':
		return true
	}
	return false
}

// DirectionLabel returns the human-readable description of a tail conversion.
func DirectionLabel(srcOS, tgtOS encode.OS) string {
	if srcOS == tgtOS {
		return ""
	}
	if srcOS == encode.Windows && tgtOS == encode.POSIX {
		return `\->/`
	}
	return `/->\`
}
