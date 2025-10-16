package scheduler

import (
	"context"
	"time"
)

// loopController manages the shared ticking logic for schedulers.
type loopController struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	lastSubscriptionCheck time.Time
	subscriptionInterval  time.Duration
}

func newLoopController(subscriptionInterval time.Duration) *loopController {
	ctx, cancel := context.WithCancel(context.Background())
	return &loopController{
		ctx:                  ctx,
		cancel:               cancel,
		subscriptionInterval: subscriptionInterval,
	}
}

// run starts the subscription-and-reset loop until Stop is called.
func (l *loopController) run(subscriptionCheck func(), resetCheck func()) {
	// 初始检查：确保启动时立即执行一次
	subscriptionCheck()
	l.lastSubscriptionCheck = time.Now()
	resetCheck()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-l.ctx.Done():
			return
		case <-ticker.C:
			if time.Since(l.lastSubscriptionCheck) >= l.subscriptionInterval {
				subscriptionCheck()
				l.lastSubscriptionCheck = time.Now()
			}
			resetCheck()
		}
	}
}

func (l *loopController) Stop() {
	l.cancel()
}
