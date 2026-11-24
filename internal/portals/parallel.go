package portals

import "sync"

// maxParallel is the number of requests sent at once. The Management API limits the rate of
// requests, and a handful at a time is fast enough.
const maxParallel = 4

// forEachParallel calls fn for every index below count, with at most maxParallel calls running at
// once, and returns when all of them are done. Each call writes only to its own index of the results.
func forEachParallel(count int, fn func(index int)) {
	var group sync.WaitGroup

	slots := make(chan struct{}, maxParallel)

	for index := range count {
		slots <- struct{}{}

		group.Add(1)

		go func() {
			defer group.Done()
			defer func() { <-slots }()

			fn(index)
		}()
	}

	group.Wait()
}
