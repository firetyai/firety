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

const WorkspaceChangeScopeArtifactSchemaVersion = "1"

type WorkspaceChangeScopeArtifactOptions struct {
	Format        string
	WorkspaceRoot string
	BaseRev       string
	HeadRev       string
}

type WorkspaceChangeScopeArtifact struct {
	SchemaVersion string                          `json:"schema_version"`
	ArtifactType  string                          `json:"artifact_type"`
	Tool          SkillLintArtifactTool           `json:"tool"`
	Run           WorkspaceChangeScopeArtifactRun `json:"run"`
	Scope         workspacepkg.ChangeScope        `json:"scope"`
	Fingerprint   string                          `json:"fingerprint,omitempty"`
}

type WorkspaceChangeScopeArtifactRun struct {
	WorkspaceRoot string `json:"workspace_root"`
	BaseRev       string `json:"base_rev,omitempty"`
	HeadRev       string `json:"head_rev,omitempty"`
	StdoutFormat  string `json:"stdout_format"`
}

func BuildWorkspaceChangeScopeArtifact(version app.VersionInfo, scope workspacepkg.ChangeScope, options WorkspaceChangeScopeArtifactOptions) WorkspaceChangeScopeArtifact {
	value := WorkspaceChangeScopeArtifact{
		SchemaVersion: WorkspaceChangeScopeArtifactSchemaVersion,
		ArtifactType:  "firety.workspace-change-scope",
		Tool: SkillLintArtifactTool{
			Name:    "firety",
			Version: version.Version,
			Commit:  version.Commit,
			Date:    version.Date,
		},
		Run: WorkspaceChangeScopeArtifactRun{
			WorkspaceRoot: options.WorkspaceRoot,
			BaseRev:       options.BaseRev,
			HeadRev:       options.HeadRev,
			StdoutFormat:  workspaceChangeScopeOutputFormatDefault(options),
		},
		Scope: scope,
	}
	value.Fingerprint = workspaceChangeScopeArtifactFingerprint(value)
	return value
}

func WriteWorkspaceChangeScopeArtifact(path string, value WorkspaceChangeScopeArtifact) error {
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

func workspaceChangeScopeArtifactFingerprint(value WorkspaceChangeScopeArtifact) string {
	type fingerprintInput struct {
		SchemaVersion string                          `json:"schema_version"`
		Run           WorkspaceChangeScopeArtifactRun `json:"run"`
		Scope         workspacepkg.ChangeScope        `json:"scope"`
	}
	payload, _ := json.Marshal(fingerprintInput{
		SchemaVersion: value.SchemaVersion,
		Run:           value.Run,
		Scope:         value.Scope,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func workspaceChangeScopeOutputFormatDefault(options WorkspaceChangeScopeArtifactOptions) string {
	if options.Format == "" {
		return "text"
	}
	return options.Format
}
