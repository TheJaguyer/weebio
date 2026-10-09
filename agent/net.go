package main

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// Online reports whether the box can actually reach Stremio's services, probing at most
// once per cacheFor so the boot page can poll freely.
type Online struct {
	url      string
	client   *http.Client
	cacheFor time.Duration

	mu      sync.Mutex
	checked time.Time
	online  bool
}

func NewOnline(url string) *Online {
	return &Online{url: url, client: &http.Client{Timeout: 4 * time.Second}, cacheFor: 2 * time.Second}
}

func (o *Online) Check(ctx context.Context) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	if time.Since(o.checked) < o.cacheFor {
		return o.online
	}
	o.online = o.probe(ctx)
	o.checked = time.Now()
	return o.online
}

// Forget drops the cached answer, e.g. right after joining a network.
func (o *Online) Forget() {
	o.mu.Lock()
	o.checked = time.Time{}
	o.mu.Unlock()
}

func (o *Online) probe(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, o.url, nil)
	if err != nil {
		return false
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 500
}
