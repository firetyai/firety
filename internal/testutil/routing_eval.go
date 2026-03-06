package testutil

import (
	"encoding/json"
	"strings"

	domaineval "github.com/firety/firety/internal/domain/eval"
)

func RoutingEvalFixtureSuite() domaineval.RoutingEvalSuite {
	return domaineval.RoutingEvalSuite{
		SchemaVersion: domaineval.RoutingEvalSchemaVersion,
		Name:          "curated-routing-suite",
		Description:   "Small routing-eval corpus covering clear positives, negatives, ambiguity, portability traps, and profile-sensitive prompts.",
		Cases: []domaineval.RoutingEvalCase{
			{
				ID:          "positive-validate-local-skill",
				Label:       "clear positive trigger",
				Prompt:      "Validate this local skill bundle before publishing it.",
				Expectation: domaineval.RoutingShouldTrigger,
				Tags:        []string{"positive", "bundle"},
			},
			{
				ID:          "negative-unrelated-task",
				Label:       "clear negative non-trigger",
				Prompt:      "Draft a postgres migration rollout plan for production.",
				Expectation: domaineval.RoutingShouldNotTrigger,
				Tags:        []string{"negative", "false-positive-trap"},
			},
			{
				ID:          "ambiguous-help-request",
				Label:       "ambiguous prompt",
				Prompt:      "Help me with this workflow.",
				Expectation: domaineval.RoutingShouldNotTrigger,
				Tags:        []string{"ambiguous", "false-positive-trap"},
			},
			{
				ID:          "profile-codex-positive",
				Label:       "profile-sensitive positive",
				Prompt:      "Validate this local skill directory in Codex before publishing changes.",
				Expectation: domaineval.RoutingShouldTrigger,
				Profile:     "codex",
				Tags:        []string{"positive", "profile-sensitive"},
			},
		},
	}
}

func RoutingEvalFixtureJSON() string {
	suite := RoutingEvalFixtureSuite()
	data, err := json.MarshalIndent(suite, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(data) + "\n"
}

func RoutingEvalSkillFiles() map[string]string {
	files := ValidSkillFiles()
	files["evals/routing.json"] = RoutingEvalFixtureJSON()
	return files
}

func RoutingEvalPortableSkillMarkdown() string {
	return strings.Join([]string{
		"---",
		"name: Portable Validation Skill",
		"description: Validates local skill bundles before publishing changes or sharing them with other developers.",
		"---",
		"# Portable Validation Skill",
		"",
		"## When To Use",
		"",
		"Use this skill when you need to validate a local skill bundle before publishing changes or sharing it with other developers.",
		"",
		"## Usage",
		"",
		"Run `firety skill lint .` from the skill root before opening a pull request.",
		"",
		"## Limitations",
		"",
		"Do not use this skill for remote repositories, provider-specific integrations, or runtime execution validation.",
		"",
		"## Examples",
		"",
		"Request: Validate this local skill bundle before publishing it.",
		"Invocation: Run `firety skill lint . --format json` from the skill root.",
		"Result: Review the findings and fix any broken links or weak guidance before publishing.",
	}, "\n")
}
