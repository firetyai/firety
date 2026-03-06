package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/domain/analysis"
	domaineval "github.com/firety/firety/internal/domain/eval"
	"github.com/firety/firety/internal/domain/lint"
	"github.com/firety/firety/internal/service"
)

const SkillAnalysisArtifactSchemaVersion = "1"

type SkillAnalysisArtifactOptions struct {
	Format     string
	Profile    string
	Strictness string
	FailOn     string
	Suite      string
	Runner     string
}

type SkillAnalysisArtifact struct {
	SchemaVersion string                       `json:"schema_version"`
	ArtifactType  string                       `json:"artifact_type"`
	Tool          SkillLintArtifactTool        `json:"tool"`
	Run           SkillAnalysisArtifactRun     `json:"run"`
	Lint          SkillAnalysisArtifactLint    `json:"lint"`
	Eval          SkillAnalysisArtifactEval    `json:"eval"`
	Correlation   analysis.LintEvalCorrelation `json:"correlation,omitempty"`
	Fingerprint   string                       `json:"fingerprint,omitempty"`
}

type SkillAnalysisArtifactRun struct {
	Target       string `json:"target"`
	Profile      string `json:"profile"`
	Strictness   string `json:"strictness"`
	FailOn       string `json:"fail_on"`
	SuitePath    string `json:"suite_path"`
	Runner       string `json:"runner,omitempty"`
	ExitCode     int    `json:"exit_code"`
	StdoutFormat string `json:"stdout_format"`
}

type SkillAnalysisArtifactLint struct {
	Summary     SkillLintArtifactSummary   `json:"summary"`
	Findings    []SkillLintArtifactFinding `json:"findings"`
	RoutingRisk *lint.RoutingRiskSummary   `json:"routing_risk,omitempty"`
	ActionAreas []lint.ActionArea          `json:"action_areas,omitempty"`
	RuleCatalog []SkillLintArtifactRule    `json:"rule_catalog"`
}

type SkillAnalysisArtifactEval struct {
	Suite   domaineval.RoutingEvalSuiteInfo    `json:"suite"`
	Backend domaineval.RoutingEvalBackendInfo  `json:"backend"`
	Summary domaineval.RoutingEvalSummary      `json:"summary"`
	Results []domaineval.RoutingEvalCaseResult `json:"results"`
}

func BuildSkillAnalysisArtifact(version app.VersionInfo, result service.SkillAnalyzeResult, options SkillAnalysisArtifactOptions, exitCode int) SkillAnalysisArtifact {
	lintArtifact := BuildSkillLintArtifact(version, result.LintReport, service.SkillFixResult{}, SkillLintArtifactOptions{
		Format:      options.Format,
		Profile:     options.Profile,
		Strictness:  options.Strictness,
		FailOn:      options.FailOn,
		Explain:     false,
		RoutingRisk: true,
		Fix:         false,
	}, exitCode)
	evalArtifact := BuildSkillEvalArtifact(version, result.EvalReport, SkillEvalArtifactOptions{
		Format:  options.Format,
		Profile: options.Profile,
		Suite:   options.Suite,
		Runner:  options.Runner,
	}, exitCode)

	artifact := SkillAnalysisArtifact{
		SchemaVersion: SkillAnalysisArtifactSchemaVersion,
		ArtifactType:  "firety.skill-analysis",
		Tool: SkillLintArtifactTool{
			Name:    "firety",
			Version: version.Version,
			Commit:  version.Commit,
			Date:    version.Date,
		},
		Run: SkillAnalysisArtifactRun{
			Target:       result.LintReport.Target,
			Profile:      options.Profile,
			Strictness:   options.Strictness,
			FailOn:       options.FailOn,
			SuitePath:    result.EvalReport.Suite.Path,
			Runner:       options.Runner,
			ExitCode:     exitCode,
			StdoutFormat: analysisOptionsOutputFormatDefault(options),
		},
		Lint: SkillAnalysisArtifactLint{
			Summary:     lintArtifact.Summary,
			Findings:    lintArtifact.Findings,
			RoutingRisk: lintArtifact.RoutingRisk,
			ActionAreas: lintArtifact.ActionAreas,
			RuleCatalog: lintArtifact.RuleCatalog,
		},
		Eval: SkillAnalysisArtifactEval{
			Suite:   evalArtifact.Suite,
			Backend: evalArtifact.Backend,
			Summary: evalArtifact.Summary,
			Results: evalArtifact.Results,
		},
		Correlation: result.Correlation,
	}
	artifact.Fingerprint = skillAnalysisArtifactFingerprint(artifact)

	return artifact
}

func WriteSkillAnalysisArtifact(path string, artifact SkillAnalysisArtifact) error {
	if path == "" {
		return fmt.Errorf("artifact path must not be empty")
	}
	if path == "-" {
		return fmt.Errorf(`artifact path "-" is not supported; choose a file path`)
	}

	data, err := marshalSkillAnalysisArtifact(artifact)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func marshalSkillAnalysisArtifact(artifact SkillAnalysisArtifact) ([]byte, error) {
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func skillAnalysisArtifactFingerprint(artifact SkillAnalysisArtifact) string {
	type fingerprintInput struct {
		SchemaVersion string                        `json:"schema_version"`
		Target        string                        `json:"target"`
		Profile       string                        `json:"profile"`
		Strictness    string                        `json:"strictness"`
		FailOn        string                        `json:"fail_on"`
		SuitePath     string                        `json:"suite_path"`
		LintSummary   SkillLintArtifactSummary      `json:"lint_summary"`
		EvalSummary   domaineval.RoutingEvalSummary `json:"eval_summary"`
		Correlation   analysis.LintEvalCorrelation  `json:"correlation,omitempty"`
	}

	payload, _ := json.Marshal(fingerprintInput{
		SchemaVersion: artifact.SchemaVersion,
		Target:        artifact.Run.Target,
		Profile:       artifact.Run.Profile,
		Strictness:    artifact.Run.Strictness,
		FailOn:        artifact.Run.FailOn,
		SuitePath:     artifact.Run.SuitePath,
		LintSummary:   artifact.Lint.Summary,
		EvalSummary:   artifact.Eval.Summary,
		Correlation:   artifact.Correlation,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func analysisOptionsOutputFormatDefault(options SkillAnalysisArtifactOptions) string {
	if options.Format == "" {
		return "text"
	}
	return options.Format
}
