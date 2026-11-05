package batchctrl

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewBatchController(t *testing.T) {
	total := 1000

	totalTasks := int32(0)
	testHandle := func(futures []Future) {
		atomic.AddInt32(&totalTasks, int32(len(futures)))
		for i := range futures {
			futures[i].Reply(nil, nil)
		}
	}

	ctrl := NewBatchController(context.Background(), CtrlConfig{
		QueueSize:     32,
		MaxBatchCount: 16,
		WaitTime:      32 * time.Millisecond,
		Concurrency:   8,
		Handler:       testHandle,
	})

	wg := &sync.WaitGroup{}

	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			future := ctrl.Submit(fmt.Sprintf("%d", i))
			_, _ = future.Done()
		}(i)
	}

	wg.Wait()
	assert.Equal(t, total, int(atomic.LoadInt32(&totalTasks)))
	ctrl.Stop()
}

func TestNewBatchControllerSubmitTimeout(t *testing.T) {
	total := 1000

	totalTasks := int32(0)
	testHandle := func(futures []Future) {
		time.Sleep(100 * time.Millisecond)
		atomic.AddInt32(&totalTasks, int32(len(futures)))
		for i := range futures {
			futures[i].Reply(nil, nil)
		}
	}

	ctrl := NewBatchController(context.Background(), CtrlConfig{
		QueueSize:     1,
		MaxBatchCount: uint32(total * 2),
		WaitTime:      32 * time.Millisecond,
		Concurrency:   8,
		Handler:       testHandle,
	})

	wg := &sync.WaitGroup{}

	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			future := ctrl.SubmitWithTimeout(fmt.Sprintf("%d", i), time.Millisecond)
			_, err := future.Done()
			if err != nil {
				assert.True(t, errors.Is(err, context.DeadlineExceeded), err)
			}
		}(i)
	}

	wg.Wait()
	ctrl.Stop()
}

func TestNewBatchControllerDoneTimeout(t *testing.T) {
	total := 1000

	totalTasks := int32(0)
	testHandle := func(futures []Future) {
		time.Sleep(100 * time.Millisecond)
		atomic.AddInt32(&totalTasks, int32(len(futures)))
		for i := range futures {
			futures[i].Reply(nil, nil)
		}
	}

	ctrl := NewBatchController(context.Background(), CtrlConfig{
		QueueSize:     1,
		MaxBatchCount: uint32(total * 2),
		WaitTime:      32 * time.Millisecond,
		Concurrency:   8,
		Handler:       testHandle,
	})

	wg := &sync.WaitGroup{}

	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			future := ctrl.Submit(fmt.Sprintf("%d", i))
			_, err := future.DoneTimeout(time.Millisecond)
			if err != nil {
				assert.True(t, errors.Is(err, context.DeadlineExceeded), err)
			}
		}(i)
	}

	wg.Wait()
	ctrl.Stop()
}
func TestNewBatchControllerStop(t *testing.T) {
	total := 1000

	totalTasks := int32(0)
	testHandle := func(futures []Future) {
		atomic.AddInt32(&totalTasks, int32(len(futures)))
		for i := range futures {
			futures[i].Reply(atomic.LoadInt32(&totalTasks), nil)
		}
	}

	ctrl := NewBatchController(context.Background(), CtrlConfig{
		QueueSize:     uint32(total * 2),
		MaxBatchCount: 64,
		WaitTime:      32 * time.Millisecond,
		Concurrency:   8,
		Handler:       testHandle,
	})

	sbWg := &sync.WaitGroup{}
	wg := &sync.WaitGroup{}
	sbWg.Add(total)
	wg.Add(total)
	submitTask := int32(0)
	for i := 0; i < total; i++ {
		go func(i int) {
			defer func() {
				wg.Done()
			}()
			future := ctrl.Submit(fmt.Sprintf("%d", i))
			atomic.AddInt32(&submitTask, 1)
			sbWg.Done()
			_, err := future.Done()
			if err != nil {
				assert.ErrorIs(t, err, ErrorBatchControllerStopped)
			}
		}(i)
	}

	ctrl.Stop()
	t.Log("BatchController already stop")
	sbWg.Wait()
	t.Logf("submit jobs : %d", atomic.LoadInt32(&submitTask))
	wg.Wait()
	t.Log("finish all batch job")
}

func TestNewBatchControllerGracefulStop(t *testing.T) {
	total := 1000

	totalTasks := int32(0)
	testHandle := func(futures []Future) {
		atomic.AddInt32(&totalTasks, int32(len(futures)))
		for i := range futures {
			futures[i].Reply(atomic.LoadInt32(&totalTasks), nil)
		}
	}

	ctrl := NewBatchController(context.Background(), CtrlConfig{
		QueueSize:     uint32(total * 2),
		MaxBatchCount: 64,
		WaitTime:      32 * time.Millisecond,
		Concurrency:   8,
		Handler:       testHandle,
	})

	sbWg := &sync.WaitGroup{}
	wg := &sync.WaitGroup{}
	sbWg.Add(total)
	wg.Add(total)
	submitTask := int32(0)
	for i := 0; i < total; i++ {
		go func(i int) {
			defer func() {
				wg.Done()
			}()
			future := ctrl.Submit(fmt.Sprintf("%d", i))
			atomic.AddInt32(&submitTask, 1)
			sbWg.Done()
			_, err := future.Done()
			if err != nil {
				assert.ErrorIs(t, err, ErrorBatchControllerStopped)
			}
		}(i)
	}

	ctrl.GracefulStop()
	t.Log("BatchController already stop")
	sbWg.Wait()
	t.Logf("submit jobs : %d", atomic.LoadInt32(&submitTask))
	wg.Wait()
	t.Log("finish all batch job")
}

func TestNewBatchControllerGracefulStopFlushesDrainedPartialBatch(t *testing.T) {
	total := 3

	totalTasks := int32(0)
	testHandle := func(futures []Future) {
		atomic.AddInt32(&totalTasks, int32(len(futures)))
		for i := range futures {
			futures[i].Reply(nil, nil)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	ctrl := &BatchController{
		label: "graceful-drain-partial-batch",
		conf: CtrlConfig{
			QueueSize:     uint32(total),
			MaxBatchCount: 16,
			WaitTime:      time.Hour,
			Concurrency:   1,
			Handler:       testHandle,
		},
		cancel:     cancel,
		tasksChan:  make(chan Future, total),
		workers:    make([]chan []Future, 0, 1),
		idleSignal: make(chan int, 1),
		handler:    testHandle,
	}
	ctrl.runWorkers(ctx)

	futures := make([]Future, 0, total)
	for i := 0; i < total; i++ {
		futureCtx, futureCancel := context.WithCancel(context.Background())
		future := &future{
			task:      fmt.Sprintf("%d", i),
			ctx:       futureCtx,
			cancel:    futureCancel,
			setsignal: make(chan struct{}),
		}
		futures = append(futures, future)
		ctrl.tasksChan <- future
	}

	atomic.StoreInt32(&ctrl.stop, shutdownGraceful)
	cancel()
	ctrl.mainLoop(ctx)

	for _, future := range futures {
		_, err := future.DoneTimeout(time.Second)
		assert.NoError(t, err)
	}
	assert.Equal(t, total, int(atomic.LoadInt32(&totalTasks)))
}

func TestNewBatchControllerGracefulStopFlushesAcceptedPartialBatch(t *testing.T) {
	total := 3

	totalTasks := int32(0)
	testHandle := func(futures []Future) {
		atomic.AddInt32(&totalTasks, int32(len(futures)))
		for i := range futures {
			futures[i].Reply(nil, nil)
		}
	}

	ctrl := NewBatchController(context.Background(), CtrlConfig{
		QueueSize:     uint32(total),
		MaxBatchCount: 16,
		WaitTime:      time.Hour,
		Concurrency:   1,
		Handler:       testHandle,
	})

	futures := make([]Future, 0, total)
	for i := 0; i < total; i++ {
		futures = append(futures, ctrl.Submit(fmt.Sprintf("%d", i)))
	}

	ctrl.GracefulStop()
	for _, future := range futures {
		_, err := future.DoneTimeout(time.Second)
		assert.NoError(t, err)
	}
	assert.Equal(t, total, int(atomic.LoadInt32(&totalTasks)))
}

func TestBatchControllerRejectsSubmitAfterParentContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ctrl := NewBatchController(ctx, CtrlConfig{
		Label:         "cancelled-parent",
		QueueSize:     1,
		MaxBatchCount: 1,
		WaitTime:      time.Millisecond,
		Concurrency:   1,
		Handler:       func([]Future) {},
	})

	cancel()
	assert.Eventually(t, ctrl.isStop, time.Second, time.Millisecond)
	_, err := ctrl.Submit("late-task").Done()
	assert.ErrorIs(t, err, ErrorBatchControllerStopped)
}
