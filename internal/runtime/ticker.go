package runtime

import (
	"context"
	"fmt"
	"time"
)

type tickerWorker struct {
	interval time.Duration
	ticker   Ticker
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
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		}
		for {
			if ctx.Err() != nil {
				return
			}
			_ = rt.AutoApply(ctx)
			wait := rt.nextAutoApplyWait(interval)
			timer = time.NewTimer(wait)
			select {
			case <-timer.C:
			case <-ctx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return
			}
		}
	}()
}

func stopWorker(ctx context.Context, worker *tickerWorker) error {
	if worker == nil {
		return nil
	}
	if worker.ticker != nil {
		worker.ticker.Stop()
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
		if worker.ticker != nil {
			worker.ticker.Stop()
		}
		if worker.cancel != nil {
			worker.cancel()
		}
	}
	return err
}

type timeTickerFactory struct{}

func (timeTickerFactory) NewTicker(interval time.Duration) Ticker {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	return &timeTicker{ticker: time.NewTicker(interval)}
}

type timeTicker struct {
	ticker *time.Ticker
}

func (t *timeTicker) Chan() <-chan time.Time {
	return t.ticker.C
}

func (t *timeTicker) Stop() {
	t.ticker.Stop()
}
