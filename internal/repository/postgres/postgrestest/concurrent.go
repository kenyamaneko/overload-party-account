package postgrestest

import "sync"

// RunConcurrently は fn を count 個、すべての準備が整ってから同時に開始し、すべての終了を待って、index 順の結果を返す。
func RunConcurrently[T any](count int, fn func(index int) T) []T {
	results := make([]T, count)
	var ready, done sync.WaitGroup
	start := make(chan struct{})
	ready.Add(count)
	done.Add(count)
	for i := range count {
		go func() {
			defer done.Done()
			ready.Done()
			<-start
			results[i] = fn(i)
		}()
	}
	ready.Wait()
	close(start)
	done.Wait()
	return results
}
