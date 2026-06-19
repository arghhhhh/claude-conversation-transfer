package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/arghhhhh/claude-conversation-transfer/internal/cli"
)

const usage = `claude-conversation-transfer — bundle a Claude Code project for transport across machines.

Usage:
  claude-conversation-transfer export [--cwd PATH] [--out DIR]
  claude-conversation-transfer import <zip> [--target-cwd PATH] [--json]

Exit codes:
  0  success
  1  verification failure (post-import .jsonl files contain invalid JSON)
  2  usage error
  3  I/O / extract / read failure
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "export":
		os.Exit(runExport(os.Args[2:]))
	case "import":
		os.Exit(runImport(os.Args[2:]))
	case "-h", "--help", "help":
		fmt.Print(usage)
		os.Exit(0)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}

func runExport(args []string) int {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	cwdFlag := fs.String("cwd", "", "Project CWD to export (default: current shell CWD)")
	outFlag := fs.String("out", "", "Where to drop the zip (default: current shell CWD)")
	asJSON := fs.Bool("json", false, "Emit machine-readable JSON report")
	fs.Parse(args)

	cwd, err := resolvePath(*cwdFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 3
	}
	out, err := resolvePath(*outFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 3
	}
	rep, err := cli.RunExport(cli.ExportOpts{CWD: cwd, OutDir: out})
	if err != nil {
		fmt.Fprintln(os.Stderr, "export failed:", err)
		return 3
	}
	if *asJSON {
		json.NewEncoder(os.Stdout).Encode(rep)
		return 0
	}
	fmt.Printf("Archive:     %s\n", rep.Archive)
	fmt.Printf("Size:        %d bytes\n", rep.SizeBytes)
	fmt.Printf("JSONL files: %d\n", rep.JSONLFiles)
	fmt.Printf("memory/:     %v\n", rep.HasMemory)
	fmt.Printf("Source:      %s\n", rep.Source)
	return 0
}

func runImport(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: claude-conversation-transfer import <zip> [--target-cwd PATH] [--json]")
		return 2
	}
	zip := args[0]
	rest := args[1:]
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	tgtFlag := fs.String("target-cwd", "", "Project CWD on this machine (default: current shell CWD)")
	asJSON := fs.Bool("json", false, "Emit machine-readable JSON report")
	fs.Parse(rest)
	tgt, err := resolvePath(*tgtFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 3
	}
	rep, err := cli.RunImport(cli.ImportOpts{Zip: zip, TargetCWD: tgt})
	if err != nil {
		fmt.Fprintln(os.Stderr, "import failed:", err)
		return 3
	}
	if *asJSON {
		json.NewEncoder(os.Stdout).Encode(rep)
	} else {
		printImportHuman(rep)
	}
	if rep.Verification.TotalBad > 0 {
		return 1
	}
	return 0
}

func resolvePath(p string) (string, error) {
	if p != "" {
		return p, nil
	}
	return os.Getwd()
}

func printImportHuman(rep cli.ImportReport) {
	fmt.Printf("Target:        %s\n", rep.Target)
	fmt.Printf("JSONL files:   %d\n", rep.JSONLFiles)
	fmt.Printf("memory/:       %v\n", rep.HasMemory)
	if rep.Backup != "" {
		fmt.Printf("Backup:        %s\n", rep.Backup)
		fmt.Println("  (Manually merge anything you want to keep from the backup: memory/MEMORY.md, per-memory .md files, any .jsonl sessions not in the imported set.)")
	}
	fmt.Printf("Source CWD:    %s (%s)\n", rep.SourceCWD, rep.SourceOS)
	fmt.Printf("Target CWD:    %s (%s)\n", rep.TargetCWD, rep.TargetOS)
	if rep.FilesRewritten > 0 || rep.TailConversions > 0 {
		fmt.Printf("Rewrote:       %d .jsonl files\n", rep.FilesRewritten)
		if rep.TailDirection != "" {
			fmt.Printf("Tail seps:     %d conversions (%s)\n", rep.TailConversions, rep.TailDirection)
		}
	} else {
		fmt.Println("Rewrite:       not needed (same source/target CWD)")
	}
	fmt.Printf("Verification:  %d OK lines, %d bad lines\n", rep.Verification.TotalOK, rep.Verification.TotalBad)
	if rep.Verification.TotalBad > 0 {
		fmt.Println("  FAILED: one or more .jsonl files contain invalid JSON after import.")
		for _, f := range rep.Verification.Files {
			if f.BadLines > 0 {
				fmt.Printf("    %s: bad=%d first=%s\n", f.Path, f.BadLines, f.FirstError)
			}
		}
	}
	fmt.Println(rep.OutOfCWDIgnored)
}
