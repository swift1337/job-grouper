package main

import (
	"context"
	"fmt"
	"time"

	"github.com/swift1337/job-grouper"
)

// Job grouper can handle panics
func main() {
	jobs := jobgrouper.New(time.Second, false, nil)

	jobs.Add("Say hi", func() error {
		fmt.Println("Hi!")
		return nil
	})

	jobs.Add("Oops", func() error {
		panic("Oops")
		return nil
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

	// Hi!
	// execution failure: error during job processing
	// job 'Say hi' has status 'finished' (err: '<nil>')
	// job 'Oops' has status 'panicked' (err: 'Oops')
}
