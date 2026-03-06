package eval

type BackendDefinition struct {
	ID              string `json:"id"`
	DisplayName     string `json:"display_name"`
	ProfileAffinity string `json:"profile_affinity,omitempty"`
	RunnerEnvVar    string `json:"runner_env_var"`
}

var backendDefinitions = []BackendDefinition{
	{
		ID:              "generic",
		DisplayName:     "Generic",
		ProfileAffinity: "generic",
		RunnerEnvVar:    "FIRETY_SKILL_EVAL_RUNNER_GENERIC",
	},
	{
		ID:              "codex",
		DisplayName:     "Codex",
		ProfileAffinity: "codex",
		RunnerEnvVar:    "FIRETY_SKILL_EVAL_RUNNER_CODEX",
	},
	{
		ID:              "claude-code",
		DisplayName:     "Claude Code",
		ProfileAffinity: "claude-code",
		RunnerEnvVar:    "FIRETY_SKILL_EVAL_RUNNER_CLAUDE_CODE",
	},
	{
		ID:              "copilot",
		DisplayName:     "Copilot",
		ProfileAffinity: "copilot",
		RunnerEnvVar:    "FIRETY_SKILL_EVAL_RUNNER_COPILOT",
	},
	{
		ID:              "cursor",
		DisplayName:     "Cursor",
		ProfileAffinity: "cursor",
		RunnerEnvVar:    "FIRETY_SKILL_EVAL_RUNNER_CURSOR",
	},
}

func AllBackendDefinitions() []BackendDefinition {
	return append([]BackendDefinition(nil), backendDefinitions...)
}

func FindBackendDefinition(id string) (BackendDefinition, bool) {
	for _, backend := range backendDefinitions {
		if backend.ID == id {
			return backend, true
		}
	}

	return BackendDefinition{}, false
}
