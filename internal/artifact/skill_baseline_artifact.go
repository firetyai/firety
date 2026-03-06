package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/domain/baseline"
)

const (
	SkillBaselineSnapshotArtifactSchemaVersion = "1"
	SkillBaselineCompareArtifactSchemaVersion  = "1"
)

type SkillBaselineSnapshotArtifact struct {
	SchemaVersion string                `json:"schema_version"`
	ArtifactType  string                `json:"artifact_type"`
	Tool          SkillLintArtifactTool `json:"tool"`
	Snapshot      baseline.Snapshot     `json:"snapshot"`
	Fingerprint   string                `json:"fingerprint,omitempty"`
}

type SkillBaselineCompareArtifactOptions struct {
	Format       string
	BaselinePath string
}

type SkillBaselineCompareArtifact struct {
	SchemaVersion string                  `json:"schema_version"`
	ArtifactType  string                  `json:"artifact_type"`
	Tool          SkillLintArtifactTool   `json:"tool"`
	Run           SkillBaselineCompareRun `json:"run"`
	Comparison    baseline.Comparison     `json:"comparison"`
	Fingerprint   string                  `json:"fingerprint,omitempty"`
}

type SkillBaselineCompareRun struct {
	BaselinePath  string `json:"baseline_path"`
	CurrentTarget string `json:"current_target"`
	ExitCode      int    `json:"exit_code"`
	StdoutFormat  string `json:"stdout_format"`
}

func BuildSkillBaselineSnapshotArtifact(version app.VersionInfo, snapshot baseline.Snapshot) SkillBaselineSnapshotArtifact {
	artifact := SkillBaselineSnapshotArtifact{
		SchemaVersion: SkillBaselineSnapshotArtifactSchemaVersion,
		ArtifactType:  "firety.skill-baseline",
		Tool: SkillLintArtifactTool{
			Name:    "firety",
			Version: version.Version,
			Commit:  version.Commit,
			Date:    version.Date,
		},
		Snapshot: snapshot,
	}
	artifact.Fingerprint = skillBaselineSnapshotArtifactFingerprint(artifact)
	return artifact
}

func WriteSkillBaselineSnapshotArtifact(path string, artifact SkillBaselineSnapshotArtifact) error {
	return writeSkillBaselineArtifact(path, artifact)
}

func BuildSkillBaselineCompareArtifact(version app.VersionInfo, comparison baseline.Comparison, options SkillBaselineCompareArtifactOptions, exitCode int) SkillBaselineCompareArtifact {
	artifact := SkillBaselineCompareArtifact{
		SchemaVersion: SkillBaselineCompareArtifactSchemaVersion,
		ArtifactType:  "firety.skill-baseline-compare",
		Tool: SkillLintArtifactTool{
			Name:    "firety",
			Version: version.Version,
			Commit:  version.Commit,
			Date:    version.Date,
		},
		Run: SkillBaselineCompareRun{
			BaselinePath:  options.BaselinePath,
			CurrentTarget: comparison.CurrentTarget,
			ExitCode:      exitCode,
			StdoutFormat:  baselineCompareOutputFormatDefault(options),
		},
		Comparison: comparison,
	}
	artifact.Fingerprint = skillBaselineCompareArtifactFingerprint(artifact)
	return artifact
}

func WriteSkillBaselineCompareArtifact(path string, artifact SkillBaselineCompareArtifact) error {
	return writeSkillBaselineArtifact(path, artifact)
}

func writeSkillBaselineArtifact(path string, artifact any) error {
	if path == "" {
		return fmt.Errorf("artifact path must not be empty")
	}
	if path == "-" {
		return fmt.Errorf(`artifact path "-" is not supported; choose a file path`)
	}
	data, err := marshalSkillBaselineArtifact(artifact)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func marshalSkillBaselineArtifact(artifact any) ([]byte, error) {
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func skillBaselineSnapshotArtifactFingerprint(artifact SkillBaselineSnapshotArtifact) string {
	type fingerprintInput struct {
		SchemaVersion string            `json:"schema_version"`
		Snapshot      baseline.Snapshot `json:"snapshot"`
	}
	payload, _ := json.Marshal(fingerprintInput{
		SchemaVersion: artifact.SchemaVersion,
		Snapshot:      artifact.Snapshot,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func skillBaselineCompareArtifactFingerprint(artifact SkillBaselineCompareArtifact) string {
	type fingerprintInput struct {
		SchemaVersion string              `json:"schema_version"`
		BaselinePath  string              `json:"baseline_path"`
		Comparison    baseline.Comparison `json:"comparison"`
	}
	payload, _ := json.Marshal(fingerprintInput{
		SchemaVersion: artifact.SchemaVersion,
		BaselinePath:  artifact.Run.BaselinePath,
		Comparison:    artifact.Comparison,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func baselineCompareOutputFormatDefault(options SkillBaselineCompareArtifactOptions) string {
	if options.Format == "" {
		return "text"
	}
	return options.Format
}
