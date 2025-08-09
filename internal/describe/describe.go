package describe

import "context"

// ActionContext = one move being used by one Pokémon on another.
type ActionContext struct {
	Source struct {
		Name  string
		Types []string
	}
	Target struct {
		Name  string
		Types []string
	}
	Move struct {
		ID          int32
		Name        string
		Type        string
		Power       int32
		Description string // From the PokeAPI
	}

	// Optional hints that could be used later if we implement more game logic
	//Missed        bool
	//Crit          bool
	Effectiveness string // "super-effective", "not very effective", "no effect", ""
	//StatHint string // e.g., "lowers the target's Speed"
}

type Describer interface {
	DescribeAction(ctx context.Context, a ActionContext) (string, error)
}
