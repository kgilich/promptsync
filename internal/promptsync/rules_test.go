package promptsync

import (
	"path/filepath"
	"strings"
	"testing"
)

const scopedConfig = `version: 1
source: master.md
variables:
  Lang: TypeScript
rules:
  - source: rules/frontend.md
    globs: ["src/**/*.tsx", "*.css"]
    description: React conventions
targets:
  - preset: cursor
  - preset: copilot
  - preset: claude
  - preset: windsurf
  - preset: agents
`

func TestScopedRulesUseNativeFormatsOrFoldIntoMainFile(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "promptsync.yaml", scopedConfig)
	writeTestFile(t, root, "master.md", "# Main\n")
	writeTestFile(t, root, "rules/frontend.md", "Use {{ .Lang }} hooks.\n")

	if _, err := Sync(filepath.Join(root, "promptsync.yaml")); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	for path, wants := range map[string][]string{
		".cursor/rules/frontend.mdc":                    {`globs: "src/**/*.tsx,*.css"`, "alwaysApply: false", "Use TypeScript hooks."},
		".github/instructions/frontend.instructions.md": {`applyTo: "src/**/*.tsx,*.css"`},
		".claude/rules/frontend.md":                     {"paths:\n  - \"src/**/*.tsx\"\n  - \"*.css\""},
		".windsurf/rules/frontend.md":                   {"trigger: glob"},
		"AGENTS.md":                                     {"## Applies to: src/**/*.tsx, *.css", "Use TypeScript hooks."},
	} {
		got := readTestFile(t, root, path)
		for _, want := range wants {
			if !strings.Contains(got, want) {
				t.Errorf("%s missing %q:\n%s", path, want, got)
			}
		}
	}
	// Native tools keep the scoped rule out of the main file.
	if got := readTestFile(t, root, "CLAUDE.md"); strings.Contains(got, "TypeScript hooks") {
		t.Errorf("CLAUDE.md should not contain the scoped rule:\n%s", got)
	}
	statuses, err := Inspect(filepath.Join(root, "promptsync.yaml"))
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	for _, status := range statuses {
		if status.State != TargetCurrent {
			t.Errorf("%s = %s after sync", status.Path, status.State)
		}
	}
}

func TestRulesRejectInvalidDefinitions(t *testing.T) {
	for name, rules := range map[string]string{
		"no globs":  "rules:\n  - source: r.md\n",
		"bad name":  "rules:\n  - source: R ules.md\n    globs: [\"*\"]\n",
		"duplicate": "rules:\n  - source: a/r.md\n    globs: [\"*\"]\n  - source: b/r.md\n    globs: [\"*\"]\n",
	} {
		root := t.TempDir()
		writeTestFile(t, root, "promptsync.yaml", "version: 1\nsource: master.md\n"+rules+"targets:\n  - preset: claude\n")
		if _, err := Inspect(filepath.Join(root, "promptsync.yaml")); err == nil {
			t.Errorf("%s: config unexpectedly accepted", name)
		}
	}
}
