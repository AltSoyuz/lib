package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const DefaultHeartbeatInterval = 15 * time.Second

func WriteData(w http.ResponseWriter, flusher http.Flusher, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func Heartbeat(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, interval time.Duration) {
	if interval <= 0 {
		interval = DefaultHeartbeatInterval
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
