package service

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/domain/readiness"
	workspacepkg "github.com/firety/firety/internal/domain/workspace"
)

const workspaceSkillFileName = "SKILL.md"

type WorkspaceAnalyzeOptions struct {
	Profile          SkillLintProfile
	Strictness       lint.Strictness
	IncludeReadiness bool
	ReadinessContext readiness.PublishContext
	SuitePath        string
	Runner           string
	Backends         []SkillEvalBackendSelection
	GateCriteria     *workspacepkg.GateCriteria
}

type WorkspaceService struct {
	linter    SkillLinter
	readiness SkillReadinessService
}

func NewWorkspaceService(linter SkillLinter, readinessService SkillReadinessService) WorkspaceService {
	return WorkspaceService{
		linter:    linter,
		readiness: readinessService,
	}
}

func (s WorkspaceService) Analyze(root string, options WorkspaceAnalyzeOptions) (workspacepkg.Report, error) {
	discovery, err := discoverWorkspace(root)
	if err != nil {
		return workspacepkg.Report{}, err
	}

	skills := make([]workspacepkg.SkillResult, 0, len(discovery.Skills))
	for _, skill := range discovery.Skills {
		lintReport, err := s.linter.LintWithProfileAndStrictness(skill.Path, options.Profile, options.Strictness)
		if err != nil {
			return workspacepkg.Report{}, fmt.Errorf("lint %s: %w", skill.Path, err)
		}

		result := workspacepkg.SkillResult{
			Skill: skill,
			Lint: workspacepkg.SkillLintSummary{
				Valid:        !lintReport.HasErrors(),
				ErrorCount:   lintReport.ErrorCount(),
				WarningCount: lintReport.WarningCount(),
				Summary:      summarizeLint(lintReport),
			},
		}

		if options.IncludeReadiness {
			readinessResult, err := s.readiness.Evaluate(skill.Path, SkillReadinessOptions{
				Context:    options.ReadinessContext,
				Profiles:   []SkillLintProfile{options.Profile},
				Strictness: options.Strictness,
				SuitePath:  options.SuitePath,
				Runner:     options.Runner,
				Backends:   append([]SkillEvalBackendSelection(nil), options.Backends...),
				Freshness: &readiness.FreshnessSummary{
					Status:          readiness.FreshnessFresh,
					AgeSummary:      "fresh local workspace analysis",
					SupportingPaths: []string{skill.Path},
				},
			})
			if err != nil {
				return workspacepkg.Report{}, fmt.Errorf("evaluate readiness for %s: %w", skill.Path, err)
			}
			result.Readiness = &workspacepkg.SkillReadinessSummary{
				Decision:           readinessResult.Readiness.Decision,
				Summary:            readinessResult.Readiness.Summary,
				SupportPosture:     readinessResult.Readiness.EvidenceSummary.SupportPosture,
				FreshnessStatus:    readinessResult.Readiness.EvidenceSummary.FreshnessStatus,
				TopBlockers:        firstReasons(readinessResult.Readiness.Blockers, 3),
				TopCaveats:         firstReasons(readinessResult.Readiness.Caveats, 3),
				RecommendedActions: workspaceFirstStrings(readinessResult.Readiness.RecommendedActions, 3),
			}
		}

		skills = append(skills, result)
	}

	report := workspacepkg.Report{
		WorkspaceRoot: discovery.WorkspaceRoot,
		Discovery:     discovery,
		Skills:        skills,
	}
	report.Summary = workspacepkg.BuildSummary(report.Skills, report.Discovery.Warnings)
	if options.GateCriteria != nil {
		gateResult := workspacepkg.EvaluateGate(report, *options.GateCriteria)
		report.Gate = &gateResult
	}
	return report, nil
}

func discoverWorkspace(root string) (workspacepkg.Discovery, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return workspacepkg.Discovery{}, err
	}

	skills := make([]workspacepkg.SkillRef, 0, 8)
	warnings := make([]workspacepkg.DiscoveryWarning, 0, 4)
	seen := make(map[string]struct{})

	err = filepath.WalkDir(absoluteRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			warnings = append(warnings, workspacepkg.DiscoveryWarning{
				Path:    path,
				Summary: fmt.Sprintf("could not inspect %s: %v", path, walkErr),
			})
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor":
				if path != absoluteRoot {
					return filepath.SkipDir
				}
			}
			return nil
		}

		if d.Name() != workspaceSkillFileName {
			return nil
		}

		skillDir := filepath.Dir(path)
		if _, ok := seen[skillDir]; ok {
			return nil
		}
		seen[skillDir] = struct{}{}
		skills = append(skills, workspacepkg.SkillRef{
			Name: filepath.Base(skillDir),
			Path: skillDir,
		})
		return nil
	})
	if err != nil {
		return workspacepkg.Discovery{}, err
	}

	if len(skills) == 0 {
		return workspacepkg.Discovery{}, fmt.Errorf("no skill directories containing %s were found under %s", workspaceSkillFileName, absoluteRoot)
	}

	slices.SortFunc(skills, func(a, b workspacepkg.SkillRef) int {
		return strings.Compare(a.Path, b.Path)
	})

	return workspacepkg.Discovery{
		WorkspaceRoot: absoluteRoot,
		SourcePath:    root,
		Skills:        skills,
		Warnings:      warnings,
	}, nil
}

func summarizeLint(report lint.Report) string {
	switch {
	case report.ErrorCount() > 0 && report.WarningCount() > 0:
		return fmt.Sprintf("%d error(s), %d warning(s)", report.ErrorCount(), report.WarningCount())
	case report.ErrorCount() > 0:
		return fmt.Sprintf("%d error(s)", report.ErrorCount())
	case report.WarningCount() > 0:
		return fmt.Sprintf("%d warning(s)", report.WarningCount())
	default:
		return "clean"
	}
}

func firstReasons(values []readiness.Reason, limit int) []readiness.Reason {
	if len(values) <= limit {
		return append([]readiness.Reason(nil), values...)
	}
	return append([]readiness.Reason(nil), values[:limit]...)
}

func workspaceFirstStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return append([]string(nil), values...)
	}
	return append([]string(nil), values[:limit]...)
}
