package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/swift1337/job-grouper"
)

func main() {
	jobs := jobgrouper.New(time.Second, false, nil)

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
