// Command counter runs goroutines that update a shared counter, protected by
// a mutex. In patina mode the goroutines are planned to become Tokio tasks
// and the counter an Arc<Mutex<i64>>. See EXAMPLES.md.
package main

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// count starts workers goroutines, each adding perWorker to a shared total.
func count(workers, perWorker int) int {
	var (
		mu    sync.Mutex
		wg    sync.WaitGroup
		total int
	)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perWorker; j++ {
				mu.Lock()
				total++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return total
}

func run(w io.Writer) {
	fmt.Fprintln(w, "total:", count(4, 1000))
}

func main() {
	run(os.Stdout)
}
