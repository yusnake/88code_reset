package scheduler

import (
	"context"
	"time"
)

// loopController manages the shared ticking logic for schedulers.
type loopController struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func newLoopController() *loopController {
	ctx, cancel := context.WithCancel(context.Background())
	return &loopController{
		ctx:    ctx,
		cancel: cancel,
	}
}

// run starts the reset-check loop until Stop is called.
func (l *loopController) run(resetCheck func()) {
	// 初始检查：确保启动时立即执行一次
	resetCheck()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-l.ctx.Done():
			return
		case <-ticker.C:
			resetCheck()
		}
	}
}

func (l *loopController) Stop() {
	l.cancel()
}
