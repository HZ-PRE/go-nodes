package services

import (
	"sync"

	"nodes/utils"
)

// probeAll bounds both active probes and goroutines, preserving input order.
func probeAll[T, R any](items []T, probe func(T) R) []R {
	results := make([]R, len(items))
	workers := min(utils.ProbeWorkers(), len(items))
	if workers == 0 {
		return results
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for i := range jobs {
				results[i] = probe(items[i])
			}
		})
	}
	for i := range items {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return results
}
