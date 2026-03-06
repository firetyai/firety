package lint_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firety/firety/internal/domain/lint"
)

func TestAllRulesHaveRequiredMetadata(t *testing.T) {
	t.Parallel()

	seenIDs := make(map[string]struct{}, len(lint.AllRules()))
	seenSlugs := make(map[string]struct{}, len(lint.AllRules()))

	for _, rule := range lint.AllRules() {
		if rule.ID == "" {
			t.Fatalf("expected rule id, got %#v", rule)
		}
		if rule.Slug == "" {
			t.Fatalf("expected rule slug, got %#v", rule)
		}
		if rule.Category == "" {
			t.Fatalf("expected rule category, got %#v", rule)
		}
		if rule.Severity == "" {
			t.Fatalf("expected default severity, got %#v", rule)
		}
		if rule.Title == "" {
			t.Fatalf("expected title, got %#v", rule)
		}
		if rule.Description == "" {
			t.Fatalf("expected description, got %#v", rule)
		}
		if rule.Why == "" {
			t.Fatalf("expected why it matters text, got %#v", rule)
		}
		if rule.WhatGoodLooksLike == "" {
			t.Fatalf("expected what-good-looks-like text, got %#v", rule)
		}
		if rule.ImprovementHint == "" {
			t.Fatalf("expected improvement hint text, got %#v", rule)
		}
		if rule.Fixability == "" {
			t.Fatalf("expected fixability metadata, got %#v", rule)
		}
		for _, severity := range []lint.Severity{rule.StrictSeverity, rule.PedanticSeverity} {
			if severity == "" {
				continue
			}
			if severity != lint.SeverityWarning && severity != lint.SeverityError {
				t.Fatalf("expected valid strictness severity, got %#v", rule)
			}
		}

		if _, exists := seenIDs[rule.ID]; exists {
			t.Fatalf("duplicate rule id %q", rule.ID)
		}
		if _, exists := seenSlugs[rule.Slug]; exists {
			t.Fatalf("duplicate rule slug %q", rule.Slug)
		}

		seenIDs[rule.ID] = struct{}{}
		seenSlugs[rule.Slug] = struct{}{}
	}
}

func TestGroupedRulesPreserveCatalogOrder(t *testing.T) {
	t.Parallel()

	groups := lint.GroupedRules()
	if len(groups) != len(lint.AllCategories()) {
		t.Fatalf("expected %d groups, got %d", len(lint.AllCategories()), len(groups))
	}

	for index, group := range groups {
		if group.Category == "" || group.Title == "" {
			t.Fatalf("expected populated group metadata, got %#v", group)
		}

		if group.Category != lint.AllCategories()[index] {
			t.Fatalf("expected group %d to be %q, got %#v", index, lint.AllCategories()[index], group)
		}
	}

	allRules := lint.AllRules()
	lastIndexByCategory := make(map[lint.Category]int, len(groups))
	for _, rule := range allRules {
		categoryIndex := -1
		for index, group := range groups {
			if group.Category != rule.Category {
				continue
			}

			categoryIndex = index
			if lastIndexByCategory[group.Category] >= len(group.Rules) {
				t.Fatalf("category %q ran out of grouped rules", group.Category)
			}
			if group.Rules[lastIndexByCategory[group.Category]].ID != rule.ID {
				t.Fatalf("expected next %q rule to be %q, got %#v", group.Category, rule.ID, group.Rules[lastIndexByCategory[group.Category]])
			}
			lastIndexByCategory[group.Category]++
			break
		}

		if categoryIndex == -1 {
			t.Fatalf("expected to find category %q for rule %q", rule.Category, rule.ID)
		}
	}
}

func TestMarkdownCatalogMatchesCheckedInDocs(t *testing.T) {
	t.Parallel()

	root := projectRoot(t)
	content, err := os.ReadFile(filepath.Join(root, "docs", "lint-rules.md"))
	if err != nil {
		t.Fatalf("read docs file: %v", err)
	}

	if string(content) != lint.MarkdownCatalog() {
		t.Fatalf("docs/lint-rules.md is out of sync with the rule catalog; run `make rules-docs`")
	}
}

func TestRuleCatalogJSONIsStable(t *testing.T) {
	t.Parallel()

	payload := struct {
		Rules []lint.Rule `json:"rules"`
	}{
		Rules: lint.AllRules(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}

	var decoded struct {
		Rules []lint.Rule `json:"rules"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}

	if len(decoded.Rules) != len(lint.AllRules()) {
		t.Fatalf("expected %d rules, got %d", len(lint.AllRules()), len(decoded.Rules))
	}

	for index, rule := range lint.AllRules() {
		if decoded.Rules[index].ID != rule.ID {
			t.Fatalf("expected rule %d to be %q, got %#v", index, rule.ID, decoded.Rules[index])
		}
	}
}

func TestTextCatalogIsGroupedAndReadable(t *testing.T) {
	t.Parallel()

	output := lint.TextCatalog()
	for _, expected := range []string{
		"Firety lint rules",
		"Structure",
		"Metadata / Spec",
		"Examples",
		"Portability",
		"skill.target-not-found [error]",
		"Why:",
		"Good:",
		"Improve:",
		"Metadata:",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got %q", expected, output)
		}
	}
}

func projectRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	return filepath.Clean(filepath.Join(dir, "..", "..", ".."))
}
