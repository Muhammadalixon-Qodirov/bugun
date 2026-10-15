package jobs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Heartbeat pings an external uptime monitor (HEARTBEAT_URL, e.g. a
// healthchecks.io check) from the scheduler loop. The monitor alerts when
// the pings stop: the process died, hung, or its scheduler stopped ticking,
// which /healthz alone cannot tell. A failed ping is logged and forgotten.
type Heartbeat struct {
	URL    string
	Every  time.Duration
	Client *http.Client // nil: a client with a 10 s timeout

	mu   sync.Mutex
	last time.Time
}

// due reports whether Every has passed since the last ping, and if so
// counts now as the last one.
func (h *Heartbeat) due(now time.Time) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.last.IsZero() && now.Sub(h.last) < h.Every {
		return false
	}
	h.last = now
	return true
}

// Ping requests the URL once.
func (h *Heartbeat) Ping(ctx context.Context) {
	if err := h.ping(ctx); err != nil && ctx.Err() == nil {
		slog.WarnContext(ctx, "heartbeat ping failed", "err", err)
	}
}

func (h *Heartbeat) ping(ctx context.Context) error {
	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.URL, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err // the URL is a secret of its own: keep it out of the logs
	}
	if err != nil {
		return err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<10))
	if res.StatusCode/100 != 2 {
		return fmt.Errorf("heartbeat: %s", res.Status)
	}
	return nil
}
