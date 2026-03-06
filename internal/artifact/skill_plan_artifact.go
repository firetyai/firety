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

const SkillPlanArtifactSchemaVersion = "1"

type SkillPlanArtifactOptions struct {
	Format     string
	Profile    string
	Strictness string
	FailOn     string
	Suite      string
}

type SkillPlanArtifact struct {
	SchemaVersion    string                              `json:"schema_version"`
	ArtifactType     string                              `json:"artifact_type"`
	Tool             SkillLintArtifactTool               `json:"tool"`
	Run              SkillPlanArtifactRun                `json:"run"`
	LintSummary      SkillPlanLintSummary                `json:"lint_summary"`
	RoutingRisk      lint.RoutingRiskSummary             `json:"routing_risk"`
	ActionAreas      []lint.ActionArea                   `json:"action_areas,omitempty"`
	EvalSummary      *domaineval.RoutingEvalSummary      `json:"eval_summary,omitempty"`
	Correlation      *analysis.LintEvalCorrelation       `json:"correlation,omitempty"`
	MultiBackendEval *domaineval.MultiBackendEvalSummary `json:"multi_backend_eval,omitempty"`
	Plan             analysis.ImprovementPlan            `json:"plan"`
	Fingerprint      string                              `json:"fingerprint,omitempty"`
}

type SkillPlanArtifactRun struct {
	Target       string `json:"target"`
	Profile      string `json:"profile"`
	Strictness   string `json:"strictness"`
	FailOn       string `json:"fail_on"`
	SuitePath    string `json:"suite_path,omitempty"`
	ExitCode     int    `json:"exit_code"`
	StdoutFormat string `json:"stdout_format"`
}

type SkillPlanLintSummary struct {
	ErrorCount   int `json:"error_count"`
	WarningCount int `json:"warning_count"`
}

func BuildSkillPlanArtifact(version app.VersionInfo, result service.SkillPlanResult, options SkillPlanArtifactOptions, exitCode int) SkillPlanArtifact {
	artifact := SkillPlanArtifact{
		SchemaVersion: SkillPlanArtifactSchemaVersion,
		ArtifactType:  "firety.skill-improvement-plan",
		Tool: SkillLintArtifactTool{
			Name:    "firety",
			Version: version.Version,
			Commit:  version.Commit,
			Date:    version.Date,
		},
		Run: SkillPlanArtifactRun{
			Target:       result.LintReport.Target,
			Profile:      options.Profile,
			Strictness:   options.Strictness,
			FailOn:       options.FailOn,
			ExitCode:     exitCode,
			StdoutFormat: planOptionsOutputFormatDefault(options),
		},
		LintSummary: SkillPlanLintSummary{
			ErrorCount:   result.LintReport.ErrorCount(),
			WarningCount: result.LintReport.WarningCount(),
		},
		RoutingRisk: result.RoutingRisk,
		ActionAreas: result.ActionAreas,
		Plan:        result.Plan,
	}
	if result.EvalReport != nil {
		artifact.Run.SuitePath = result.EvalReport.Suite.Path
		artifact.EvalSummary = &result.EvalReport.Summary
	}
	if result.Correlation.HasEvidence() || result.Correlation.Summary != "" {
		artifact.Correlation = &result.Correlation
	}
	if result.MultiBackendEval != nil {
		artifact.Run.SuitePath = result.MultiBackendEval.Suite.Path
		artifact.MultiBackendEval = &result.MultiBackendEval.Summary
	}
	artifact.Fingerprint = skillPlanArtifactFingerprint(artifact)
	return artifact
}

func WriteSkillPlanArtifact(path string, artifact SkillPlanArtifact) error {
	if path == "" {
		return fmt.Errorf("artifact path must not be empty")
	}
	if path == "-" {
		return fmt.Errorf(`artifact path "-" is not supported; choose a file path`)
	}

	data, err := marshalSkillPlanArtifact(artifact)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func marshalSkillPlanArtifact(artifact SkillPlanArtifact) ([]byte, error) {
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func skillPlanArtifactFingerprint(artifact SkillPlanArtifact) string {
	type fingerprintInput struct {
		SchemaVersion string                              `json:"schema_version"`
		Target        string                              `json:"target"`
		Profile       string                              `json:"profile"`
		Strictness    string                              `json:"strictness"`
		FailOn        string                              `json:"fail_on"`
		LintSummary   SkillPlanLintSummary                `json:"lint_summary"`
		EvalSummary   *domaineval.RoutingEvalSummary      `json:"eval_summary,omitempty"`
		MultiBackend  *domaineval.MultiBackendEvalSummary `json:"multi_backend_eval,omitempty"`
		Plan          analysis.ImprovementPlan            `json:"plan"`
	}

	payload, _ := json.Marshal(fingerprintInput{
		SchemaVersion: artifact.SchemaVersion,
		Target:        artifact.Run.Target,
		Profile:       artifact.Run.Profile,
		Strictness:    artifact.Run.Strictness,
		FailOn:        artifact.Run.FailOn,
		LintSummary:   artifact.LintSummary,
		EvalSummary:   artifact.EvalSummary,
		MultiBackend:  artifact.MultiBackendEval,
		Plan:          artifact.Plan,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func planOptionsOutputFormatDefault(options SkillPlanArtifactOptions) string {
	if options.Format == "" {
		return "text"
	}
	return options.Format
}
