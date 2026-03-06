package capability

type Kind string

const (
	KindSkill Kind = "skill"
	KindMCP   Kind = "mcp"
	KindAgent Kind = "agent"
)

func (k Kind) String() string {
	return string(k)
}
