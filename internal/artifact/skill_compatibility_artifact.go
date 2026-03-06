package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/firety/firety/internal/app"
	"github.com/firety/firety/internal/domain/compatibility"
)

const SkillCompatibilityArtifactSchemaVersion = "1"

type SkillCompatibilityArtifactOptions struct {
	Format         string
	Target         string
	Profiles       []string
	Strictness     string
	SuitePath      string
	Backends       []string
	InputArtifacts []string
}

type SkillCompatibilityArtifact struct {
	SchemaVersion string                        `json:"schema_version"`
	ArtifactType  string                        `json:"artifact_type"`
	Tool          SkillLintArtifactTool         `json:"tool"`
	Run           SkillCompatibilityArtifactRun `json:"run"`
	Report        compatibility.Report          `json:"report"`
	Fingerprint   string                        `json:"fingerprint,omitempty"`
}

type SkillCompatibilityArtifactRun struct {
	Target         string   `json:"target,omitempty"`
	Profiles       []string `json:"profiles,omitempty"`
	Strictness     string   `json:"strictness"`
	SuitePath      string   `json:"suite_path,omitempty"`
	Backends       []string `json:"backends,omitempty"`
	InputArtifacts []string `json:"input_artifacts,omitempty"`
	StdoutFormat   string   `json:"stdout_format"`
}

func BuildSkillCompatibilityArtifact(version app.VersionInfo, report compatibility.Report, options SkillCompatibilityArtifactOptions) SkillCompatibilityArtifact {
	value := SkillCompatibilityArtifact{
		SchemaVersion: SkillCompatibilityArtifactSchemaVersion,
		ArtifactType:  "firety.skill-compatibility",
		Tool: SkillLintArtifactTool{
			Name:    "firety",
			Version: version.Version,
			Commit:  version.Commit,
			Date:    version.Date,
		},
		Run: SkillCompatibilityArtifactRun{
			Target:         options.Target,
			Profiles:       append([]string(nil), options.Profiles...),
			Strictness:     options.Strictness,
			SuitePath:      options.SuitePath,
			Backends:       append([]string(nil), options.Backends...),
			InputArtifacts: append([]string(nil), options.InputArtifacts...),
			StdoutFormat:   compatibilityOptionsOutputFormatDefault(options),
		},
		Report: report,
	}
	value.Fingerprint = skillCompatibilityArtifactFingerprint(value)
	return value
}

func WriteSkillCompatibilityArtifact(path string, value SkillCompatibilityArtifact) error {
	if path == "" {
		return fmt.Errorf("artifact path must not be empty")
	}
	if path == "-" {
		return fmt.Errorf(`artifact path "-" is not supported; choose a file path`)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func skillCompatibilityArtifactFingerprint(value SkillCompatibilityArtifact) string {
	type fingerprintInput struct {
		SchemaVersion string                        `json:"schema_version"`
		Run           SkillCompatibilityArtifactRun `json:"run"`
		Report        compatibility.Report          `json:"report"`
	}
	payload, _ := json.Marshal(fingerprintInput{
		SchemaVersion: value.SchemaVersion,
		Run:           value.Run,
		Report:        value.Report,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func compatibilityOptionsOutputFormatDefault(options SkillCompatibilityArtifactOptions) string {
	if options.Format == "" {
		return "text"
	}
	return options.Format
}
