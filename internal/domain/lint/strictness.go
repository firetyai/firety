package lint

import (
	"fmt"
	"strings"
)

type Strictness string

const (
	StrictnessDefault  Strictness = "default"
	StrictnessStrict   Strictness = "strict"
	StrictnessPedantic Strictness = "pedantic"
)

func (s Strictness) Validate() error {
	switch s {
	case StrictnessDefault, StrictnessStrict, StrictnessPedantic:
		return nil
	default:
		return fmt.Errorf("invalid strictness %q: must be one of default, strict, pedantic", s)
	}
}

func ParseStrictness(raw string) (Strictness, error) {
	strictness := Strictness(strings.TrimSpace(strings.ToLower(raw)))
	if strictness == "" {
		strictness = StrictnessDefault
	}

	if err := strictness.Validate(); err != nil {
		return "", err
	}

	return strictness, nil
}

func (s Strictness) DisplayName() string {
	switch s {
	case StrictnessStrict:
		return "strict"
	case StrictnessPedantic:
		return "pedantic"
	default:
		return "default"
	}
}
