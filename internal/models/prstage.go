package models

import "slices"

// The stages at which a project opens the draft pull request of its tasks
// (#61, #580), stored in Project.PRCreationStage.
const (
	PRCreationClarified   = "clarified"
	PRCreationSpecified   = "specified"
	PRCreationImplemented = "implemented"
)

// PRCreationStages lists the accepted values of Project.PRCreationStage, in
// workflow order.
var PRCreationStages = []string{PRCreationClarified, PRCreationSpecified, PRCreationImplemented}

// ValidPRCreationStage reports whether a project may store the value. The
// empty string is not one: it means the default on create and is resolved
// before storing.
func ValidPRCreationStage(stage string) bool {
	return slices.Contains(PRCreationStages, stage)
}

// PRCreationOwner names the stage skill that opens the draft pull request for
// a creation stage. The default, and any value this build does not know,
// leaves creation to implementation, which always requires a pull request.
func PRCreationOwner(stage string) string {
	switch stage {
	case PRCreationClarified:
		return "clarify"
	case PRCreationSpecified:
		return "specify"
	}
	return "implement"
}
