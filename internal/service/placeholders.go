package service

import (
	"fmt"

	"github.com/firety/firety/internal/domain/capability"
)

type PlaceholderService struct{}

func NewPlaceholderService() PlaceholderService {
	return PlaceholderService{}
}

func (PlaceholderService) Message(kind capability.Kind) string {
	switch kind {
	case capability.KindSkill:
		return "skill command is scaffolded but not implemented yet"
	case capability.KindMCP:
		return "mcp command is scaffolded but not implemented yet"
	case capability.KindAgent:
		return "agent command is scaffolded but not implemented yet"
	default:
		return fmt.Sprintf("%s command is scaffolded but not implemented yet", kind)
	}
}
