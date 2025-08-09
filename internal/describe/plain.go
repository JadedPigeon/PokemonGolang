package describe

import (
	"context"
	"fmt"
	"strings"
)

// Plain is a deterministic, zero-dependency fallback.
type Plain struct{}

func (Plain) DescribeAction(ctx context.Context, a ActionContext) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "%s used %s on %s", a.Source.Name, a.Move.Name, a.Target.Name)

	// Light flavor if hints are present (no numbers, no mechanics).
	switch {
	case a.Effectiveness != "":
		fmt.Fprintf(&b, ". It was %s!", a.Effectiveness)
	default:
		b.WriteString(".")
	}

	return b.String(), nil
}
