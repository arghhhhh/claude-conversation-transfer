// Package archive zips and unzips a project folder. We bundle the contents
// of the folder, not the wrapping folder itself, matching the prose spec.
package archive

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Create writes a zip of srcDir's contents to outPath.
func Create(srcDir, outPath string) (int, error) {
	info, err := os.Stat(srcDir)
	if err != nil {
		return 0, err
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("not a directory: %s", srcDir)
	}
	f, err := os.Create(outPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()
	count := 0
	err = filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == srcDir {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			_, err = zw.Create(rel + "/")
			return err
		}
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		r, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, r)
		r.Close()
		if err != nil {
			return err
		}
		count++
		return nil
	})
	return count, err
}

// Extract unzips zipPath into destDir. destDir must already exist and be
// empty (the caller handles backup).
func Extract(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if err := extractOne(f, destDir); err != nil {
			return err
		}
	}
	return nil
}

func extractOne(f *zip.File, destDir string) error {
	clean := filepath.Clean(f.Name)
	if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return errors.New("unsafe path in archive: " + f.Name)
	}
	target := filepath.Join(destDir, clean)
	if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) && target != filepath.Clean(destDir) {
		return errors.New("zip slip: " + f.Name)
	}
	if f.FileInfo().IsDir() {
		return os.MkdirAll(target, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, rc)
	cerr := out.Close()
	if err != nil {
		return err
	}
	return cerr
}

// PeekCWD reads the first .jsonl entry inside zipPath and extracts its
// "cwd" field. Returns "" if no .jsonl has a cwd.
func PeekCWD(zipPath string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer r.Close()
	for _, f := range r.File {
		if !strings.HasSuffix(strings.ToLower(f.Name), ".jsonl") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		const max = 1 << 20 // 1MB header is plenty to find cwd
		buf := make([]byte, max)
		n, _ := io.ReadFull(rc, buf)
		rc.Close()
		cwd := findCWD(buf[:n])
		if cwd != "" {
			return cwd, nil
		}
	}
	return "", nil
}

// findCWD scans for `"cwd":"..."` and returns the unescaped value.
func findCWD(buf []byte) string {
	needle := []byte(`"cwd":"`)
	i := 0
	for {
		idx := indexOf(buf, needle, i)
		if idx < 0 {
			return ""
		}
		start := idx + len(needle)
		// Find unescaped closing quote.
		j := start
		for j < len(buf) {
			if buf[j] == '\\' && j+1 < len(buf) {
				j += 2
				continue
			}
			if buf[j] == '"' {
				break
			}
			j++
		}
		if j >= len(buf) {
			return ""
		}
		// Unescape: \\ -> \, \" -> ", \/ -> /
		raw := buf[start:j]
		out := make([]byte, 0, len(raw))
		for k := 0; k < len(raw); k++ {
			if raw[k] == '\\' && k+1 < len(raw) {
				out = append(out, raw[k+1])
				k++
				continue
			}
			out = append(out, raw[k])
		}
		return string(out)
	}
}

func indexOf(haystack, needle []byte, from int) int {
	if from >= len(haystack) {
		return -1
	}
	idx := -1
	for k := from; k+len(needle) <= len(haystack); k++ {
		ok := true
		for m := 0; m < len(needle); m++ {
			if haystack[k+m] != needle[m] {
				ok = false
				break
			}
		}
		if ok {
			idx = k
			break
		}
	}
	return idx
}
