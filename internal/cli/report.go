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
