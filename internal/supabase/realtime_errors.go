package supabase

import (
	"errors"
	"net"
	"strings"
)

func isRealtimeTransientError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, net.ErrClosed) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "i/o timeout"),
		strings.Contains(lower, "connection reset"),
		strings.Contains(lower, "broken pipe"),
		strings.Contains(lower, "connection refused"),
		strings.Contains(lower, "no such host"),
		strings.Contains(lower, "eof"),
		strings.Contains(lower, "use of closed network connection"),
		strings.Contains(lower, "unexpected eof"):
		return true
	default:
		return false
	}
}

func normalizeRealtimeError(err error) string {
	if err == nil {
		return ""
	}

	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "i/o timeout"):
		return "tiempo de espera en lectura realtime (red inactiva o interrumpida)"
	case strings.Contains(lower, "bad handshake"):
		return "handshake realtime rechazado (revisar SUPABASE_SERVICE_ROLE_KEY y canal)"
	case strings.Contains(lower, "connection reset"),
		strings.Contains(lower, "broken pipe"),
		strings.Contains(lower, "use of closed network connection"):
		return "conexión realtime cerrada por el servidor o la red"
	case strings.Contains(lower, "connection refused"):
		return "conexión realtime rechazada (host/puerto o firewall)"
	case strings.Contains(lower, "no such host"):
		return "host realtime no resuelto (SUPABASE_REALTIME_URL)"
	default:
		msg := err.Error()
		if len(msg) > 160 {
			return msg[:157] + "..."
		}
		return msg
	}
}
