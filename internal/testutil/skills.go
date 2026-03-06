package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func WriteFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()

	for path, content := range files {
		fullPath := filepath.Join(root, path)

		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("create dir for %s: %v", fullPath, err)
		}

		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("write file %s: %v", fullPath, err)
		}
	}
}

func ValidSkillFiles() map[string]string {
	return map[string]string{
		"SKILL.md": `---
name: Example Skill
description: Validates local skill directories and demonstrates clear invocation guidance with practical examples.
---

# Example Skill

## When To Use

Use this skill when you need to validate a reusable skill directory before sharing it or adding it to a larger skill collection.

## Usage

Run ` + "`firety skill lint .`" + ` to validate the skill directory.
Use this skill when you want a compact example of a documented capability with clear invocation guidance and a working local reference.

## Limitations

Do not use this skill to evaluate remote repositories, external tool integrations, or runtime behavior.
Use another tool when you need execution traces, network-aware validation, or provider-specific checks.

## Examples

- Request: "Lint this local skill directory before I publish it."
- Invocation: Run ` + "`firety skill lint . --format json`" + ` from the skill root.
- Result: Review the reported findings and fix any missing links or weak guidance before publishing.
- Reference: see [Reference](docs/reference.md)
`,
		"docs/reference.md": "# Reference\n\nThis reference explains the expected lint workflow and the supporting files that belong in the bundle.\n",
	}
}
