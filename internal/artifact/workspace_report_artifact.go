package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/firety/firety/internal/app"
	workspacepkg "github.com/firety/firety/internal/domain/workspace"
)

const WorkspaceReportArtifactSchemaVersion = "1"

type WorkspaceReportArtifactOptions struct {
	Format         string
	WorkspaceRoot  string
	Profile        string
	Strictness     string
	PublishContext string
	SuitePath      string
	Runner         string
	Backends       []string
}

type WorkspaceReportArtifact struct {
	SchemaVersion string                     `json:"schema_version"`
	ArtifactType  string                     `json:"artifact_type"`
	Tool          SkillLintArtifactTool      `json:"tool"`
	Run           WorkspaceReportArtifactRun `json:"run"`
	Report        workspacepkg.Report        `json:"report"`
	Fingerprint   string                     `json:"fingerprint,omitempty"`
}

type WorkspaceReportArtifactRun struct {
	WorkspaceRoot  string   `json:"workspace_root"`
	Profile        string   `json:"profile"`
	Strictness     string   `json:"strictness"`
	PublishContext string   `json:"publish_context,omitempty"`
	SuitePath      string   `json:"suite_path,omitempty"`
	Runner         string   `json:"runner,omitempty"`
	Backends       []string `json:"backends,omitempty"`
	StdoutFormat   string   `json:"stdout_format"`
}

func BuildWorkspaceReportArtifact(version app.VersionInfo, report workspacepkg.Report, options WorkspaceReportArtifactOptions) WorkspaceReportArtifact {
	value := WorkspaceReportArtifact{
		SchemaVersion: WorkspaceReportArtifactSchemaVersion,
		ArtifactType:  "firety.workspace-report",
		Tool: SkillLintArtifactTool{
			Name:    "firety",
			Version: version.Version,
			Commit:  version.Commit,
			Date:    version.Date,
		},
		Run: WorkspaceReportArtifactRun{
			WorkspaceRoot:  options.WorkspaceRoot,
			Profile:        options.Profile,
			Strictness:     options.Strictness,
			PublishContext: options.PublishContext,
			SuitePath:      options.SuitePath,
			Runner:         options.Runner,
			Backends:       append([]string(nil), options.Backends...),
			StdoutFormat:   workspaceOptionsOutputFormatDefault(options),
		},
		Report: report,
	}
	value.Fingerprint = workspaceReportArtifactFingerprint(value)
	return value
}

func WriteWorkspaceReportArtifact(path string, value WorkspaceReportArtifact) error {
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

func workspaceReportArtifactFingerprint(value WorkspaceReportArtifact) string {
	type fingerprintInput struct {
		SchemaVersion string                     `json:"schema_version"`
		Run           WorkspaceReportArtifactRun `json:"run"`
		Report        workspacepkg.Report        `json:"report"`
	}
	payload, _ := json.Marshal(fingerprintInput{
		SchemaVersion: value.SchemaVersion,
		Run:           value.Run,
		Report:        value.Report,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func workspaceOptionsOutputFormatDefault(options WorkspaceReportArtifactOptions) string {
	if options.Format == "" {
		return "text"
	}
	return options.Format
}
