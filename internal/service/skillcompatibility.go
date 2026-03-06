package service

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"github.com/firety/firety/internal/domain/compatibility"
	domaineval "github.com/firety/firety/internal/domain/eval"
	"github.com/firety/firety/internal/domain/lint"
)

type SkillCompatibilityOptions struct {
	Profiles       []SkillLintProfile
	Strictness     lint.Strictness
	SuitePath      string
	Backends       []SkillEvalBackendSelection
	InputArtifacts []string
}

type SkillCompatibilityResult struct {
	Report compatibility.Report
}

type SkillCompatibilityService struct {
	linter SkillLinter
	eval   SkillEvalService
}

func NewSkillCompatibilityService(linter SkillLinter, eval SkillEvalService) SkillCompatibilityService {
	return SkillCompatibilityService{
		linter: linter,
		eval:   eval,
	}
}

func (s SkillCompatibilityService) Analyze(target string, options SkillCompatibilityOptions) (SkillCompatibilityResult, error) {
	if len(options.InputArtifacts) > 0 {
		report, err := s.analyzeFromArtifacts(options.InputArtifacts)
		if err != nil {
			return SkillCompatibilityResult{}, err
		}
		return SkillCompatibilityResult{Report: report}, nil
	}

	profiles := options.Profiles
	if len(profiles) == 0 {
		profiles = []SkillLintProfile{
			SkillLintProfileGeneric,
			SkillLintProfileCodex,
			SkillLintProfileClaudeCode,
			SkillLintProfileCopilot,
			SkillLintProfileCursor,
		}
	}

	evidence := compatibility.Evidence{Target: target}
	for _, profile := range profiles {
		report, err := s.linter.LintWithProfileAndStrictness(target, profile, options.Strictness)
		if err != nil {
			return SkillCompatibilityResult{}, err
		}
		evidence.Profiles = append(evidence.Profiles, compatibility.ProfileSummaryFromLint(string(profile), report))
	}

	if len(options.Backends) > 0 {
		if len(options.Backends) == 1 {
			report, err := s.eval.Evaluate(target, SkillEvalOptions{
				SuitePath: options.SuitePath,
				Profile:   SkillLintProfile(options.Backends[0].ID),
				Runner:    options.Backends[0].Runner,
			})
			if err != nil {
				return SkillCompatibilityResult{}, err
			}
			evidence.Backends = append(evidence.Backends, compatibility.BackendSummaryFromEval(report))
		} else {
			report, err := s.eval.EvaluateAcrossBackends(target, options.SuitePath, options.Backends)
			if err != nil {
				return SkillCompatibilityResult{}, err
			}
			evidence.Backends = compatibility.BackendSummariesFromMulti(report)
		}
	}

	return SkillCompatibilityResult{Report: compatibility.BuildReport(evidence)}, nil
}

func (s SkillCompatibilityService) analyzeFromArtifacts(paths []string) (compatibility.Report, error) {
	evidence := compatibility.Evidence{}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return compatibility.Report{}, err
		}

		var envelope gateArtifactEnvelope
		if err := json.Unmarshal(content, &envelope); err != nil {
			return compatibility.Report{}, fmt.Errorf("parse artifact envelope: %w", err)
		}

		switch envelope.ArtifactType {
		case "firety.skill-lint":
			var value gateSkillLintArtifact
			if err := json.Unmarshal(content, &value); err != nil {
				return compatibility.Report{}, err
			}
			report := lint.Report{
				Target:   value.Run.Target,
				Findings: lintFindingsFromArtifact(value.Findings),
			}
			profile := value.Run.Profile
			if profile == "" {
				profile = string(SkillLintProfileGeneric)
			}
			evidence.Target = nonEmptyString(evidence.Target, value.Run.Target)
			evidence.Profiles = appendOrReplaceProfileSummary(evidence.Profiles, compatibility.ProfileSummaryFromLint(profile, report))
		case "firety.skill-analysis":
			var value gateSkillAnalysisArtifact
			if err := json.Unmarshal(content, &value); err != nil {
				return compatibility.Report{}, err
			}
			report := lint.Report{
				Target:   value.Run.Target,
				Findings: lintFindingsFromArtifact(value.Lint.Findings),
			}
			profile := value.Run.Profile
			if profile == "" {
				profile = string(SkillLintProfileGeneric)
			}
			evidence.Target = nonEmptyString(evidence.Target, value.Run.Target)
			evidence.Profiles = appendOrReplaceProfileSummary(evidence.Profiles, compatibility.ProfileSummaryFromLint(profile, report))
			evidence.Backends = appendOrReplaceBackendSummary(evidence.Backends, compatibility.BackendSummaryFromEval(domaineval.RoutingEvalReport{
				Target:  value.Run.Target,
				Suite:   value.Eval.Suite,
				Backend: value.Eval.Backend,
				Summary: value.Eval.Summary,
				Results: value.Eval.Results,
			}))
		case "firety.skill-routing-eval":
			var value gateSkillEvalArtifact
			if err := json.Unmarshal(content, &value); err != nil {
				return compatibility.Report{}, err
			}
			evidence.Target = nonEmptyString(evidence.Target, value.Run.Target)
			evidence.Backends = appendOrReplaceBackendSummary(evidence.Backends, compatibility.BackendSummaryFromEval(domaineval.RoutingEvalReport{
				Target:  value.Run.Target,
				Suite:   value.Suite,
				Backend: value.Backend,
				Summary: value.Summary,
				Results: value.Results,
			}))
		case "firety.skill-routing-eval-multi":
			var value gateSkillEvalMultiArtifact
			if err := json.Unmarshal(content, &value); err != nil {
				return compatibility.Report{}, err
			}
			evidence.Target = nonEmptyString(evidence.Target, value.Run.Target)
			for _, summary := range compatibility.BackendSummariesFromMulti(domaineval.MultiBackendEvalReport{
				Target:         value.Run.Target,
				Suite:          value.Suite,
				Backends:       value.Results,
				Summary:        value.Summary,
				DifferingCases: value.DifferingCases,
			}) {
				evidence.Backends = appendOrReplaceBackendSummary(evidence.Backends, summary)
			}
		default:
			return compatibility.Report{}, fmt.Errorf("artifact %s has unsupported type %q for compatibility analysis", path, envelope.ArtifactType)
		}
	}

	return compatibility.BuildReport(evidence), nil
}

func appendOrReplaceProfileSummary(values []compatibility.ProfileSummary, item compatibility.ProfileSummary) []compatibility.ProfileSummary {
	for index, value := range values {
		if value.Profile == item.Profile {
			values[index] = item
			return values
		}
	}
	return append(values, item)
}

func appendOrReplaceBackendSummary(values []compatibility.BackendSummary, item compatibility.BackendSummary) []compatibility.BackendSummary {
	for index, value := range values {
		if value.BackendID == item.BackendID {
			values[index] = item
			return values
		}
	}
	return append(values, item)
}

func nonEmptyString(current, candidate string) string {
	if current != "" {
		return current
	}
	return candidate
}

func ParseSkillLintProfiles(values []string) ([]SkillLintProfile, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := make([]SkillLintProfile, 0, len(values))
	seen := make(map[SkillLintProfile]struct{}, len(values))
	for _, value := range values {
		profile, err := ParseSkillLintProfile(value)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[profile]; ok {
			continue
		}
		seen[profile] = struct{}{}
		out = append(out, profile)
	}
	slices.SortStableFunc(out, func(left, right SkillLintProfile) int {
		return stringsCompare(string(left), string(right))
	})
	return out, nil
}

func stringsCompare(left, right string) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}
