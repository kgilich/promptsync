# PromptSync

**One source of truth for AI coding instructions.** Every AI coding tool wants its own file — `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, `.github/copilot-instructions.md`, Cursor `.mdc` rules, Windsurf, Cline, Junie… PromptSync lets you write your project guidance once in Markdown and generates all of them, keeps them from drifting, and verifies them in CI.

[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![CI](https://github.com/kgilich/promptsync/actions/workflows/ci.yml/badge.svg)](https://github.com/kgilich/promptsync/actions/workflows/ci.yml)

Keep the library local, or share it across projects through Git. A committed lockfile makes every project use the same library revision.

## Get started

```sh
go install ./cmd/promptsync
```

**New project** — pick the tools your team uses:

```sh
promptsync init -tools claude,cursor,copilot,agents
promptsync sync
```

**Project that already has instruction files?** Adopt them instead of rewriting:

```sh
promptsync import          # merges CLAUDE.md, AGENTS.md, .cursorrules, ... into prompts/master.md
promptsync sync --force    # replace the originals with generated files
```

Identical rules found in several files are imported once; differing ones are kept as separate sections for you to reconcile.

Edit `prompts/master.md`, run `promptsync sync`, and commit both the source and the generated files.

### Built-in presets

Targets can use a `preset` instead of a hand-written template. A preset knows the tool's file location and required front matter (e.g. Cursor's `alwaysApply`), and adds a "generated, do not edit" banner.

```yaml
version: 1
source: prompts/master.md
variables:
  Project: Example App
targets:
  - preset: claude
  - preset: cursor
  - preset: copilot
  - preset: agents
    path: docs/AGENTS.md   # optional: override the default location
```

Run `promptsync presets` to list them: `agents`, `amazonq`, `claude`, `cline`, `continue`, `copilot`, `cursor`, `gemini`, `junie`, `windsurf`. Custom `template:` targets still work for anything else (`preset` and `template` cannot be combined).

### Scoped rules

Some guidance only applies to part of a codebase. Declare it as a `rule` with `globs`:

```yaml
rules:
  - source: prompts/frontend.md
    globs: ["src/**/*.tsx", "*.css"]
    description: React conventions
```

Tools with native scoped rules get one file per rule, using their own syntax: Cursor (`.cursor/rules/frontend.mdc`), Copilot (`.github/instructions/frontend.instructions.md` with `applyTo`), Claude Code (`.claude/rules/frontend.md` with `paths`), Windsurf, and Continue. Tools without that feature (`agents`, `gemini`, `junie`, `cline`, `amazonq`) get the rule appended to their main file under an "Applies to: …" heading. Rule sources are templates too and share `variables`. The rule name defaults to the file name; set `name:` to override it. Rules apply to preset targets only.

### Watch mode

```sh
promptsync sync --watch
```

Re-syncs whenever the source, a rule, a template, or the config changes. A manually edited target is reported and left alone (unless `--force`), and the watch keeps running.

### CI check

Keep generated files honest in pull requests with the bundled GitHub Action:

```yaml
# .github/workflows/promptsync.yml
name: PromptSync
on: [pull_request]
permissions:
  contents: read
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - uses: kgilich/promptsync@main   # pin a tag or commit SHA in real use
```

It installs PromptSync, restores the locked library revision if you use a remote one, and runs `promptsync sync --check`.

### Shared Git library

Add a remote library to `promptsync.yaml`:

```yaml
version: 1
library:
  repository: https://github.com/your-org/prompt-library.git
  ref: main
source: prompts/master.md
variables:
  Project: Example App
targets:
  - preset: claude
  - preset: cursor
  - preset: copilot
```

Fetch the latest configured revision and render it:

```sh
promptsync update
promptsync sync
```

Commit `promptsync.lock` with your project. It records the exact library commit. On a fresh clone or CI runner, install that pinned revision without changing the lock:

```sh
promptsync install
promptsync sync --check
```

Run `promptsync update` only when you intentionally want to move to a newer revision, then review and commit the lockfile change.

## Commands

| Command | What it does |
| --- | --- |
| `promptsync init [-dir PATH] [-tools a,b]` | Create a starter prompt library for the chosen tool presets. |
| `promptsync import [-dir PATH]` | Adopt existing instruction files (CLAUDE.md, AGENTS.md, .cursorrules, ...) as the source. |
| `promptsync presets` | List built-in tool presets. |
| `promptsync update [-config PATH]` | Fetch the configured Git library and update `promptsync.lock`. |
| `promptsync install [-config PATH]` | Install the exact commit from `promptsync.lock` without changing it. |
| `promptsync status [-config PATH]` | Report each target as `missing`, `current`, or `modified`. |
| `promptsync sync [-config PATH]` | Render and write all configured targets. |
| `promptsync sync --check [-config PATH]` | Check target freshness without writing; fail if any target is missing or modified. |
| `promptsync sync --watch [-config PATH]` | Re-sync automatically when inputs change. |
| `promptsync sync --force [-config PATH]` | Overwrite targets that differ from generated content. |
| `promptsync help` | Show command usage. |

## How it works

- **One source:** Markdown is the canonical prompt; Go templates adapt it for each target.
- **Consistent output:** Generated text always uses LF line endings, regardless of the source checkout's CRLF/LF settings.
- **Explicit network steps:** `update` advances the lock; `install` materializes the locked commit. `sync`, `status`, and `sync --check` do not access the network.
- **Reproducible projects:** `sync` and `status` use the commit in `promptsync.lock`.
- **Local or shared:** Without `library`, paths are relative to the config file. With `library`, source and template paths are relative to the cloned library.
- **Contained inputs:** Source and template paths, including symlinks, cannot escape the library or project root.
- **Safe defaults:** Modified files are not overwritten unless `sync --force` is given. Output paths, including symlink-resolved paths, must stay inside the project unless `allow_external_targets: true` is set in the config.
- **Git required for remote libraries:** Remote URLs must use HTTPS and identify a branch or tag. The repository is cached in the operating system's user cache directory.

`status` and `sync --check` are read-only. `sync` creates missing targets and leaves current ones alone. If a target was manually changed, inspect it with `status` and use `sync --force` only when you intend to discard those edits.

For CI using a remote library, restore the exact lock first, then verify that generated files are committed and current:

```sh
promptsync install
promptsync sync --check
```

## Develop

Supports Linux, macOS, and Windows. Requires Go 1.22 or newer; Git is needed only when using a remote library.

```sh
go test ./...
go build ./cmd/promptsync
```
