# Job Grouper

A small package for async tasks. It's like WaitGroup, but with callbacks. Features:
- Ease of use
- Panic handling built-in
- Each job has its own error response

### See examples:
- [ping pong](examples/pingpong.go)
- [sample](examples/sample.go)
- [panic](examples/panic.go)

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/swift1337/job-grouper"
)

func main() { 
	// API if very simple and consists of 3 methods:
	// jg := New(...)
	// jg.Add(name, callback)
	// jg.Work(ctx)
	
	jobs := jobgrouper.New(
		time.Second, // max timeout
		false,       // abort all jobs if one failed? 
		nil,         // optional error callback
    )

	jobs.Add("Make DB Call", func() error {
		time.Sleep(time.Second)

		return nil
	})

	jobs.Add("Request third-party service A", func() error {
		time.Sleep(time.Millisecond * 500)

		return nil
	})

	jobs.Add("Request third-party service B", func() error {
		time.Sleep(time.Millisecond * 500)

		return nil
	})

	jobs.Add("Request unavailable service C", func() error {
		time.Sleep(time.Second * 5)
		return nil
	})

	jobs.Add("I will fail", func() error {
		return errors.New("got error as expected")
	})

	results, err := jobs.Work(context.Background())

	if err != nil {
		fmt.Println("execution failure: " + err.Error())
	}

	for name, jobResult := range results {
		fmt.Printf(
			"job '%s' has status '%s' (err: '%v')\n",
			name,
			jobResult.StatusName(),
			jobResult.Error,
		)
	}

	// execution failure: error during job processing
	// job 'Request unavailable service C' has status 'aborted' (err: '<nil>')
	// job 'I will fail' has status 'error' (err: 'got error as expected')
	// job 'Make DB Call' has status 'finished' (err: '<nil>')
	// job 'Request third-party service A' has status 'finished' (err: '<nil>')
	// job 'Request third-party service B' has status 'finished' (err: '<nil>')
}
```