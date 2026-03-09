package main

// Classify determines execution mode for a step.
func Classify(s Step) string {
	if s.Command == "" {
		return "manual"
	}
	switch s.Lang {
	case "bash", "sh", "":
		return "auto"
	default:
		return "manual"
	}
}

// ClassifyAll batch classifies all steps.
func ClassifyAll(steps []Step) []Step {
	for i := range steps {
		steps[i].Executor = Classify(steps[i])
	}
	return steps
}
