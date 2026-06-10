package supabase

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
)

func TestIsRealtimeTransientError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "io timeout", err: fmt.Errorf("read realtime message: read tcp 192.168.0.100:58579->172.64.149.246:443: i/o timeout"), want: true},
		{name: "net timeout", err: &net.DNSError{IsTimeout: true}, want: true},
		{name: "connection reset", err: errors.New("read tcp: connection reset by peer"), want: true},
		{name: "bad handshake", err: errors.New("websocket: bad handshake"), want: false},
		{name: "broken pipe", err: errors.New("write tcp: broken pipe"), want: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isRealtimeTransientError(tc.err); got != tc.want {
				t.Fatalf("isRealtimeTransientError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestNormalizeRealtimeError(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("read realtime message: read tcp 192.168.0.100:58579->172.64.149.246:443: i/o timeout")
	got := normalizeRealtimeError(err)
	if !strings.Contains(got, "tiempo de espera") {
		t.Fatalf("expected timeout summary, got %q", got)
	}
	if strings.Contains(got, "192.168") {
		t.Fatalf("expected normalized message without endpoint details, got %q", got)
	}
}
