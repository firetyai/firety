package evalrunner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/firety/firety/internal/domain/eval"
)

type CommandBackend struct {
	commandPath string
}

func NewCommandBackend(commandPath string) (*CommandBackend, error) {
	if commandPath == "" {
		return nil, fmt.Errorf("routing eval runner command must not be empty")
	}
	if _, err := os.Stat(commandPath); err != nil {
		return nil, fmt.Errorf("routing eval runner %q is not available: %w", commandPath, err)
	}

	return &CommandBackend{commandPath: commandPath}, nil
}

func (b *CommandBackend) Name() string {
	return "command:" + filepath.Base(b.commandPath)
}

func (b *CommandBackend) Evaluate(ctx context.Context, request eval.RoutingEvalRequest) (eval.RoutingEvalDecision, error) {
	request.SchemaVersion = eval.RoutingEvalSchemaVersion

	payload, err := json.Marshal(request)
	if err != nil {
		return eval.RoutingEvalDecision{}, fmt.Errorf("marshal routing eval request: %w", err)
	}

	cmd := exec.CommandContext(ctx, b.commandPath)
	cmd.Stdin = bytes.NewReader(payload)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return eval.RoutingEvalDecision{}, fmt.Errorf("runner failed: %w: %s", err, stderr.String())
		}
		return eval.RoutingEvalDecision{}, fmt.Errorf("runner failed: %w", err)
	}

	var decision eval.RoutingEvalDecision
	if err := json.Unmarshal(stdout.Bytes(), &decision); err != nil {
		return eval.RoutingEvalDecision{}, fmt.Errorf("parse runner response: %w", err)
	}
	if decision.SchemaVersion != "" && decision.SchemaVersion != eval.RoutingEvalSchemaVersion {
		return eval.RoutingEvalDecision{}, fmt.Errorf("unsupported runner response schema version %q", decision.SchemaVersion)
	}

	return decision, nil
}
