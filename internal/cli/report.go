package cli

import "github.com/arghhhhh/claude-conversation-transfer/internal/verify"

type ExportReport struct {
	Archive    string `json:"archive"`
	SizeBytes  int64  `json:"size_bytes"`
	JSONLFiles int    `json:"jsonl_files"`
	HasMemory  bool   `json:"has_memory"`
	Source     string `json:"source"`
}

type ImportReport struct {
	Target           string             `json:"target"`
	JSONLFiles       int                `json:"jsonl_files"`
	HasMemory        bool               `json:"has_memory"`
	Backup           string             `json:"backup,omitempty"`
	SourceCWD        string             `json:"source_cwd"`
	SourceOS         string             `json:"source_os"`
	TargetCWD        string             `json:"target_cwd"`
	TargetOS         string             `json:"target_os"`
	FilesRewritten   int                `json:"files_rewritten"`
	TailConversions  int                `json:"tail_conversions"`
	TailDirection    string             `json:"tail_direction,omitempty"`
	OutOfCWDIgnored  string             `json:"out_of_cwd_ignored"`
	Verification     verify.Report      `json:"verification"`
}

// RenameReport describes an in-place project rename: migrate the Claude-side
// data from the old CWD's encoded folder to the new one, verify, and (on a
// clean verify) delete the old folder. Mirrors export/import report style.
type RenameReport struct {
	OldProject string `json:"old_project"` // ~/.claude/projects/<old-encoded>
	NewProject string `json:"new_project"` // ~/.claude/projects/<new-encoded>
	OldCWD     string `json:"old_cwd"`
	NewCWD     string `json:"new_cwd"`
	JSONLFiles int    `json:"jsonl_files"`
	HasMemory  bool   `json:"has_memory"`
	// DirRenamed is true when --rename-dir was requested and the on-disk
	// project directory was successfully moved from OldCWD to NewCWD.
	DirRenamed bool `json:"dir_renamed"`
	// PreexistingTargetBackup is the .bak-<ts> path import created when a
	// project folder already lived at the new encoded path. Empty if none.
	// This is NEVER auto-deleted — it may hold real prior sessions/memory.
	PreexistingTargetBackup string `json:"preexisting_target_backup"`
	// OldDataDeleted reports whether the old encoded folder was removed. Only
	// happens after verification passes (never on a failed verify).
	OldDataDeleted  bool          `json:"old_data_deleted"`
	FilesRewritten  int           `json:"files_rewritten"`
	TailConversions int           `json:"tail_conversions"`
	TailDirection   string        `json:"tail_direction,omitempty"`
	Verification    verify.Report `json:"verification"`
}
