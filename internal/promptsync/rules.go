package promptsync

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// Rule is extra guidance that applies only to files matching Globs, e.g.
// frontend conventions for "src/**/*.tsx". Tools with native scoped rules get
// one file per rule; for the rest the rule is folded into the main file under
// a heading that names its scope.
type Rule struct {
	Name        string   `yaml:"name,omitempty"`
	Source      string   `yaml:"source"`
	Globs       []string `yaml:"globs"`
	Description string   `yaml:"description,omitempty"`
}

var ruleNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// scopedFormat describes how a tool stores glob-scoped rules natively.
type scopedFormat struct {
	pathFormat  string // %s is the rule name
	frontmatter func(rule Rule) string
}

func quoteGlobs(globs []string) string { return strconv.Quote(strings.Join(globs, ",")) }

func yamlList(key string, values []string) string {
	var out strings.Builder
	out.WriteString(key + ":\n")
	for _, value := range values {
		out.WriteString("  - " + strconv.Quote(value) + "\n")
	}
	return out.String()
}

func descriptionLine(rule Rule) string {
	if rule.Description == "" {
		return ""
	}
	return "description: " + strconv.Quote(rule.Description) + "\n"
}

var scopedFormats = map[string]scopedFormat{
	"cursor": {".cursor/rules/%s.mdc", func(r Rule) string {
		return "---\n" + descriptionLine(r) + "globs: " + quoteGlobs(r.Globs) + "\nalwaysApply: false\n---\n"
	}},
	"copilot": {".github/instructions/%s.instructions.md", func(r Rule) string {
		return "---\n" + descriptionLine(r) + "applyTo: " + quoteGlobs(r.Globs) + "\n---\n"
	}},
	"claude": {".claude/rules/%s.md", func(r Rule) string {
		return "---\n" + yamlList("paths", r.Globs) + "---\n"
	}},
	"windsurf": {".windsurf/rules/%s.md", func(r Rule) string {
		return "---\ntrigger: glob\n" + descriptionLine(r) + "globs: " + quoteGlobs(r.Globs) + "\n---\n"
	}},
	"continue": {".continue/rules/%s.md", func(r Rule) string {
		return "---\nname: " + strconv.Quote(r.Name) + "\n" + yamlList("globs", r.Globs) + "---\n"
	}},
}

type renderedRule struct {
	Rule
	content string
}

// normalizeRules fills in rule names and rejects invalid or duplicate rules.
func (config *Config) normalizeRules() error {
	seen := map[string]bool{}
	for index, rule := range config.Rules {
		label := fmt.Sprintf("rule %d", index+1)
		if strings.TrimSpace(rule.Source) == "" {
			return fmt.Errorf("%s source must not be empty", label)
		}
		if len(rule.Globs) == 0 {
			return fmt.Errorf("%s (%s) needs at least one glob; put unscoped guidance in the main source", label, rule.Source)
		}
		for _, glob := range rule.Globs {
			if strings.TrimSpace(glob) == "" || strings.ContainsAny(glob, "\r\n") {
				return fmt.Errorf("%s (%s) has an invalid glob %q", label, rule.Source, glob)
			}
		}
		if rule.Name == "" {
			base := path.Base(strings.ReplaceAll(rule.Source, "\\", "/"))
			rule.Name = strings.ToLower(strings.TrimSuffix(base, path.Ext(base)))
		}
		if !ruleNamePattern.MatchString(rule.Name) {
			return fmt.Errorf("%s name %q must be lowercase letters, digits, '.', '_' or '-'; set 'name' explicitly", label, rule.Name)
		}
		if seen[rule.Name] {
			return fmt.Errorf("duplicate rule name %q", rule.Name)
		}
		seen[rule.Name] = true
		config.Rules[index] = rule
	}
	return nil
}

func loadRules(config Config, library string) ([]renderedRule, error) {
	rules := make([]renderedRule, 0, len(config.Rules))
	for _, rule := range config.Rules {
		if err := validatePathWithin(library, rule.Source, false, "rule source"); err != nil {
			return nil, err
		}
		sourcePath := resolve(library, rule.Source)
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("read rule source %q: %w", sourcePath, err)
		}
		content, err := render(rule.Source, string(data), config.Variables)
		if err != nil {
			return nil, err
		}
		rules = append(rules, renderedRule{Rule: rule, content: content})
	}
	return rules, nil
}

// foldRules appends scoped rules to a main file for tools without native support.
func foldRules(content string, rules []renderedRule) string {
	var out strings.Builder
	out.WriteString(strings.TrimRight(content, "\n"))
	out.WriteString("\n")
	for _, rule := range rules {
		fmt.Fprintf(&out, "\n## Applies to: %s\n\n%s", strings.Join(rule.Globs, ", "), strings.TrimLeft(strings.TrimRight(rule.content, "\n"), "\n"))
		out.WriteString("\n")
	}
	return out.String()
}

// ruleTargets returns the per-rule files for a preset with native scoped rules.
func ruleTargets(preset Preset, rules []renderedRule) []generatedTarget {
	format, ok := scopedFormats[preset.Name]
	if !ok {
		return nil
	}
	targets := make([]generatedTarget, 0, len(rules))
	for _, rule := range rules {
		body := fmt.Sprintf(generatedBannerFormat, rule.Source) + "\n\n" + strings.TrimLeft(rule.content, "\n")
		targets = append(targets, generatedTarget{
			path:    fmt.Sprintf(format.pathFormat, rule.Name),
			content: []byte(format.frontmatter(rule.Rule) + body),
		})
	}
	return targets
}
