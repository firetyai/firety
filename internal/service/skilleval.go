package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/firety/firety/internal/domain/eval"
	"github.com/firety/firety/internal/platform/evalrunner"
)

const defaultRoutingEvalSuiteRelativePath = "evals/routing.json"

type RoutingEvalBackend interface {
	Name() string
	Evaluate(context.Context, eval.RoutingEvalRequest) (eval.RoutingEvalDecision, error)
}

type SkillEvalOptions struct {
	SuitePath string
	Profile   SkillLintProfile
	Runner    string
}

type SkillEvalBackendSelection struct {
	ID     string
	Runner string
}

type SkillEvalService struct{}

func NewSkillEvalService() SkillEvalService {
	return SkillEvalService{}
}

func (s SkillEvalService) Evaluate(target string, options SkillEvalOptions) (eval.RoutingEvalReport, error) {
	suitePath, suite, skillMarkdown, err := loadRoutingEvalInputs(target, options.SuitePath)
	if err != nil {
		return eval.RoutingEvalReport{}, err
	}

	runnerPath := options.Runner
	if runnerPath == "" {
		runnerPath = os.Getenv("FIRETY_SKILL_EVAL_RUNNER")
	}
	if runnerPath == "" {
		return eval.RoutingEvalReport{}, fmt.Errorf("routing eval runner is not configured; set --runner or FIRETY_SKILL_EVAL_RUNNER")
	}

	backend, err := evalrunner.NewCommandBackend(runnerPath)
	if err != nil {
		return eval.RoutingEvalReport{}, err
	}

	return s.evaluateLoadedInputs(target, suitePath, suite, skillMarkdown, string(options.Profile), eval.RoutingEvalBackendInfo{
		Name:            backend.Name(),
		ProfileAffinity: string(options.Profile),
	}, backend)
}

func (s SkillEvalService) EvaluateAcrossBackends(target string, suitePath string, selections []SkillEvalBackendSelection) (eval.MultiBackendEvalReport, error) {
	if len(selections) < 2 {
		return eval.MultiBackendEvalReport{}, fmt.Errorf("multi-backend eval requires at least two backend selections")
	}

	resolvedSuitePath, suite, skillMarkdown, err := loadRoutingEvalInputs(target, suitePath)
	if err != nil {
		return eval.MultiBackendEvalReport{}, err
	}

	reports := make([]eval.RoutingEvalReport, 0, len(selections))
	for _, selection := range selections {
		definition, ok := eval.FindBackendDefinition(selection.ID)
		if !ok {
			return eval.MultiBackendEvalReport{}, fmt.Errorf("unsupported backend %q", selection.ID)
		}

		runnerPath := selection.Runner
		if runnerPath == "" {
			runnerPath = os.Getenv(definition.RunnerEnvVar)
		}
		if runnerPath == "" {
			return eval.MultiBackendEvalReport{}, fmt.Errorf("routing eval runner for backend %q is not configured; set %s or pass --backend %s=/path/to/runner", definition.ID, definition.RunnerEnvVar, definition.ID)
		}

		backend, err := evalrunner.NewCommandBackend(runnerPath)
		if err != nil {
			return eval.MultiBackendEvalReport{}, err
		}

		report, err := s.evaluateLoadedInputs(target, resolvedSuitePath, suite, skillMarkdown, definition.ProfileAffinity, eval.RoutingEvalBackendInfo{
			ID:              definition.ID,
			Name:            definition.DisplayName,
			ProfileAffinity: definition.ProfileAffinity,
		}, backend)
		if err != nil {
			return eval.MultiBackendEvalReport{}, err
		}
		reports = append(reports, report)
	}

	return eval.BuildMultiBackendEvalReport(target, reports)
}

func (s SkillEvalService) evaluateLoadedInputs(target, suitePath string, suite eval.RoutingEvalSuite, skillMarkdown []byte, defaultProfile string, backendInfo eval.RoutingEvalBackendInfo, backend RoutingEvalBackend) (eval.RoutingEvalReport, error) {
	results := make([]eval.RoutingEvalCaseResult, 0, len(suite.Cases))
	for _, testCase := range suite.Cases {
		profile := defaultProfile
		if profile == "" {
			profile = string(SkillLintProfileGeneric)
		}
		if testCase.Profile != "" {
			profile = testCase.Profile
		}

		decision, err := backend.Evaluate(context.Background(), eval.RoutingEvalRequest{
			SchemaVersion: eval.RoutingEvalSchemaVersion,
			SkillPath:     target,
			SkillMarkdown: string(skillMarkdown),
			Prompt:        testCase.Prompt,
			Profile:       profile,
			CaseID:        testCase.ID,
			Label:         testCase.Label,
			Tags:          append([]string(nil), testCase.Tags...),
		})
		if err != nil {
			return eval.RoutingEvalReport{}, fmt.Errorf("evaluate case %q: %w", testCase.ID, err)
		}

		results = append(results, eval.BuildCaseResult(testCase, profile, decision))
	}

	eval.SortCaseResults(results)

	return eval.RoutingEvalReport{
		Target: target,
		Suite: eval.RoutingEvalSuiteInfo{
			Path:          suitePath,
			SchemaVersion: suite.SchemaVersion,
			Name:          suite.Name,
			CaseCount:     len(suite.Cases),
		},
		Backend: backendInfo,
		Profile: defaultProfile,
		Summary: eval.SummarizeRoutingEval(results),
		Results: results,
	}, nil
}

func loadRoutingEvalInputs(target, requestedSuitePath string) (string, eval.RoutingEvalSuite, []byte, error) {
	suitePath := requestedSuitePath
	if suitePath == "" {
		suitePath = filepath.Join(target, defaultRoutingEvalSuiteRelativePath)
	}

	suite, err := loadRoutingEvalSuite(suitePath)
	if err != nil {
		return "", eval.RoutingEvalSuite{}, nil, err
	}

	skillMarkdown, err := os.ReadFile(filepath.Join(target, "SKILL.md"))
	if err != nil {
		return "", eval.RoutingEvalSuite{}, nil, fmt.Errorf("read SKILL.md: %w", err)
	}

	return suitePath, suite, skillMarkdown, nil
}

func loadRoutingEvalSuite(path string) (eval.RoutingEvalSuite, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return eval.RoutingEvalSuite{}, fmt.Errorf("read routing eval suite: %w", err)
	}

	var suite eval.RoutingEvalSuite
	if err := json.Unmarshal(content, &suite); err != nil {
		return eval.RoutingEvalSuite{}, fmt.Errorf("parse routing eval suite: %w", err)
	}

	if suite.SchemaVersion == "" {
		suite.SchemaVersion = eval.RoutingEvalSchemaVersion
	}
	if suite.SchemaVersion != eval.RoutingEvalSchemaVersion {
		return eval.RoutingEvalSuite{}, fmt.Errorf("unsupported routing eval suite schema version %q", suite.SchemaVersion)
	}
	if suite.Name == "" {
		return eval.RoutingEvalSuite{}, fmt.Errorf("routing eval suite name must not be empty")
	}
	if len(suite.Cases) == 0 {
		return eval.RoutingEvalSuite{}, fmt.Errorf("routing eval suite must contain at least one case")
	}

	seen := make(map[string]struct{}, len(suite.Cases))
	for _, testCase := range suite.Cases {
		if testCase.ID == "" {
			return eval.RoutingEvalSuite{}, fmt.Errorf("routing eval suite case id must not be empty")
		}
		if _, ok := seen[testCase.ID]; ok {
			return eval.RoutingEvalSuite{}, fmt.Errorf("routing eval suite case %q is duplicated", testCase.ID)
		}
		seen[testCase.ID] = struct{}{}

		if testCase.Prompt == "" {
			return eval.RoutingEvalSuite{}, fmt.Errorf("routing eval suite case %q prompt must not be empty", testCase.ID)
		}
		switch testCase.Expectation {
		case eval.RoutingShouldTrigger, eval.RoutingShouldNotTrigger:
		default:
			return eval.RoutingEvalSuite{}, fmt.Errorf("routing eval suite case %q has invalid expectation %q", testCase.ID, testCase.Expectation)
		}
		if testCase.Profile != "" {
			if _, err := ParseSkillLintProfile(testCase.Profile); err != nil {
				return eval.RoutingEvalSuite{}, fmt.Errorf("routing eval suite case %q profile: %w", testCase.ID, err)
			}
		}
	}

	return suite, nil
}
