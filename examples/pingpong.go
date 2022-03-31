package main

import (
	"context"
	"fmt"
	"time"

	"github.com/swift1337/job-grouper"
)

func main() {
	jg := jobgrouper.New(time.Second, true, nil)

	jg.Add("Ping", func() error {
		for i := 0; i < 5; i++ {
			fmt.Println("Ping", i+1)
		}

		return nil
	})

	jg.Add("Pong", func() error {
		for i := 0; i < 5; i++ {
			fmt.Println("Pong", i+1)
		}

		return nil
	})

	results, err := jg.Work(context.Background())

	fmt.Println("Done")
	fmt.Printf("%+v (err: %+v)", results, err)
}
