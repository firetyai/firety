package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/firety/firety/internal/domain/attestation"
	"github.com/firety/firety/internal/domain/compatibility"
	"github.com/firety/firety/internal/domain/gate"
	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/domain/readiness"
)

type SkillReadinessOptions struct {
	Context        readiness.PublishContext
	Profiles       []SkillLintProfile
	Strictness     lint.Strictness
	SuitePath      string
	Runner         string
	Backends       []SkillEvalBackendSelection
	InputArtifacts []string
	InputPacks     []string
	InputReports   []string
	Freshness      *readiness.FreshnessSummary
}

type SkillReadinessResult struct {
	Readiness readiness.Result
}

type SkillReadinessService struct {
	compatibility SkillCompatibilityService
	gate          SkillGateService
	attest        SkillAttestService
}

func NewSkillReadinessService(
	compatibilityService SkillCompatibilityService,
	gateService SkillGateService,
	attestService SkillAttestService,
) SkillReadinessService {
	return SkillReadinessService{
		compatibility: compatibilityService,
		gate:          gateService,
		attest:        attestService,
	}
}

func (s SkillReadinessService) Evaluate(target string, options SkillReadinessOptions) (SkillReadinessResult, error) {
	if strings.TrimSpace(target) == "" && len(options.InputArtifacts) == 0 && len(options.InputPacks) == 0 && len(options.InputReports) == 0 {
		return SkillReadinessResult{}, fmt.Errorf("readiness requires a target path or at least one input artifact, pack, or report")
	}

	inputArtifacts, err := resolveReadinessInputs(options.InputArtifacts, options.InputPacks, options.InputReports)
	if err != nil {
		return SkillReadinessResult{}, err
	}

	var (
		compatibilityResult SkillCompatibilityResult
		gateResult          SkillGateResult
		attestResult        SkillAttestResult
	)

	if strings.TrimSpace(target) != "" {
		compatibilityResult, err = s.compatibility.Analyze(target, SkillCompatibilityOptions{
			Profiles:   options.Profiles,
			Strictness: options.Strictness,
			SuitePath:  options.SuitePath,
			Backends:   options.Backends,
		})
		if err != nil {
			return SkillReadinessResult{}, err
		}

		gateResult, err = s.gate.Evaluate(target, SkillGateOptions{
			Profile:           readinessPrimaryProfile(options.Profiles),
			Strictness:        options.Strictness,
			SuitePath:         options.SuitePath,
			Runner:            options.Runner,
			BackendSelections: options.Backends,
		})
		if err != nil {
			return SkillReadinessResult{}, err
		}

		attestResult, err = s.attest.Generate(target, SkillAttestOptions{
			Profiles:    options.Profiles,
			Strictness:  options.Strictness,
			SuitePath:   options.SuitePath,
			Runner:      options.Runner,
			Backends:    options.Backends,
			IncludeGate: true,
		})
		if err != nil {
			return SkillReadinessResult{}, err
		}

	} else {
		compatibilityInputs, err := filterReadinessArtifacts(inputArtifacts, compatibilityArtifactTypes())
		if err != nil {
			return SkillReadinessResult{}, err
		}
		if len(compatibilityInputs) > 0 {
			compatibilityResult, err = s.compatibility.Analyze("", SkillCompatibilityOptions{
				InputArtifacts: compatibilityInputs,
			})
			if err != nil {
				return SkillReadinessResult{}, err
			}
		}

		gateInputs, err := filterReadinessArtifacts(inputArtifacts, gateArtifactTypes())
		if err != nil {
			return SkillReadinessResult{}, err
		}
		if len(gateInputs) > 0 {
			gateResult, err = s.gate.Evaluate("", SkillGateOptions{
				InputArtifacts: gateInputs,
			})
			if err != nil {
				return SkillReadinessResult{}, err
			}
		}

		attestInputs, err := filterReadinessArtifacts(inputArtifacts, attestArtifactTypes())
		if err != nil {
			return SkillReadinessResult{}, err
		}
		if len(attestInputs) > 0 {
			attestResult, err = s.attest.Generate("", SkillAttestOptions{
				InputArtifacts: attestInputs,
				IncludeGate:    true,
			})
			if err != nil {
				return SkillReadinessResult{}, err
			}
		}

	}

	readinessResult := readiness.Build(readiness.Evidence{
		Target:        target,
		Context:       options.Context,
		Gate:          optionalGateResult(gateResult),
		Compatibility: optionalCompatibilityReport(compatibilityResult),
		Attestation:   optionalAttestationReport(attestResult),
		Freshness:     options.Freshness,
		ArtifactRefs:  append([]string(nil), inputArtifacts...),
	})
	return SkillReadinessResult{Readiness: readinessResult}, nil
}

func readinessPrimaryProfile(profiles []SkillLintProfile) SkillLintProfile {
	if len(profiles) == 1 {
		return profiles[0]
	}
	return SkillLintProfileGeneric
}

func resolveReadinessInputs(artifacts, packs, reports []string) ([]string, error) {
	resolvedArtifacts := append([]string(nil), artifacts...)

	for _, pack := range packs {
		packArtifacts, err := loadPackArtifactPaths(pack)
		if err != nil {
			return nil, err
		}
		resolvedArtifacts = append(resolvedArtifacts, packArtifacts...)
	}

	for _, report := range reports {
		reportArtifacts, err := loadTrustReportArtifactPaths(report)
		if err != nil {
			return nil, err
		}
		resolvedArtifacts = append(resolvedArtifacts, reportArtifacts...)
	}

	return uniqueSortedAbsPaths(resolvedArtifacts), nil
}

func loadPackArtifactPaths(packDir string) ([]string, error) {
	manifestPath := filepath.Join(packDir, "manifest.json")
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	var manifest struct {
		PackType  string `json:"pack_type"`
		Artifacts []struct {
			Path string `json:"path"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(content, &manifest); err != nil {
		return nil, fmt.Errorf("parse evidence-pack manifest: %w", err)
	}
	if manifest.PackType != "firety.evidence-pack" {
		return nil, fmt.Errorf("directory %s does not contain a supported Firety evidence pack", packDir)
	}

	out := make([]string, 0, len(manifest.Artifacts))
	for _, item := range manifest.Artifacts {
		if strings.TrimSpace(item.Path) == "" {
			continue
		}
		out = append(out, filepath.Join(packDir, filepath.FromSlash(item.Path)))
	}
	return uniqueSortedAbsPaths(out), nil
}

func loadTrustReportArtifactPaths(reportDir string) ([]string, error) {
	manifestPath := filepath.Join(reportDir, "manifest.json")
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	var manifest struct {
		ReportType string `json:"report_type"`
		Artifacts  []struct {
			Path string `json:"path"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(content, &manifest); err != nil {
		return nil, fmt.Errorf("parse trust-report manifest: %w", err)
	}
	if manifest.ReportType != "firety.trust-report" {
		return nil, fmt.Errorf("directory %s does not contain a supported Firety trust report", reportDir)
	}

	out := make([]string, 0, len(manifest.Artifacts))
	for _, item := range manifest.Artifacts {
		if strings.TrimSpace(item.Path) == "" {
			continue
		}
		out = append(out, filepath.Join(reportDir, filepath.FromSlash(item.Path)))
	}
	return uniqueSortedAbsPaths(out), nil
}

func uniqueSortedAbsPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			absolute = path
		}
		if _, ok := seen[absolute]; ok {
			continue
		}
		seen[absolute] = struct{}{}
		out = append(out, absolute)
	}
	slices.Sort(out)
	return out
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
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

func filterReadinessArtifacts(paths []string, supported map[string]struct{}) ([]string, error) {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var envelope struct {
			ArtifactType string `json:"artifact_type"`
		}
		if err := json.Unmarshal(content, &envelope); err != nil {
			return nil, fmt.Errorf("parse artifact envelope: %w", err)
		}
		if _, ok := supported[envelope.ArtifactType]; ok {
			out = append(out, path)
		}
	}
	return uniqueSortedAbsPaths(out), nil
}

func compatibilityArtifactTypes() map[string]struct{} {
	return map[string]struct{}{
		"firety.skill-lint":               {},
		"firety.skill-analysis":           {},
		"firety.skill-routing-eval":       {},
		"firety.skill-routing-eval-multi": {},
	}
}

func gateArtifactTypes() map[string]struct{} {
	return map[string]struct{}{
		"firety.skill-lint":                       {},
		"firety.skill-analysis":                   {},
		"firety.skill-lint-compare":               {},
		"firety.skill-routing-eval":               {},
		"firety.skill-routing-eval-compare":       {},
		"firety.skill-routing-eval-multi":         {},
		"firety.skill-routing-eval-compare-multi": {},
		"firety.skill-baseline-compare":           {},
	}
}

func attestArtifactTypes() map[string]struct{} {
	return map[string]struct{}{
		"firety.skill-compatibility":      {},
		"firety.skill-quality-gate":       {},
		"firety.skill-routing-eval":       {},
		"firety.skill-routing-eval-multi": {},
		"firety.skill-analysis":           {},
		"firety.skill-lint":               {},
	}
}

func optionalGateResult(result SkillGateResult) *gate.Result {
	if result.Gate.Decision == "" && result.Gate.Summary == "" {
		return nil
	}
	return &result.Gate
}

func optionalCompatibilityReport(result SkillCompatibilityResult) *compatibility.Report {
	if result.Report.SupportPosture == "" && result.Report.Summary == "" {
		return nil
	}
	return &result.Report
}

func optionalAttestationReport(result SkillAttestResult) *attestation.Report {
	if result.Report.SupportPosture == "" && result.Report.Summary == "" {
		return nil
	}
	return &result.Report
}
