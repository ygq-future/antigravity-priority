package runtime

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type tickerWorker struct {
	interval time.Duration
	cancel   context.CancelFunc
	done     chan struct{}
}

func (w *tickerWorker) start(rootCtx context.Context, rt *Runtime) {
	ctx, cancel := context.WithCancel(rootCtx)
	w.cancel = cancel
	interval := w.interval
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	go func() {
		defer close(w.done)
		// Startup delay avoids immediate callback into host during register/reconfigure call stack.
		startupDelay := 2 * time.Second
		timer := time.NewTimer(startupDelay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			stopTimer(timer)
			return
		}
		for {
			if ctx.Err() != nil {
				return
			}
			now := rt.clock.Now()
			plan, wait, active := rt.evaluateWorkerStep(now, interval)
			if active && plan.ShouldRunNow {
				runErr := rt.AutoApply(ctx)
				if ctx.Err() != nil {
					return
				}
				now = rt.clock.Now()
				if runErr != nil {
					retryWait := min(interval, 2*time.Minute)
					if errors.Is(runErr, ErrRunInProgress) {
						retryWait = 15 * time.Second
					}
					rt.latchRetryDeadline(now.UTC().Add(retryWait))
					wait = retryWait
				} else {
					_, wait, _ = rt.evaluateWorkerStep(now, interval)
				}
			}

			var waitTimer *time.Timer
			var waitC <-chan time.Time
			if wait > 0 {
				waitTimer = time.NewTimer(wait)
				waitC = waitTimer.C
			}

			select {
			case <-ctx.Done():
				stopTimer(waitTimer)
				return
			case <-rt.autoApplyWake:
				stopTimer(waitTimer)
			case <-waitC:
			}
		}
	}()
}

func stopTimer(timer *time.Timer) {
	if timer == nil {
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func stopWorker(ctx context.Context, worker *tickerWorker) error {
	if worker == nil {
		return nil
	}
	if worker.cancel != nil {
		worker.cancel()
	}
	select {
	case <-worker.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait ticker worker: %w", ctx.Err())
	}
}

func stopNewWorker(worker *tickerWorker, err error) error {
	if worker != nil {
		if worker.cancel != nil {
			worker.cancel()
		}
	}
	return err
}
