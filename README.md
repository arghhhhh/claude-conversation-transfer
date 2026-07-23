# claude-conversation-transfer

Bundle a Claude Code project's conversation history into a portable zip and
import it onto another machine — correctly retargeting absolute paths across
OSes.

Replaces the `/export-proper` and `/import-proper` slash commands, which kept
accumulating subtle bugs because every run re-derived the rewrite logic from
prose. This is a compiled Go binary with regression tests for the cases that
historically broke.

## Install

```
go install github.com/arghhhhh/claude-conversation-transfer@latest
```

## Usage

```
claude-conversation-transfer export [--cwd PATH] [--out DIR]
claude-conversation-transfer import <zip> [--target-cwd PATH] [--json]
claude-conversation-transfer rename --to PATH [--from PATH] [--rename-dir] [--json]
```

`export` zips `~/.claude/projects/<encoded-cwd>/` into the current directory.
`import` extracts that zip into `~/.claude/projects/<encoded-current-cwd>/`,
rewrites embedded path prefixes from the source CWD to the current one, and
verifies every `.jsonl` line still parses as JSON.

`rename` collapses the four-step "export → rename the folder → new session →
import → clean up" dance into one command for renaming a project **on the same
machine**. It exports the project at `--from` (default: current CWD), imports
it into the encoded folder for `--to`, rewrites embedded paths, verifies, and —
only if verification passes — deletes the old `~/.claude/projects/` folder. Add
`--rename-dir` to also move the working directory on disk from `--from` to
`--to` (off by default; the safe assumption is that you already renamed it).

### Examples

Export the active project:

```
cd ~/work/my-project
claude-conversation-transfer export
# -> claude-convo-export-<encoded>-<YYYYMMDD-HHMMSS>.zip
```

Import onto a different machine where the project lives at a different path:

```
cd C:\Users\me\projects\my-project
claude-conversation-transfer import claude-convo-export-...zip
```

Machine-readable report:

```
claude-conversation-transfer import foo.zip --json
```

Rename a project in place (you already renamed the folder yourself):

```
cd C:\Users\me\projects\unfold-museum-pod-design
claude-conversation-transfer rename --from C:\Users\me\projects\pod-design --to C:\Users\me\projects\unfold-museum-pod-design
```

Or have the tool move the directory too:

```
claude-conversation-transfer rename \
  --from ~/work/pod-design --to ~/work/unfold-museum-pod-design --rename-dir
```

A `rename` `--json` report looks like:
`{"old_project":...,"new_project":...,"jsonl_files":N,"has_memory":bool,`
`"dir_renamed":bool,"preexisting_target_backup":path|"","old_data_deleted":bool,...}`.
If a project folder already existed at the new path, import backs it up to a
`.bak-<timestamp>` folder; `rename` surfaces it as `preexisting_target_backup`
and **never** deletes it — it may hold real prior sessions or memory to merge.

## What it rewrites

- Every occurrence of the source CWD inside `.jsonl` files, in both raw and
  JSON-escaped (`\\`) forms.
- Path tails under the rewritten prefix — separators in nested file paths
  are translated to the target OS's separator, scoped strictly to tokens that
  start with the rewritten prefix.

## What it does NOT rewrite

- `memory/MEMORY.md` and `memory/*.md` (already portable).
- Path references outside the project CWD (source user's home, system paths,
  other projects). Those would not resolve on the target machine either way,
  and broad rewrites would corrupt message text, code blocks, and URLs.

## Environment

`CLAUDE_CONFIG_DIR` is honored: when set, the projects directory is
`$CLAUDE_CONFIG_DIR/projects/` instead of the default `~/.claude/projects/`.

## Verification is part of the contract

After import, every `.jsonl` line is re-parsed as JSON. If any line fails,
the binary exits non-zero and points at the offending file. Claude Code's
session list populating is **not** evidence the import worked — sidecar
records survive most corruption and the conversation will still open empty.

## Exit codes

| Code | Meaning |
| ---- | ------- |
| 0    | success |
| 1    | verification failure (post-import/rename `.jsonl` files contain invalid JSON) |
| 2    | usage error |
| 3    | I/O / extract / read failure |
| 4    | `rename --rename-dir`: the on-disk directory move failed (Claude-side data left untouched) |

## Tests

```
go test ./...
```

Round-trip fixtures cover POSIX→Windows, Windows→POSIX, same-OS no-op,
underscore-prefix filenames, `memory/` preservation, and `subagents/`
substructure. `rename` has its own round-trip fixtures (same-machine Windows
rename, cross-OS rewrite, preexisting-target backup, `--rename-dir` on a real
directory, and the refusal/missing-source guards). The regression guard for the
historical backslash-transport bug — where `\\` became `\` and produced invalid
JSON escapes — runs on every test invocation.
