package service

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/firety/firety/internal/domain/attestation"
	"github.com/firety/firety/internal/domain/compatibility"
	domaineval "github.com/firety/firety/internal/domain/eval"
	domaingate "github.com/firety/firety/internal/domain/gate"
	"github.com/firety/firety/internal/domain/lint"
)

type attestSkillCompatibilityArtifact struct {
	Run struct {
		Target string `json:"target"`
	} `json:"run"`
	Report compatibility.Report `json:"report"`
}

type SkillAttestOptions struct {
	Profiles       []SkillLintProfile
	Strictness     lint.Strictness
	SuitePath      string
	Runner         string
	Backends       []SkillEvalBackendSelection
	InputArtifacts []string
	IncludeGate    bool
}

type SkillAttestResult struct {
	Report attestation.Report
}

type SkillAttestService struct {
	compatibility SkillCompatibilityService
	gate          SkillGateService
	eval          SkillEvalService
}

func NewSkillAttestService(compatibilityService SkillCompatibilityService, gateService SkillGateService, evalService SkillEvalService) SkillAttestService {
	return SkillAttestService{
		compatibility: compatibilityService,
		gate:          gateService,
		eval:          evalService,
	}
}

func (s SkillAttestService) Generate(target string, options SkillAttestOptions) (SkillAttestResult, error) {
	if len(options.InputArtifacts) > 0 {
		return s.generateFromArtifacts(options)
	}
	return s.generateFresh(target, options)
}

func (s SkillAttestService) generateFresh(target string, options SkillAttestOptions) (SkillAttestResult, error) {
	compatibilityResult, err := s.compatibility.Analyze(target, SkillCompatibilityOptions{
		Profiles:   options.Profiles,
		Strictness: options.Strictness,
		SuitePath:  options.SuitePath,
		Backends:   options.Backends,
	})
	if err != nil {
		return SkillAttestResult{}, err
	}

	var (
		testedProfiles []string
		testedBackends []compatibility.BackendSummary
		refs           []attestation.EvidenceRef
		gateResult     *domaingate.Result
	)

	refs = append(refs, attestation.EvidenceRef{
		ID:      "fresh-compatibility",
		Kind:    "compatibility",
		Source:  "fresh-compatibility",
		Summary: compatibilityResult.Report.Summary,
	})

	if len(options.Backends) > 0 {
		report, err := s.eval.EvaluateAcrossBackends(target, options.SuitePath, options.Backends)
		if err != nil {
			return SkillAttestResult{}, err
		}
		testedProfiles = profilesFromMultiEval(report)
		testedBackends = compatibility.BackendSummariesFromMulti(report)
		refs = append(refs, attestation.EvidenceRef{
			ID:           "fresh-routing-eval-multi",
			Kind:         "routing-eval",
			ArtifactType: "firety.skill-routing-eval-multi",
			Source:       "fresh-routing-eval-multi",
			Summary:      report.Summary.Summary,
		})
	} else if options.Runner != "" {
		profile := SkillLintProfileGeneric
		if len(options.Profiles) == 1 {
			profile = options.Profiles[0]
		}
		report, err := s.eval.Evaluate(target, SkillEvalOptions{
			SuitePath: options.SuitePath,
			Profile:   profile,
			Runner:    options.Runner,
		})
		if err != nil {
			return SkillAttestResult{}, err
		}
		testedProfiles = profilesFromEval(report)
		testedBackends = []compatibility.BackendSummary{compatibility.BackendSummaryFromEval(report)}
		refs = append(refs, attestation.EvidenceRef{
			ID:           "fresh-routing-eval",
			Kind:         "routing-eval",
			ArtifactType: "firety.skill-routing-eval",
			Source:       "fresh-routing-eval",
			Summary:      fmt.Sprintf("%.0f%% pass rate on %s.", report.Summary.PassRate*100, report.Backend.Name),
		})
	}

	if options.IncludeGate {
		profile := SkillLintProfileGeneric
		if len(options.Profiles) == 1 {
			profile = options.Profiles[0]
		}
		result, err := s.gate.Evaluate(target, SkillGateOptions{
			Profile:           profile,
			Strictness:        options.Strictness,
			SuitePath:         options.SuitePath,
			Runner:            options.Runner,
			BackendSelections: options.Backends,
		})
		if err != nil {
			return SkillAttestResult{}, err
		}
		gateResult = &result.Gate
		refs = append(refs, attestation.EvidenceRef{
			ID:           "fresh-quality-gate",
			Kind:         "quality-gate",
			ArtifactType: "firety.skill-quality-gate",
			Source:       "fresh-quality-gate",
			Summary:      result.Gate.Summary,
		})
	}

	report := attestation.BuildReport(attestation.Evidence{
		Target:         target,
		Compatibility:  compatibilityResult.Report,
		Gate:           gateResult,
		TestedProfiles: testedProfiles,
		TestedBackends: testedBackends,
		EvidenceRefs:   refs,
	})
	return SkillAttestResult{Report: report}, nil
}

func (s SkillAttestService) generateFromArtifacts(options SkillAttestOptions) (SkillAttestResult, error) {
	compatibilityReport, gateResult, testedProfiles, testedBackends, refs, target, err := s.loadArtifactEvidence(options.InputArtifacts, options.IncludeGate)
	if err != nil {
		return SkillAttestResult{}, err
	}

	report := attestation.BuildReport(attestation.Evidence{
		Target:         target,
		Compatibility:  compatibilityReport,
		Gate:           gateResult,
		TestedProfiles: testedProfiles,
		TestedBackends: testedBackends,
		EvidenceRefs:   refs,
	})
	return SkillAttestResult{Report: report}, nil
}

func (s SkillAttestService) loadArtifactEvidence(paths []string, includeGate bool) (compatibility.Report, *domaingate.Result, []string, []compatibility.BackendSummary, []attestation.EvidenceRef, string, error) {
	type envelope struct {
		ArtifactType string `json:"artifact_type"`
	}

	var (
		explicitCompatibility *compatibility.Report
		explicitGate          *domaingate.Result
		target                string
		testedProfiles        []string
		testedBackends        []compatibility.BackendSummary
		refs                  []attestation.EvidenceRef
	)

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return compatibility.Report{}, nil, nil, nil, nil, "", err
		}

		var header envelope
		if err := json.Unmarshal(content, &header); err != nil {
			return compatibility.Report{}, nil, nil, nil, nil, "", fmt.Errorf("parse artifact envelope: %w", err)
		}

		switch header.ArtifactType {
		case "firety.skill-compatibility":
			var value attestSkillCompatibilityArtifact
			if err := json.Unmarshal(content, &value); err != nil {
				return compatibility.Report{}, nil, nil, nil, nil, "", err
			}
			explicitCompatibility = &value.Report
			target = firstNonEmpty(target, value.Report.Target, value.Run.Target)
			refs = append(refs, attestation.EvidenceRef{
				ID:           refID(path),
				Kind:         "compatibility",
				ArtifactType: header.ArtifactType,
				Source:       path,
				Summary:      value.Report.Summary,
			})
		case "firety.skill-quality-gate":
			var value gateSkillGateArtifact
			if err := json.Unmarshal(content, &value); err != nil {
				return compatibility.Report{}, nil, nil, nil, nil, "", err
			}
			result := value.Result
			explicitGate = &result
			target = firstNonEmpty(target, value.Run.Target)
			refs = append(refs, attestation.EvidenceRef{
				ID:           refID(path),
				Kind:         "quality-gate",
				ArtifactType: header.ArtifactType,
				Source:       path,
				Summary:      value.Result.Summary,
			})
		case "firety.skill-routing-eval":
			var value gateSkillEvalArtifact
			if err := json.Unmarshal(content, &value); err != nil {
				return compatibility.Report{}, nil, nil, nil, nil, "", err
			}
			target = firstNonEmpty(target, value.Run.Target)
			testedProfiles = append(testedProfiles, profilesFromEvalArtifact(value, value.Run.Profile)...)
			testedBackends = append(testedBackends, compatibility.BackendSummary{
				BackendID:      value.Backend.ID,
				BackendName:    value.Backend.Name,
				Status:         backendStatusFromSummary(value.Summary),
				Summary:        fmt.Sprintf("%.0f%% pass rate, %d miss(es).", value.Summary.PassRate*100, value.Summary.Failed),
				PassRate:       value.Summary.PassRate,
				FalsePositives: value.Summary.FalsePositives,
				FalseNegatives: value.Summary.FalseNegatives,
				Failed:         value.Summary.Failed,
			})
			refs = append(refs, attestation.EvidenceRef{
				ID:           refID(path),
				Kind:         "routing-eval",
				ArtifactType: header.ArtifactType,
				Source:       path,
				Summary:      fmt.Sprintf("%.0f%% pass rate on %s.", value.Summary.PassRate*100, value.Backend.Name),
			})
		case "firety.skill-routing-eval-multi":
			var value gateSkillEvalMultiArtifact
			if err := json.Unmarshal(content, &value); err != nil {
				return compatibility.Report{}, nil, nil, nil, nil, "", err
			}
			target = firstNonEmpty(target, value.Run.Target)
			testedProfiles = append(testedProfiles, profilesFromMultiEvalArtifact(value)...)
			testedBackends = append(testedBackends, compatibility.BackendSummariesFromMulti(domaineval.MultiBackendEvalReport{
				Target:         value.Run.Target,
				Suite:          value.Suite,
				Backends:       value.Results,
				Summary:        value.Summary,
				DifferingCases: value.DifferingCases,
			})...)
			refs = append(refs, attestation.EvidenceRef{
				ID:           refID(path),
				Kind:         "routing-eval",
				ArtifactType: header.ArtifactType,
				Source:       path,
				Summary:      value.Summary.Summary,
			})
		case "firety.skill-analysis":
			var value gateSkillAnalysisArtifact
			if err := json.Unmarshal(content, &value); err != nil {
				return compatibility.Report{}, nil, nil, nil, nil, "", err
			}
			target = firstNonEmpty(target, value.Run.Target)
			testedProfiles = append(testedProfiles, profilesFromEvalArtifact(gateSkillEvalArtifact{
				Run: struct {
					Target  string `json:"target"`
					Profile string `json:"profile,omitempty"`
					Runner  string `json:"runner,omitempty"`
				}{Target: value.Run.Target, Profile: value.Run.Profile},
				Summary: value.Eval.Summary,
			}, value.Run.Profile)...)
			testedBackends = append(testedBackends, compatibility.BackendSummary{
				BackendID:      value.Eval.Backend.ID,
				BackendName:    value.Eval.Backend.Name,
				Status:         backendStatusFromSummary(value.Eval.Summary),
				Summary:        fmt.Sprintf("%.0f%% pass rate, %d miss(es).", value.Eval.Summary.PassRate*100, value.Eval.Summary.Failed),
				PassRate:       value.Eval.Summary.PassRate,
				FalsePositives: value.Eval.Summary.FalsePositives,
				FalseNegatives: value.Eval.Summary.FalseNegatives,
				Failed:         value.Eval.Summary.Failed,
			})
			refs = append(refs, attestation.EvidenceRef{
				ID:           refID(path),
				Kind:         "routing-eval",
				ArtifactType: header.ArtifactType,
				Source:       path,
				Summary:      fmt.Sprintf("%.0f%% pass rate on %s.", value.Eval.Summary.PassRate*100, value.Eval.Backend.Name),
			})
		case "firety.skill-lint":
			var value gateSkillLintArtifact
			if err := json.Unmarshal(content, &value); err != nil {
				return compatibility.Report{}, nil, nil, nil, nil, "", err
			}
			target = firstNonEmpty(target, value.Run.Target)
			refs = append(refs, attestation.EvidenceRef{
				ID:           refID(path),
				Kind:         "lint",
				ArtifactType: header.ArtifactType,
				Source:       path,
				Summary:      fmt.Sprintf("%d error(s), %d warning(s), %d finding(s).", value.Summary.ErrorCount, value.Summary.WarningCount, len(value.Findings)),
			})
		default:
			refs = append(refs, attestation.EvidenceRef{
				ID:           refID(path),
				Kind:         "artifact",
				ArtifactType: header.ArtifactType,
				Source:       path,
			})
		}
	}

	compatibilityReport := compatibility.Report{}
	if explicitCompatibility != nil {
		compatibilityReport = *explicitCompatibility
	} else {
		result, err := s.compatibility.Analyze("", SkillCompatibilityOptions{
			InputArtifacts: append([]string(nil), paths...),
		})
		if err != nil {
			return compatibility.Report{}, nil, nil, nil, nil, "", err
		}
		compatibilityReport = result.Report
		refs = append(refs, attestation.EvidenceRef{
			ID:      "derived-compatibility",
			Kind:    "compatibility",
			Source:  "derived-compatibility",
			Summary: compatibilityReport.Summary,
		})
	}

	var gateResult *domaingate.Result
	if explicitGate != nil {
		gateResult = explicitGate
	} else if includeGate {
		result, err := s.gate.Evaluate("", SkillGateOptions{
			InputArtifacts: append([]string(nil), paths...),
		})
		if err != nil {
			return compatibility.Report{}, nil, nil, nil, nil, "", err
		}
		gateResult = &result.Gate
		refs = append(refs, attestation.EvidenceRef{
			ID:      "derived-quality-gate",
			Kind:    "quality-gate",
			Source:  "derived-quality-gate",
			Summary: result.Gate.Summary,
		})
	}

	return compatibilityReport, gateResult, uniqueStrings(testedProfiles), uniqueBackendSummaries(testedBackends), refs, firstNonEmpty(target, compatibilityReport.Target), nil
}

func profilesFromEval(report domaineval.RoutingEvalReport) []string {
	values := make([]string, 0, len(report.Summary.ByProfile)+1)
	if report.Profile != "" {
		values = append(values, report.Profile)
	}
	for _, item := range report.Summary.ByProfile {
		values = append(values, item.Key)
	}
	return uniqueStrings(values)
}

func profilesFromEvalArtifact(value gateSkillEvalArtifact, profile string) []string {
	values := make([]string, 0, len(value.Summary.ByProfile)+1)
	if profile != "" {
		values = append(values, profile)
	}
	for _, item := range value.Summary.ByProfile {
		values = append(values, item.Key)
	}
	return uniqueStrings(values)
}

func profilesFromMultiEval(report domaineval.MultiBackendEvalReport) []string {
	values := make([]string, 0)
	for _, backend := range report.Backends {
		for _, item := range backend.Summary.ByProfile {
			values = append(values, item.Key)
		}
	}
	return uniqueStrings(values)
}

func profilesFromMultiEvalArtifact(value gateSkillEvalMultiArtifact) []string {
	values := make([]string, 0)
	for _, backend := range value.Results {
		for _, item := range backend.Summary.ByProfile {
			values = append(values, item.Key)
		}
	}
	return uniqueStrings(values)
}

func backendStatusFromSummary(summary domaineval.RoutingEvalSummary) compatibility.Status {
	switch {
	case summary.Failed == 0 && summary.PassRate >= 1:
		return compatibility.StatusStrong
	case summary.PassRate >= 0.75:
		return compatibility.StatusMixed
	default:
		return compatibility.StatusRisky
	}
}

func uniqueBackendSummaries(values []compatibility.BackendSummary) []compatibility.BackendSummary {
	out := make([]compatibility.BackendSummary, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		key := value.BackendID
		if key == "" {
			key = value.BackendName
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	slices.SortFunc(out, func(a, b compatibility.BackendSummary) int {
		return strings.Compare(a.BackendID+a.BackendName, b.BackendID+b.BackendName)
	})
	return out
}

func uniqueStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	slices.Sort(out)
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func refID(path string) string {
	base := path
	if index := strings.LastIndex(base, "/"); index >= 0 {
		base = base[index+1:]
	}
	if base == "" {
		return "artifact"
	}
	return base
}
