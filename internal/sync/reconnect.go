package sync

import (
	"fmt"
	"strings"
	"time"

	"sycronizafhir/internal/monitor"
)

const reconnectLogMinInterval = 60 * time.Second

type reconnectBackoff struct {
	min     time.Duration
	max     time.Duration
	current time.Duration
}

func newReconnectBackoff(min, max time.Duration) *reconnectBackoff {
	if min <= 0 {
		min = 5 * time.Second
	}
	if max < min {
		max = min
	}
	return &reconnectBackoff{
		min:     min,
		max:     max,
		current: min,
	}
}

func (b *reconnectBackoff) Next() time.Duration {
	wait := b.current
	next := b.current * 2
	if next > b.max {
		next = b.max
	}
	b.current = next
	return wait
}

func (b *reconnectBackoff) Reset() {
	b.current = b.min
}

type reconnectFailureLog struct {
	failures    int
	lastLogAt   time.Time
	lastSummary string
	minInterval time.Duration
}

func newReconnectFailureLog() *reconnectFailureLog {
	return &reconnectFailureLog{
		minInterval: reconnectLogMinInterval,
	}
}

func (l *reconnectFailureLog) Record(runtime *monitor.Runtime, prefix, summary string, wait time.Duration) {
	if runtime == nil {
		return
	}

	l.failures++
	now := time.Now()
	summary = strings.TrimSpace(summary)
	if summary == "" {
		summary = "error de conexión"
	}

	shouldLog := l.failures == 1 ||
		summary != l.lastSummary ||
		now.Sub(l.lastLogAt) >= l.minInterval

	if !shouldLog {
		return
	}

	if l.failures > 1 && summary == l.lastSummary {
		runtime.AddLog(fmt.Sprintf(
			"%s: reconectando (%d intentos, último: %s); próximo en %s",
			prefix,
			l.failures,
			summary,
			wait,
		))
	} else {
		runtime.AddLog(fmt.Sprintf("%s: desconectado — %s; reintento en %s", prefix, summary, wait))
	}

	l.lastLogAt = now
	l.lastSummary = summary
}

func (l *reconnectFailureLog) Reset(runtime *monitor.Runtime, prefix string) {
	if runtime != nil && l.failures > 0 {
		runtime.AddLog(fmt.Sprintf("%s: realtime reconectado tras %d intento(s)", prefix, l.failures))
	}
	l.failures = 0
	l.lastSummary = ""
}
