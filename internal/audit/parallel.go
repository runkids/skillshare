package audit

import (
	"runtime"
	"sync"
	"time"
)

// workerCount returns a bounded worker count based on available CPUs.
// Floor: 2 (always some parallelism). Ceiling: 32 (avoid fd exhaustion).
func workerCount() int {
	n := runtime.NumCPU()
	if n < 2 {
		return 2
	}
	if n > 32 {
		return 32
	}
	return n
}

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
// onDone is called after each skill finishes (nil-safe); use it to drive a progress bar.
// Returns []ScanOutput aligned by index with the input slice.
func ParallelScan(skills []SkillInput, projectRoot string, onDone func()) []ScanOutput {
	outputs := make([]ScanOutput, len(skills))
	if len(skills) == 0 {
		return outputs
	}

	sem := make(chan struct{}, workerCount())
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
			if onDone != nil {
				onDone()
			}
		}(i, sk.Path)
	}
	wg.Wait()

	return outputs
}
