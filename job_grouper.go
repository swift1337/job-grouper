package jobgrouper

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type JobGrouper struct {
	errMu        sync.RWMutex
	jobsMu       sync.RWMutex
	maxTimeout   time.Duration
	stopOnError  uint32
	jobErr       error
	workIsLocked uint32
	jobs         map[string]jobItem
	errHandler   func(err error)
}

type jobItem struct {
	job    func() error
	name   string
	status JobStatus
	err    error
}

var ErrLocked = errors.New("jobGrouper is locked by other Work() call")
var ErrFailed = errors.New("error during job processing")
var ErrContextTimeout = errors.New("context timeout")

// New Create new JobGrouper.
// maxTimeout specified total jobs execution time
// when stopOnError is true, any failed job will abort other running jobs
// errHandler is triggered if any job failed (optional)
func New(maxTimeout time.Duration, stopOnError bool, errHandler func(err error)) *JobGrouper {
	stopOnErr := uint32(0)
	if stopOnError {
		stopOnErr = 1
	}

	return &JobGrouper{
		maxTimeout:  maxTimeout,
		errHandler:  errHandler,
		stopOnError: stopOnErr,
		jobs:        make(map[string]jobItem),
	}
}

// Add adds job to group by key. This method is not concurrent safe.
// Nil callback is skipped
func (group *JobGrouper) Add(name string, job func() error) {
	if job == nil {
		return
	}

	group.jobs[name] = jobItem{
		status: Pending,
		name:   name,
		job:    job,
	}
}

// Work blocks current goroutine while jobs are processing.
// Calling function concurrent will lead to ErrLocked error
func (group *JobGrouper) Work(ctx context.Context) (map[string]JobResult, error) {
	results := make(map[string]JobResult)
	err := group.acquire(func() error {
		// set max timeout
		ctx, cancel := context.WithTimeout(ctx, group.maxTimeout)
		defer cancel()

		group.processJobs(ctx)

		for name, job := range group.jobs {
			results[name] = JobResult{
				Error:  job.err,
				Status: job.status,
			}
		}

		return group.jobErr
	})

	return results, err
}

func (group *JobGrouper) processJobs(ctx context.Context) {
	defer group.abortIncompleteJobs()

	jobsToBeDone := len(group.jobs)
	ch := make(chan jobItem, jobsToBeDone)
	runnable := make([]jobItem, 0, jobsToBeDone)

	// todo
	for _, job := range group.jobs {
		runnable = append(runnable, job)
	}

	// todo
	for _, job := range runnable {
		go run(group, job, ch)
	}

	for i := 0; i < jobsToBeDone; i++ {
		select {
		case job := <-ch:
			group.setJob(job)
			if job.err != nil {
				group.setError(ErrFailed, job.err)
			}
			if group.shouldAbort() {
				return
			}
		case <-ctx.Done():
			group.setError(ErrContextTimeout, ctx.Err())
			return
		}
	}
}

func run(g *JobGrouper, job jobItem, ch chan<- jobItem) {
	// handle panic
	defer func() {
		rec := recover()

		if rec == nil {
			return
		}

		err, ok := rec.(error)
		if !ok {
			err = fmt.Errorf("%v", rec)
		}

		job.status = Panicked
		job.err = err

		ch <- job
	}()

	// don't run job if another job failed
	if g.shouldAbort() {
		job.status = Aborted
		ch <- job
		return
	}

	// process job
	job.status = Running
	g.setJob(job)

	err := job.job()

	if err != nil {
		job.status = Error
		job.err = err

		ch <- job
		return
	}

	job.status = Finished

	ch <- job
}

// Lock group to process jobs
func (group *JobGrouper) acquire(callback func() error) error {
	if !atomic.CompareAndSwapUint32(&group.workIsLocked, 0, 1) {
		return ErrLocked
	}

	defer atomic.StoreUint32(&group.workIsLocked, 0)

	return callback()
}

func (group *JobGrouper) getJob(name string) jobItem {
	group.jobsMu.RLock()
	defer group.jobsMu.RUnlock()

	return group.jobs[name]
}

func (group *JobGrouper) setJob(job jobItem) {
	group.jobsMu.Lock()
	defer group.jobsMu.Unlock()

	group.jobs[job.name] = job
}

func (group *JobGrouper) hasError() bool {
	group.errMu.RLock()
	defer group.errMu.RUnlock()

	return group.jobErr != nil
}

func (group *JobGrouper) shouldAbort() bool {
	stopOnError := atomic.LoadUint32(&group.stopOnError)
	return group.hasError() && stopOnError == 1
}

func (group *JobGrouper) setError(predefinedError error, realError error) {
	group.errMu.Lock()
	if group.jobErr == nil {
		group.jobErr = predefinedError
	}
	group.errMu.Unlock()

	if group.errHandler != nil {
		group.errHandler(realError)
	}
}

// This method should be called if there's no concurrent access to jobs
func (group *JobGrouper) abortIncompleteJobs() {
	group.jobsMu.Lock()
	defer group.jobsMu.Unlock()

	for name, job := range group.jobs {
		if job.status == Pending || job.status == Running {
			job.status = Aborted
			group.jobs[name] = job
		}
	}
}
