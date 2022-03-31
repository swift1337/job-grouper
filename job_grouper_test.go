package jobgrouper

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const testSleepFor = time.Millisecond * 10

func setupGrouper(errHandler func(err error)) *JobGrouper {
	return New(time.Millisecond*300, true, errHandler)
}

func TestJobGrouper_Work(t *testing.T) {
	var one = int64(1)

	bgCtx := context.Background()

	t.Run("NoJobs", func(t *testing.T) {
		_, err := setupGrouper(nil).Work(bgCtx)

		assert.NoError(t, err)
	})

	t.Run("OneJob", func(t *testing.T) {
		jobs := setupGrouper(nil)
		var counter int64

		jobs.Add("A", func() error {
			atomic.AddInt64(&counter, 1)
			return nil
		})

		results, err := jobs.Work(bgCtx)

		assert.NoError(t, err)
		assert.Equal(t, counter, one)
		assert.Equal(t, results["A"].Status, Finished)
	})

	t.Run("TwoJobs_ConcurrencyCheck", func(t *testing.T) {
		jobs := setupGrouper(nil)
		var mu sync.Mutex
		var testValue string

		jobs.Add("First", func() error {
			time.Sleep(testSleepFor)

			mu.Lock()
			if testValue == "" {
				testValue = "first"
			}
			mu.Unlock()

			return nil
		})

		jobs.Add("Second", func() error {
			mu.Lock()
			if testValue == "" {
				testValue = "second"
			}
			mu.Unlock()

			return nil
		})

		results, err := jobs.Work(bgCtx)

		assert.NoError(t, err)
		assert.Equal(t, testValue, "second")

		for _, r := range results {
			assert.Equal(t, r.Status, Finished)
		}
	})

	t.Run("TwoJobs_ErrorHandled", func(t *testing.T) {
		jobs := setupGrouper(nil)
		var counter, i int64

		// first job should not start at all
		for i = 1; i <= 10; i++ {
			jobs.Add(fmt.Sprintf("%d", i), func() error {
				time.Sleep(testSleepFor)
				atomic.AddInt64(&counter, one)
				return nil
			})
		}

		jobs.Add("ErrorJob", func() error {
			return errors.New("ERROR")
		})

		_, err := jobs.Work(bgCtx)

		assert.Error(t, err)
		assert.NotEqual(t, atomic.LoadInt64(&counter), i)
	})

	t.Run("MultipleRuns", func(t *testing.T) {
		jobs := setupGrouper(nil)
		var counter int64

		jobs.Add("A", func() error {
			atomic.AddInt64(&counter, one)
			return nil
		})

		jobs.Add("B", func() error {
			atomic.AddInt64(&counter, one*2)
			return nil
		})

		for i := 0; i < 10; i++ {
			results, err := jobs.Work(bgCtx)
			assert.NoError(t, err)
			for _, v := range results {
				assert.Equal(t, Finished, v.Status)
			}
		}

		assert.Equal(t, counter, one*30)
	})

	t.Run("WorkLocked", func(t *testing.T) {
		concurrentGroupWorkers := 3
		jobs := setupGrouper(nil)
		jobs.Add("A", func() error {
			time.Sleep(testSleepFor)
			return nil
		})

		var resultingErrors []error
		var mu sync.Mutex

		wg := sync.WaitGroup{}
		wg.Add(concurrentGroupWorkers)

		for i := 0; i < concurrentGroupWorkers; i++ {
			go func(j int) {
				defer wg.Done()

				_, err := jobs.Work(bgCtx)

				mu.Lock()
				defer mu.Unlock()
				{
					resultingErrors = append(resultingErrors, err)
				}
			}(i)
		}

		wg.Wait()

		assert.Contains(t, resultingErrors, ErrLocked)
	})

	t.Run("ErrorHandlerTriggered", func(t *testing.T) {
		var sampleInt uint32

		jobs := setupGrouper(func(err error) {
			atomic.StoreUint32(&sampleInt, 1)
			assert.Equal(t, err.Error(), "sample err")
		})

		jobs.Add("A", func() error {
			time.Sleep(testSleepFor)
			return errors.New("sample err")
		})

		results, err := jobs.Work(bgCtx)

		assert.ErrorIs(t, err, ErrFailed)
		assert.Equal(t, sampleInt, uint32(1))
		assert.Equal(t, results["A"].Status, Error)
	})

	t.Run("PanicHandled", func(t *testing.T) {
		var secondCompleted int64

		jobs := setupGrouper(func(err error) {
			assert.Equal(t, err.Error(), "invalid argument to Intn")
		})

		jobs.Add("I Will Panic", func() error {
			time.Sleep(testSleepFor)

			return fmt.Errorf("%d", rand.Intn(0))
		})

		jobs.Add("I Won't Panic", func() error {
			atomic.AddInt64(&secondCompleted, one)
			return nil
		})

		results, err := jobs.Work(bgCtx)

		assert.Equal(t, secondCompleted, one)
		assert.Equal(t, err, ErrFailed)
		assert.Equal(t, results["I Will Panic"].Status, Panicked)
	})

	t.Run("ContextTimeout", func(t *testing.T) {
		jobs := setupGrouper(nil)
		shouldBeZero := uint32(0)

		jobs.Add("A", func() error {
			time.Sleep(time.Millisecond * 4000)
			atomic.StoreUint32(&shouldBeZero, 1)
			return nil
		})

		jobs.Add("B", func() error {
			return nil
		})

		jobs.Add("C", func() error {
			return nil
		})

		_, err := jobs.Work(bgCtx)

		assert.Equal(t, err, ErrContextTimeout)
	})

	t.Run("AbortHandled", func(t *testing.T) {
		jobs := setupGrouper(nil)

		jobs.Add("A", func() error {
			time.Sleep(testSleepFor)
			return errors.New("sample error")
		})

		jobs.Add("B", func() error {
			return nil
		})

		jobs.Add("C", func() error {
			time.Sleep(testSleepFor * 9999)
			return nil
		})

		results, err := jobs.Work(bgCtx)

		for name, r := range results {
			if name == "A" {
				assert.Equal(t, r.Status, Error)
			}

			if name == "B" {
				assert.Equal(t, r.Status, Finished)
			}

			if name == "C" {
				assert.Equal(t, r.Status, Aborted)
			}
		}

		assert.ErrorIs(t, err, ErrFailed)
	})

	t.Run("TestConcurrencyRealWorks_:harold:", func(t *testing.T) {
		jobs := setupGrouper(nil)

		dummySleep := func() error {
			time.Sleep(testSleepFor)
			return nil
		}

		jobs.Add("A", dummySleep)
		jobs.Add("B", dummySleep)
		jobs.Add("C", dummySleep)

		start := time.Now()
		results, err := jobs.Work(bgCtx)

		concurrentDuration := time.Now().Sub(start)
		sequentialDuration := testSleepFor * 3

		assert.NoError(t, err)
		assert.Less(t, concurrentDuration, sequentialDuration)

		for _, r := range results {
			assert.Equal(t, r.Status, Finished)
		}
	})

	buildCallback := func(withError bool) func() error {
		return func() error {
			if withError {
				return errors.New("FAILED")
			}

			time.Sleep(testSleepFor)
			return nil
		}
	}

	t.Run("TestStopOnErrorEnabled", func(t *testing.T) {
		jobs := New(time.Millisecond*300, true, nil)

		jobs.Add("A", buildCallback(true))
		jobs.Add("B", buildCallback(false))
		jobs.Add("C", buildCallback(false))

		results, err := jobs.Work(bgCtx)

		assert.Error(t, err, ErrFailed)
		assert.Equal(t, results["A"].Status, Error)
		assert.Equal(t, results["B"].Status, Aborted)
		assert.Equal(t, results["C"].Status, Aborted)
	})

	t.Run("TestStopOnErrorDisabled", func(t *testing.T) {
		jobs := New(time.Millisecond*300, false, nil)

		jobs.Add("A", buildCallback(true))
		jobs.Add("B", buildCallback(false))
		jobs.Add("C", buildCallback(false))

		results, err := jobs.Work(bgCtx)

		assert.Error(t, err, ErrFailed)
		assert.Equal(t, results["A"].Status, Error)
		assert.Equal(t, results["B"].Status, Finished)
		assert.Equal(t, results["C"].Status, Finished)
	})
}
