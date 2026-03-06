package service_test

import (
	"testing"

	"github.com/firety/firety/internal/domain/capability"
	"github.com/firety/firety/internal/service"
)

func TestPlaceholderServiceMessage(t *testing.T) {
	t.Parallel()

	svc := service.NewPlaceholderService()

	testCases := []struct {
		name     string
		kind     capability.Kind
		expected string
	}{
		{
			name:     "skill",
			kind:     capability.KindSkill,
			expected: "skill command is scaffolded but not implemented yet",
		},
		{
			name:     "mcp",
			kind:     capability.KindMCP,
			expected: "mcp command is scaffolded but not implemented yet",
		},
		{
			name:     "agent",
			kind:     capability.KindAgent,
			expected: "agent command is scaffolded but not implemented yet",
		},
		{
			name:     "unknown",
			kind:     capability.Kind("custom"),
			expected: "custom command is scaffolded but not implemented yet",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := svc.Message(tc.kind); got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}
