package audit

import (
	"sync"
	"time"
)

const maxParallelWorkers = 8

// SkillInput describes a skill to scan.
type SkillInput struct {
	Name string
	Path string
}

// ScanOutput holds the result of scanning a single skill.
type ScanOutput struct {
	Result  *Result
	Err     error
	Elapsed time.Duration
}

// ParallelScan scans skills concurrently with bounded workers.
// projectRoot being empty means global mode; non-empty means project mode.
// Returns []ScanOutput aligned by index with the input slice.
func ParallelScan(skills []SkillInput, projectRoot string) []ScanOutput {
	outputs := make([]ScanOutput, len(skills))
	if len(skills) == 0 {
		return outputs
	}

	sem := make(chan struct{}, maxParallelWorkers)
	var wg sync.WaitGroup

	for i, sk := range skills {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, path string) {
			defer wg.Done()
			defer func() { <-sem }()
			start := time.Now()
			var res *Result
			var err error
			if projectRoot != "" {
				res, err = ScanSkillForProject(path, projectRoot)
			} else {
				res, err = ScanSkill(path)
			}
			outputs[idx] = ScanOutput{
				Result:  res,
				Err:     err,
				Elapsed: time.Since(start),
			}
		}(i, sk.Path)
	}
	wg.Wait()

	return outputs
}
