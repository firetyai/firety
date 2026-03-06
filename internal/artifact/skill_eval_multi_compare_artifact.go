package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/firety/firety/internal/app"
	domaineval "github.com/firety/firety/internal/domain/eval"
	"github.com/firety/firety/internal/service"
)

const SkillEvalMultiCompareArtifactSchemaVersion = "1"

type SkillEvalMultiCompareArtifactOptions struct {
	Format string
	Suite  string
}

type SkillEvalMultiCompareArtifact struct {
	SchemaVersion         string                                       `json:"schema_version"`
	ArtifactType          string                                       `json:"artifact_type"`
	Tool                  SkillLintArtifactTool                        `json:"tool"`
	Run                   SkillEvalMultiCompareArtifactRun             `json:"run"`
	Suite                 domaineval.RoutingEvalSuiteInfo              `json:"suite"`
	Backends              []domaineval.RoutingEvalBackendInfo          `json:"backends"`
	Base                  domaineval.RoutingEvalSideSummary            `json:"base"`
	Candidate             domaineval.RoutingEvalSideSummary            `json:"candidate"`
	AggregateSummary      domaineval.MultiBackendEvalComparisonSummary `json:"aggregate_summary"`
	PerBackendDeltas      []domaineval.BackendEvalComparison           `json:"per_backend_deltas"`
	DifferingCases        []domaineval.MultiBackendEvalCaseDelta       `json:"differing_cases,omitempty"`
	WidenedDisagreements  []domaineval.MultiBackendEvalCaseDelta       `json:"widened_disagreements,omitempty"`
	NarrowedDisagreements []domaineval.MultiBackendEvalCaseDelta       `json:"narrowed_disagreements,omitempty"`
	Fingerprint           string                                       `json:"fingerprint,omitempty"`
}

type SkillEvalMultiCompareArtifactRun struct {
	BaseTarget      string `json:"base_target"`
	CandidateTarget string `json:"candidate_target"`
	SuitePath       string `json:"suite_path"`
	ExitCode        int    `json:"exit_code"`
	StdoutFormat    string `json:"stdout_format"`
}

func BuildSkillEvalMultiCompareArtifact(version app.VersionInfo, result service.SkillEvalMultiCompareResult, options SkillEvalMultiCompareArtifactOptions, exitCode int) SkillEvalMultiCompareArtifact {
	artifact := SkillEvalMultiCompareArtifact{
		SchemaVersion: SkillEvalMultiCompareArtifactSchemaVersion,
		ArtifactType:  "firety.skill-routing-eval-compare-multi",
		Tool: SkillLintArtifactTool{
			Name:    "firety",
			Version: version.Version,
			Commit:  version.Commit,
			Date:    version.Date,
		},
		Run: SkillEvalMultiCompareArtifactRun{
			BaseTarget:      result.BaseReport.Target,
			CandidateTarget: result.CandidateReport.Target,
			SuitePath:       result.BaseReport.Suite.Path,
			ExitCode:        exitCode,
			StdoutFormat:    evalMultiCompareOptionsOutputFormatDefault(options),
		},
		Suite:                 result.Comparison.Suite,
		Backends:              result.Comparison.Backends,
		Base:                  result.Comparison.Base,
		Candidate:             result.Comparison.Candidate,
		AggregateSummary:      result.Comparison.AggregateSummary,
		PerBackendDeltas:      result.Comparison.PerBackend,
		DifferingCases:        result.Comparison.DifferingCases,
		WidenedDisagreements:  result.Comparison.WidenedDisagreements,
		NarrowedDisagreements: result.Comparison.NarrowedDisagreements,
	}
	artifact.Fingerprint = skillEvalMultiCompareArtifactFingerprint(artifact)
	return artifact
}

func WriteSkillEvalMultiCompareArtifact(path string, artifact SkillEvalMultiCompareArtifact) error {
	if path == "" {
		return fmt.Errorf("artifact path must not be empty")
	}
	if path == "-" {
		return fmt.Errorf(`artifact path "-" is not supported; choose a file path`)
	}
	data, err := marshalSkillEvalMultiCompareArtifact(artifact)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func marshalSkillEvalMultiCompareArtifact(artifact SkillEvalMultiCompareArtifact) ([]byte, error) {
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func skillEvalMultiCompareArtifactFingerprint(artifact SkillEvalMultiCompareArtifact) string {
	type fingerprintInput struct {
		SchemaVersion         string                                       `json:"schema_version"`
		BaseTarget            string                                       `json:"base_target"`
		CandidateTarget       string                                       `json:"candidate_target"`
		Suite                 domaineval.RoutingEvalSuiteInfo              `json:"suite"`
		Backends              []domaineval.RoutingEvalBackendInfo          `json:"backends"`
		AggregateSummary      domaineval.MultiBackendEvalComparisonSummary `json:"aggregate_summary"`
		DifferingCases        []domaineval.MultiBackendEvalCaseDelta       `json:"differing_cases,omitempty"`
		WidenedDisagreements  []domaineval.MultiBackendEvalCaseDelta       `json:"widened_disagreements,omitempty"`
		NarrowedDisagreements []domaineval.MultiBackendEvalCaseDelta       `json:"narrowed_disagreements,omitempty"`
	}

	payload, _ := json.Marshal(fingerprintInput{
		SchemaVersion:         artifact.SchemaVersion,
		BaseTarget:            artifact.Run.BaseTarget,
		CandidateTarget:       artifact.Run.CandidateTarget,
		Suite:                 artifact.Suite,
		Backends:              artifact.Backends,
		AggregateSummary:      artifact.AggregateSummary,
		DifferingCases:        artifact.DifferingCases,
		WidenedDisagreements:  artifact.WidenedDisagreements,
		NarrowedDisagreements: artifact.NarrowedDisagreements,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func evalMultiCompareOptionsOutputFormatDefault(options SkillEvalMultiCompareArtifactOptions) string {
	if options.Format == "" {
		return "text"
	}
	return options.Format
}
