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

const SkillEvalCompareArtifactSchemaVersion = "1"

type SkillEvalCompareArtifactOptions struct {
	Format  string
	Profile string
	Suite   string
	Runner  string
}

type SkillEvalCompareArtifact struct {
	SchemaVersion   string                                  `json:"schema_version"`
	ArtifactType    string                                  `json:"artifact_type"`
	Tool            SkillLintArtifactTool                   `json:"tool"`
	Run             SkillEvalCompareArtifactRun             `json:"run"`
	Suite           domaineval.RoutingEvalSuiteInfo         `json:"suite"`
	Backend         domaineval.RoutingEvalBackendInfo       `json:"backend"`
	Base            domaineval.RoutingEvalSideSummary       `json:"base"`
	Candidate       domaineval.RoutingEvalSideSummary       `json:"candidate"`
	Comparison      domaineval.RoutingEvalComparisonSummary `json:"comparison"`
	FlippedToFail   []domaineval.RoutingEvalCaseChange      `json:"flipped_to_fail,omitempty"`
	FlippedToPass   []domaineval.RoutingEvalCaseChange      `json:"flipped_to_pass,omitempty"`
	ChangedCases    []domaineval.RoutingEvalCaseChange      `json:"changed_cases,omitempty"`
	ByProfileDeltas []domaineval.RoutingEvalBreakdownDelta  `json:"by_profile_deltas,omitempty"`
	ByTagDeltas     []domaineval.RoutingEvalBreakdownDelta  `json:"by_tag_deltas,omitempty"`
	Fingerprint     string                                  `json:"fingerprint,omitempty"`
}

type SkillEvalCompareArtifactRun struct {
	BaseTarget      string `json:"base_target"`
	CandidateTarget string `json:"candidate_target"`
	Profile         string `json:"profile"`
	SuitePath       string `json:"suite_path"`
	Runner          string `json:"runner,omitempty"`
	ExitCode        int    `json:"exit_code"`
	StdoutFormat    string `json:"stdout_format"`
}

func BuildSkillEvalCompareArtifact(version app.VersionInfo, result service.SkillEvalCompareResult, options SkillEvalCompareArtifactOptions, exitCode int) SkillEvalCompareArtifact {
	artifact := SkillEvalCompareArtifact{
		SchemaVersion: SkillEvalCompareArtifactSchemaVersion,
		ArtifactType:  "firety.skill-routing-eval-compare",
		Tool: SkillLintArtifactTool{
			Name:    "firety",
			Version: version.Version,
			Commit:  version.Commit,
			Date:    version.Date,
		},
		Run: SkillEvalCompareArtifactRun{
			BaseTarget:      result.BaseReport.Target,
			CandidateTarget: result.CandidateReport.Target,
			Profile:         options.Profile,
			SuitePath:       result.BaseReport.Suite.Path,
			Runner:          options.Runner,
			ExitCode:        exitCode,
			StdoutFormat:    evalCompareOptionsOutputFormatDefault(options),
		},
		Suite:           result.Comparison.Suite,
		Backend:         result.Comparison.Backend,
		Base:            result.Comparison.Base,
		Candidate:       result.Comparison.Candidate,
		Comparison:      result.Comparison.Summary,
		FlippedToFail:   result.Comparison.FlippedToFail,
		FlippedToPass:   result.Comparison.FlippedToPass,
		ChangedCases:    result.Comparison.ChangedCases,
		ByProfileDeltas: result.Comparison.ByProfileDeltas,
		ByTagDeltas:     result.Comparison.ByTagDeltas,
	}
	artifact.Fingerprint = skillEvalCompareArtifactFingerprint(artifact)
	return artifact
}

func WriteSkillEvalCompareArtifact(path string, artifact SkillEvalCompareArtifact) error {
	if path == "" {
		return fmt.Errorf("artifact path must not be empty")
	}
	if path == "-" {
		return fmt.Errorf(`artifact path "-" is not supported; choose a file path`)
	}

	data, err := marshalSkillEvalCompareArtifact(artifact)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

func marshalSkillEvalCompareArtifact(artifact SkillEvalCompareArtifact) ([]byte, error) {
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func skillEvalCompareArtifactFingerprint(artifact SkillEvalCompareArtifact) string {
	type fingerprintInput struct {
		SchemaVersion   string                                  `json:"schema_version"`
		BaseTarget      string                                  `json:"base_target"`
		CandidateTarget string                                  `json:"candidate_target"`
		Profile         string                                  `json:"profile"`
		SuitePath       string                                  `json:"suite_path"`
		Backend         string                                  `json:"backend"`
		Comparison      domaineval.RoutingEvalComparisonSummary `json:"comparison"`
		FlippedToFail   []domaineval.RoutingEvalCaseChange      `json:"flipped_to_fail,omitempty"`
		FlippedToPass   []domaineval.RoutingEvalCaseChange      `json:"flipped_to_pass,omitempty"`
		ChangedCases    []domaineval.RoutingEvalCaseChange      `json:"changed_cases,omitempty"`
	}

	payload, _ := json.Marshal(fingerprintInput{
		SchemaVersion:   artifact.SchemaVersion,
		BaseTarget:      artifact.Run.BaseTarget,
		CandidateTarget: artifact.Run.CandidateTarget,
		Profile:         artifact.Run.Profile,
		SuitePath:       artifact.Run.SuitePath,
		Backend:         artifact.Backend.Name,
		Comparison:      artifact.Comparison,
		FlippedToFail:   artifact.FlippedToFail,
		FlippedToPass:   artifact.FlippedToPass,
		ChangedCases:    artifact.ChangedCases,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func evalCompareOptionsOutputFormatDefault(options SkillEvalCompareArtifactOptions) string {
	if options.Format == "" {
		return "text"
	}
	return options.Format
}
