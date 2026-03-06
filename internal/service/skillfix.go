package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/firety/firety/internal/domain/lint"
)

type SkillFixer struct{}

type SkillFixResult struct {
	Applied []AppliedSkillFix
}

type AppliedSkillFix struct {
	Rule    lint.Rule
	Path    string
	Message string
}

func NewSkillFixer() SkillFixer {
	return SkillFixer{}
}

func (SkillFixer) Apply(target string) (SkillFixResult, error) {
	absoluteTarget, err := filepath.Abs(target)
	if err != nil {
		return SkillFixResult{}, err
	}

	info, err := os.Stat(absoluteTarget)
	if err != nil {
		if os.IsNotExist(err) {
			return SkillFixResult{}, nil
		}
		return SkillFixResult{}, err
	}
	if !info.IsDir() {
		return SkillFixResult{}, nil
	}

	skillPath := filepath.Join(absoluteTarget, skillFileName)
	content, err := os.ReadFile(skillPath)
	if err != nil {
		if os.IsNotExist(err) {
			return SkillFixResult{}, nil
		}
		return SkillFixResult{}, err
	}

	updated, applied := applySkillMarkdownFixes(absoluteTarget, string(content))
	if !applied {
		return SkillFixResult{}, nil
	}

	if err := writeFileAtomically(skillPath, updated, infoMode(skillPath)); err != nil {
		return SkillFixResult{}, err
	}

	return SkillFixResult{
		Applied: []AppliedSkillFix{
			{
				Rule:    lint.RuleMissingTitle,
				Path:    skillFileName,
				Message: "inserted a top-level markdown title into SKILL.md",
			},
		},
	}, nil
}

func applySkillMarkdownFixes(targetDir, content string) (string, bool) {
	doc := parseSkillDocument(targetDir, content)
	if doc.hasTitle {
		return content, false
	}

	newline := detectNewline(content)
	title := deriveSkillTitle(targetDir, doc.frontMatter)
	if title == "" {
		title = "Skill"
	}

	fixed := insertTopLevelTitle(content, title, newline)
	if fixed == content {
		return content, false
	}

	return fixed, true
}

func deriveSkillTitle(targetDir string, frontMatter frontMatter) string {
	if strings.TrimSpace(frontMatter.Name) != "" {
		return strings.TrimSpace(frontMatter.Name)
	}

	base := filepath.Base(targetDir)
	base = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(base, "-", " "), "_", " "))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "Skill"
	}

	return titleCaseWords(base)
}

func titleCaseWords(text string) string {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "Skill"
	}

	for index, part := range parts {
		lower := strings.ToLower(part)
		if lower == "" {
			continue
		}
		parts[index] = strings.ToUpper(lower[:1]) + lower[1:]
	}

	return strings.Join(parts, " ")
}

func detectNewline(content string) string {
	if strings.Contains(content, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

func insertTopLevelTitle(content, title, newline string) string {
	lines := strings.Split(content, newline)
	if len(lines) == 0 {
		lines = []string{""}
	}

	if len(lines) > 0 && strings.TrimSpace(lines[0]) == frontMatterOpeningDelimiter {
		closingIndex := -1
		for index := 1; index < len(lines); index++ {
			trimmed := strings.TrimSpace(lines[index])
			if trimmed == frontMatterOpeningDelimiter || trimmed == frontMatterClosingDelimiterAlt {
				closingIndex = index
				break
			}
		}

		if closingIndex >= 0 {
			insertAt := closingIndex + 1
			prefix := append([]string{}, lines[:insertAt]...)
			suffix := append([]string{}, lines[insertAt:]...)
			prefix = append(prefix, "# "+title, "")
			return strings.Join(append(prefix, suffix...), newline)
		}
	}

	lines = append([]string{"# " + title, ""}, lines...)
	return strings.Join(lines, newline)
}

func infoMode(path string) os.FileMode {
	info, err := os.Stat(path)
	if err != nil {
		return 0o644
	}
	return info.Mode().Perm()
}

func writeFileAtomically(path, content string, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tempFile, err := os.CreateTemp(dir, ".firety-fix-*")
	if err != nil {
		return err
	}

	tempName := tempFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempName)
		}
	}()

	if _, err := tempFile.WriteString(content); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Chmod(mode); err != nil {
		_ = tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}

	cleanup = false
	return nil
}
