package lint

import (
	"fmt"
	"strings"
)

type RuleGroup struct {
	Category Category `json:"category"`
	Title    string   `json:"title"`
	Rules    []Rule   `json:"rules"`
}

func AllCategories() []Category {
	return []Category{
		CategoryStructure,
		CategoryMetadataSpec,
		CategoryInvocation,
		CategoryExamples,
		CategoryNegativeGuidance,
		CategoryConsistency,
		CategoryPortability,
		CategoryBundleResources,
		CategoryEfficiencyCost,
		CategoryTriggerQuality,
	}
}

func (c Category) Title() string {
	switch c {
	case CategoryStructure:
		return "Structure"
	case CategoryMetadataSpec:
		return "Metadata / Spec"
	case CategoryInvocation:
		return "Invocation"
	case CategoryExamples:
		return "Examples"
	case CategoryNegativeGuidance:
		return "Negative Guidance"
	case CategoryConsistency:
		return "Consistency"
	case CategoryPortability:
		return "Portability"
	case CategoryBundleResources:
		return "Bundle / Resources"
	case CategoryEfficiencyCost:
		return "Efficiency / Cost"
	case CategoryTriggerQuality:
		return "Trigger Quality"
	default:
		return string(c)
	}
}

func GroupedRules() []RuleGroup {
	rulesByCategory := make(map[Category][]Rule, len(AllCategories()))
	for _, rule := range AllRules() {
		rulesByCategory[rule.Category] = append(rulesByCategory[rule.Category], rule)
	}

	groups := make([]RuleGroup, 0, len(AllCategories()))
	for _, category := range AllCategories() {
		rules := rulesByCategory[category]
		if len(rules) == 0 {
			continue
		}

		groups = append(groups, RuleGroup{
			Category: category,
			Title:    category.Title(),
			Rules:    rules,
		})
	}

	return groups
}

func MarkdownCatalog() string {
	var builder strings.Builder

	builder.WriteString("# Firety Lint Rules\n\n")
	builder.WriteString("This document is generated from Firety's Go rule catalog. Treat the rule IDs and metadata here as the authoritative product surface for automation, CI, SARIF, and future integrations. Profile-aware rules may also drive selected-profile guidance in `firety skill lint --explain`, but those hints stay heuristic and conservative.\n")

	for _, group := range GroupedRules() {
		builder.WriteString("\n## ")
		builder.WriteString(group.Title)
		builder.WriteString("\n")

		for _, rule := range group.Rules {
			builder.WriteString("\n### `")
			builder.WriteString(rule.ID)
			builder.WriteString("`\n")
			builder.WriteString("- Default severity: `")
			builder.WriteString(string(rule.Severity))
			builder.WriteString("`\n")
			if rule.StrictSeverity != "" {
				builder.WriteString("- Strict severity: `")
				builder.WriteString(string(rule.StrictSeverity))
				builder.WriteString("`\n")
			}
			if rule.PedanticSeverity != "" {
				builder.WriteString("- Pedantic severity: `")
				builder.WriteString(string(rule.PedanticSeverity))
				builder.WriteString("`\n")
			}
			builder.WriteString("- Title: ")
			builder.WriteString(rule.Title)
			builder.WriteString("\n")
			builder.WriteString("- Description: ")
			builder.WriteString(rule.Description)
			builder.WriteString("\n")
			builder.WriteString("- Why it matters: ")
			builder.WriteString(rule.Why)
			builder.WriteString("\n")
			builder.WriteString("- What good looks like: ")
			builder.WriteString(rule.WhatGoodLooksLike)
			builder.WriteString("\n")
			builder.WriteString("- Improvement hint: ")
			builder.WriteString(rule.ImprovementHint)
			builder.WriteString("\n")
			builder.WriteString("- Profile-aware: ")
			builder.WriteString(boolText(rule.ProfileAware))
			builder.WriteString("\n")
			builder.WriteString("- Can include line numbers: ")
			builder.WriteString(boolText(rule.LineAware))
			builder.WriteString("\n")
			builder.WriteString("- Autofix: `")
			builder.WriteString(string(rule.Fixability))
			builder.WriteString("`\n")
			if rule.FixHint != "" {
				builder.WriteString("- Fix hint: ")
				builder.WriteString(rule.FixHint)
				builder.WriteString("\n")
			}
			builder.WriteString("- Docs slug: `")
			builder.WriteString(rule.Slug)
			builder.WriteString("`\n")

			if len(rule.Notes) == 0 {
				continue
			}

			builder.WriteString("- Notes:\n")
			for _, note := range rule.Notes {
				builder.WriteString("  - ")
				builder.WriteString(note)
				builder.WriteString("\n")
			}
		}
	}

	return builder.String()
}

func TextCatalog() string {
	var builder strings.Builder

	builder.WriteString("Firety lint rules\n")
	for _, group := range GroupedRules() {
		builder.WriteString("\n")
		builder.WriteString(group.Title)
		builder.WriteString("\n")

		for _, rule := range group.Rules {
			builder.WriteString(fmt.Sprintf("- %s [%s]\n", rule.ID, rule.Severity))
			if rule.StrictSeverity != "" || rule.PedanticSeverity != "" {
				builder.WriteString("  Strictness: default=")
				builder.WriteString(string(rule.Severity))
				if rule.StrictSeverity != "" {
					builder.WriteString(", strict=")
					builder.WriteString(string(rule.StrictSeverity))
				}
				if rule.PedanticSeverity != "" {
					builder.WriteString(", pedantic=")
					builder.WriteString(string(rule.PedanticSeverity))
				}
				builder.WriteString("\n")
			}
			builder.WriteString("  ")
			builder.WriteString(rule.Title)
			builder.WriteString(". ")
			builder.WriteString(rule.Description)
			builder.WriteString("\n")
			builder.WriteString("  Why: ")
			builder.WriteString(rule.Why)
			builder.WriteString("\n")
			builder.WriteString("  Good: ")
			builder.WriteString(rule.WhatGoodLooksLike)
			builder.WriteString("\n")
			builder.WriteString("  Improve: ")
			builder.WriteString(rule.ImprovementHint)
			builder.WriteString("\n")
			if rule.FixHint != "" {
				builder.WriteString("  Fix hint: ")
				builder.WriteString(rule.FixHint)
				builder.WriteString("\n")
			}
			builder.WriteString("  Metadata: ")
			if rule.ProfileAware {
				builder.WriteString("profile-aware")
			} else {
				builder.WriteString("generic")
			}
			builder.WriteString(", line-aware=")
			builder.WriteString(boolText(rule.LineAware))
			builder.WriteString(", autofix=")
			builder.WriteString(string(rule.Fixability))
			builder.WriteString(", slug=")
			builder.WriteString(rule.Slug)
			builder.WriteString("\n")
		}
	}

	return builder.String()
}

func boolText(value bool) string {
	if value {
		return "yes"
	}

	return "no"
}
