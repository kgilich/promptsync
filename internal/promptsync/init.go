package promptsync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type starterFile struct {
	path    string
	content string
}

// Init scaffolds a library for the default set of tools.
func Init(root string) error {
	return InitWithTools(root, DefaultTools)
}

// InitWithTools scaffolds promptsync.yaml and a starter prompt that target the
// given presets (see Presets).
func InitWithTools(root string, tools []string) error {
	if len(tools) == 0 {
		tools = DefaultTools
	}
	config, err := buildConfig("prompts/master.md", map[string]string{"Project": "My Project"}, tools)
	if err != nil {
		return err
	}
	return writeNewFiles(root, []starterFile{
		{path: "promptsync.yaml", content: config},
		{path: "prompts/master.md", content: starterPrompt},
	})
}

func buildConfig(source string, variables map[string]string, tools []string) (string, error) {
	var out strings.Builder
	fmt.Fprintf(&out, "version: 1\nsource: %s\n", source)
	if len(variables) > 0 {
		out.WriteString("variables:\n")
		for key, value := range variables {
			fmt.Fprintf(&out, "  %s: %s\n", key, value)
		}
	}
	out.WriteString("targets:\n")
	seen := map[string]bool{}
	for _, tool := range tools {
		preset, err := lookupPreset(tool)
		if err != nil {
			return "", err
		}
		if seen[preset.Name] {
			continue
		}
		seen[preset.Name] = true
		fmt.Fprintf(&out, "  - preset: %s\n", preset.Name)
	}
	return out.String(), nil
}

// writeNewFiles writes all files or none: it refuses if any already exists.
func writeNewFiles(root string, files []starterFile) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve project directory: %w", err)
	}
	for _, file := range files {
		path := filepath.Join(absRoot, file.path)
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("refusing to overwrite existing file %q", path)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("check file %q: %w", path, err)
		}
	}
	for _, file := range files {
		path := filepath.Join(absRoot, file.path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("create directory for %q: %w", path, err)
		}
		if err := os.WriteFile(path, []byte(file.content), 0o644); err != nil {
			return fmt.Errorf("write starter file %q: %w", path, err)
		}
	}
	return nil
}

const starterPrompt = `# Project instructions for {{ .Project }}

Describe shared engineering conventions, architecture, and coding guidance here.
`
