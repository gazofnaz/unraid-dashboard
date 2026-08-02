package linkresolver

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gazofnaz/unraid-dashboard/internal/model"
)

// Prober confirms whether a candidate target speaks HTTP. Implementations
// must never send credentials and must not follow redirects off-host.
type Prober interface {
	Probe(ctx context.Context, target string) model.ProbeResult
}

// HTTPProber is the production prober: short timeouts, bounded concurrency,
// HEAD first with a small GET fallback, redirects recorded but not followed.
// Self-signed certificates are accepted because probing only gathers
// evidence of a web interface; nothing sensitive is transmitted.
type HTTPProber struct {
	client *http.Client
	sem    chan struct{}
}

// NewHTTPProber builds a prober with the given timeouts and concurrency cap.
func NewHTTPProber(connectTO, totalTO time.Duration, concurrency int) *HTTPProber {
	if concurrency < 1 {
		concurrency = 1
	}
	dialer := &net.Dialer{Timeout: connectTO}
	transport := &http.Transport{
		DialContext:       dialer.DialContext,
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		DisableKeepAlives: true,
	}
	return &HTTPProber{
		client: &http.Client{
			Transport: transport,
			Timeout:   totalTO,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		sem: make(chan struct{}, concurrency),
	}
}

// Probe issues a HEAD then, when inconclusive, a minimal GET. Only the
// response class, redirect target, content type and timing are recorded.
func (p *HTTPProber) Probe(ctx context.Context, target string) model.ProbeResult {
	select {
	case p.sem <- struct{}{}:
		defer func() { <-p.sem }()
	case <-ctx.Done():
		return model.ProbeResult{Attempted: false, Error: "cancelled"}
	}

	res := p.request(ctx, http.MethodHead, target)
	if res.Attempted && !res.OK && (res.StatusCode == http.StatusMethodNotAllowed || res.StatusCode == http.StatusNotImplemented || res.Error != "") {
		if getRes := p.request(ctx, http.MethodGet, target); getRes.Attempted && (getRes.OK || res.Error != "") {
			return getRes
		}
	}
	return res
}

func (p *HTTPProber) request(ctx context.Context, method, target string) model.ProbeResult {
	res := model.ProbeResult{Attempted: true}
	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		res.Error = "invalid target"
		return res
	}
	// Never send credentials or identifying headers.
	req.Header.Set("User-Agent", "arraydeck-probe")
	start := time.Now()
	resp, err := p.client.Do(req)
	res.Elapsed = time.Since(start)
	if err != nil {
		res.Error = probeErrString(err)
		return res
	}
	defer resp.Body.Close()
	// Drain a tiny amount so keep-alive-free connections close cleanly.
	_, _ = io.CopyN(io.Discard, resp.Body, 512)

	res.StatusCode = resp.StatusCode
	res.StatusClass = fmt.Sprintf("%dxx", resp.StatusCode/100)
	res.ContentType = resp.Header.Get("Content-Type")
	res.RedirectTo = resp.Header.Get("Location")
	// 2xx and 3xx are direct evidence; 401/403 mean an auth wall in front of
	// a real web interface, which is still a web interface.
	res.OK = resp.StatusCode < 400 || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden
	return res
}

func probeErrString(err error) string {
	s := err.Error()
	switch {
	case strings.Contains(s, "connection refused"):
		return "connection refused"
	case strings.Contains(s, "no such host"):
		return "host not found"
	case strings.Contains(s, "timeout") || strings.Contains(s, "deadline"):
		return "timed out"
	default:
		return "unreachable"
	}
}

// NoopProber is used when probing is disabled; every result is "not attempted".
type NoopProber struct{}

// Probe implements Prober without any network traffic.
func (NoopProber) Probe(context.Context, string) model.ProbeResult {
	return model.ProbeResult{Attempted: false}
}
