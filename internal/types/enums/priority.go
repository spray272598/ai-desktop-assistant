package enums

type Priority int

const (
	PriorityCritical Priority = 3
	PriorityHigh     Priority = 2
	PriorityMedium   Priority = 1
	PriorityLow      Priority = 0
)

func (p Priority) String() string {
	names := map[Priority]string{
		PriorityCritical: "CRITICAL",
		PriorityHigh:     "HIGH",
		PriorityMedium:   "MEDIUM",
		PriorityLow:      "LOW",
	}
	return names[p]
}
