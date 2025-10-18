package scheduler

import (
	"fmt"
	"strings"
	"time"

	"code88reset/pkg/logger"
)

type logAggregator struct {
	label     string
	interval  time.Duration
	buffer    []string
	lastFlush time.Time
}

func newLogAggregator(label string, interval time.Duration) *logAggregator {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &logAggregator{
		label:     label,
		interval:  interval,
		lastFlush: time.Now(),
	}
}

func (l *logAggregator) Add(format string, args ...interface{}) {
	if l == nil {
		return
	}

	l.buffer = append(l.buffer, fmt.Sprintf(format, args...))

	if time.Since(l.lastFlush) >= l.interval {
		l.Flush()
	}
}

func (l *logAggregator) Flush() {
	if l == nil || len(l.buffer) == 0 {
		l.lastFlush = time.Now()
		return
	}

	message := strings.Join(l.buffer, "; ")
	logger.Info("[%s] 最近%d分钟检查: %s", l.label, int(l.interval.Minutes()), message)

	l.buffer = nil
	l.lastFlush = time.Now()
}
