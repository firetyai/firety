package lint

import "fmt"

type ActionArea struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

func SummarizeActionAreas(findings []Finding) []ActionArea {
	areaOrder := []ActionArea{
		{
			Key:     "structure",
			Title:   "Fix structural issues",
			Summary: "Repair entry-document and link problems first so the rest of the lint signal reflects the real skill bundle.",
		},
		{
			Key:     "metadata",
			Title:   "Improve metadata clarity",
			Summary: "Align the title, front matter, and core description so the skill's identity and scope are easy to trust.",
		},
		{
			Key:     "guidance",
			Title:   "Tighten trigger and usage guidance",
			Summary: "Clarify when to use the skill, when not to use it, and what request or input pattern should trigger it.",
		},
		{
			Key:     "examples",
			Title:   "Strengthen examples",
			Summary: "Use concrete examples that reinforce the documented trigger, invocation pattern, and expected outcome.",
		},
		{
			Key:     "portability",
			Title:   "Reduce accidental tool lock-in",
			Summary: "Either keep the wording portable across tools or state the intended tool target and audience boundaries explicitly.",
		},
		{
			Key:     "bundle",
			Title:   "Clean up bundle resources",
			Summary: "Make the documented local resources match the actual package contents and keep references stable and useful.",
		},
		{
			Key:     "cost",
			Title:   "Trim costly content",
			Summary: "Cut repeated or oversized instructions and examples so the skill stays cheaper to load and easier to maintain.",
		},
	}

	seen := map[string]bool{}
	for _, finding := range findings {
		rule, ok := FindRule(finding.RuleID)
		if !ok {
			continue
		}

		key := actionAreaKey(rule.Category)
		if key != "" {
			seen[key] = true
		}
	}

	summary := make([]ActionArea, 0, len(areaOrder))
	for _, area := range areaOrder {
		if seen[area.Key] {
			summary = append(summary, area)
		}
	}

	return summary
}

func StrictnessSummary(strictness Strictness) string {
	switch strictness {
	case StrictnessStrict:
		return "strict mode raises expectations for metadata completeness, invocation clarity, examples, and portability discipline."
	case StrictnessPedantic:
		return "pedantic mode expects explicit, disciplined skill authoring and escalates additional completeness findings to errors."
	default:
		return ""
	}
}

func actionAreaKey(category Category) string {
	switch category {
	case CategoryStructure:
		return "structure"
	case CategoryMetadataSpec, CategoryConsistency:
		return "metadata"
	case CategoryInvocation, CategoryNegativeGuidance, CategoryTriggerQuality:
		return "guidance"
	case CategoryExamples:
		return "examples"
	case CategoryPortability:
		return "portability"
	case CategoryBundleResources:
		return "bundle"
	case CategoryEfficiencyCost:
		return "cost"
	default:
		return ""
	}
}

func StrictnessDescription(strictness Strictness) string {
	switch strictness {
	case StrictnessStrict:
		return "Strict mode raises selected severities and tightens a small set of quality and efficiency thresholds."
	case StrictnessPedantic:
		return "Pedantic mode applies the strongest built-in completeness and discipline checks intended for highly opinionated teams."
	default:
		return "Default mode preserves Firety's current conservative, high-signal lint posture."
	}
}

func StrictnessMatrix(rule Rule) string {
	if rule.StrictSeverity == "" && rule.PedanticSeverity == "" {
		return ""
	}

	matrix := fmt.Sprintf("default=%s", rule.Severity)
	if rule.StrictSeverity != "" {
		matrix += fmt.Sprintf(", strict=%s", rule.StrictSeverity)
	}
	if rule.PedanticSeverity != "" {
		matrix += fmt.Sprintf(", pedantic=%s", rule.PedanticSeverity)
	}
	return matrix
}
