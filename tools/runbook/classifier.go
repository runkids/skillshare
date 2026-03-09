package main

// Classify determines execution mode for a step.
func Classify(s Step) string {
	if s.Command == "" {
		return ExecutorManual
	}
	switch s.Lang {
	case "bash", "sh", "":
		return ExecutorAuto
	default:
		return ExecutorManual
	}
}

// ClassifyAll batch classifies all steps.
func ClassifyAll(steps []Step) []Step {
	for i := range steps {
		steps[i].Executor = Classify(steps[i])
	}
	return steps
}
