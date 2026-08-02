// Package unraid is an optional adapter for the Unraid GraphQL API. It uses a
// read-only API key, discovers capabilities through schema introspection
// instead of assuming a fixed Unraid version, and degrades gracefully when
// unavailable. The key is never sent to the browser.
package unraid

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gazofnaz/unraid-dashboard/internal/model"
)

// Adapter talks to one Unraid server's GraphQL endpoint.
type Adapter struct {
	endpoint string
	apiKey   string
	http     *http.Client

	mu       sync.Mutex
	fields   map[string]bool // root query fields discovered by introspection
	lastErr  error
	identity Identity
}

// Identity is the server metadata the adapter can discover.
type Identity struct {
	Hostname      string
	UnraidVersion string
	UptimeSeconds int64
	LANAddresses  []string // active private IPv4 addresses, most preferred first
	Interfaces    map[string][]string
}

// New returns an adapter, or nil when no endpoint is configured — callers
// treat a nil adapter as "source not configured".
func New(endpoint, apiKey string) *Adapter {
	if endpoint == "" {
		return nil
	}
	return &Adapter{
		endpoint: strings.TrimRight(endpoint, "/"),
		apiKey:   apiKey,
		http:     &http.Client{Timeout: 5 * time.Second},
	}
}

// EndpointHostname returns the hostname portion of the configured API URL,
// used as a host-identity fallback.
func (a *Adapter) EndpointHostname() string {
	if a == nil {
		return ""
	}
	u, err := url.Parse(a.endpoint)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

type gqlRequest struct {
	Query string `json:"query"`
}

type gqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func (a *Adapter) query(ctx context.Context, q string) (json.RawMessage, error) {
	body, err := json.Marshal(gqlRequest{Query: q})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if a.apiKey != "" {
		req.Header.Set("x-api-key", a.apiKey)
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unraid api: %s", resp.Status)
	}
	var out gqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Errors) > 0 {
		return out.Data, fmt.Errorf("unraid api: %s", out.Errors[0].Message)
	}
	return out.Data, nil
}

// Refresh introspects capabilities and pulls server identity. Errors are
// stored, not fatal: Docker metadata keeps the dashboard usable on its own.
func (a *Adapter) Refresh(ctx context.Context) {
	if a == nil {
		return
	}
	err := a.introspect(ctx)
	if err == nil {
		err = a.fetchIdentity(ctx)
	}
	a.mu.Lock()
	a.lastErr = err
	a.mu.Unlock()
}

func (a *Adapter) introspect(ctx context.Context) error {
	raw, err := a.query(ctx, `{ __schema { queryType { fields { name } } } }`)
	if err != nil {
		return err
	}
	var doc struct {
		Schema struct {
			QueryType struct {
				Fields []struct {
					Name string `json:"name"`
				} `json:"fields"`
			} `json:"queryType"`
		} `json:"__schema"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	fields := map[string]bool{}
	for _, f := range doc.Schema.QueryType.Fields {
		fields[f.Name] = true
	}
	a.mu.Lock()
	a.fields = fields
	a.mu.Unlock()
	return nil
}

func (a *Adapter) hasField(name string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.fields[name]
}

// fetchIdentity reads hostname, version, uptime and network addresses using
// only fields the schema advertises. Every part is optional.
func (a *Adapter) fetchIdentity(ctx context.Context) error {
	ident := Identity{Interfaces: map[string][]string{}}

	if a.hasField("info") {
		raw, err := a.query(ctx, `{ info { os { hostname uptime } versions { unraid } } }`)
		if err == nil {
			var doc struct {
				Info struct {
					OS struct {
						Hostname string `json:"hostname"`
						Uptime   string `json:"uptime"`
					} `json:"os"`
					Versions struct {
						Unraid string `json:"unraid"`
					} `json:"versions"`
				} `json:"info"`
			}
			if json.Unmarshal(raw, &doc) == nil {
				ident.Hostname = doc.Info.OS.Hostname
				ident.UnraidVersion = doc.Info.Versions.Unraid
				if t, err := time.Parse(time.RFC3339, doc.Info.OS.Uptime); err == nil {
					ident.UptimeSeconds = int64(time.Since(t).Seconds())
				}
			}
		}
	}

	if a.hasField("network") {
		raw, err := a.query(ctx, `{ network { iface { ifaceName ipv4 ip } } }`)
		if err == nil && raw != nil {
			parseInterfaces(raw, &ident)
		}
	}

	a.mu.Lock()
	a.identity = ident
	a.mu.Unlock()
	return nil
}

// parseInterfaces tolerates schema drift: it walks the JSON generically and
// collects anything shaped like an interface name with IPv4 addresses.
func parseInterfaces(raw json.RawMessage, ident *Identity) {
	var doc struct {
		Network struct {
			Iface []map[string]any `json:"iface"`
		} `json:"network"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return
	}
	for _, iface := range doc.Network.Iface {
		name, _ := iface["ifaceName"].(string)
		var addrs []string
		for _, key := range []string{"ipv4", "ip"} {
			if s, ok := iface[key].(string); ok && s != "" {
				addrs = append(addrs, strings.Split(s, "/")[0])
			}
		}
		for _, addr := range addrs {
			ip := net.ParseIP(addr)
			if ip == nil || ip.To4() == nil || !ip.IsPrivate() {
				continue
			}
			if name != "" {
				ident.Interfaces[name] = append(ident.Interfaces[name], addr)
			}
			if !contains(ident.LANAddresses, addr) {
				ident.LANAddresses = append(ident.LANAddresses, addr)
			}
		}
	}
}

// Identity returns the last discovered identity snapshot.
func (a *Adapter) Identity() Identity {
	if a == nil {
		return Identity{}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.identity
}

// Status reports availability for the UI. A nil adapter is "not configured".
func (a *Adapter) Status() model.SourceStatus {
	st := model.SourceStatus{Name: "unraid-api"}
	if a == nil {
		st.Detail = "not configured"
		return st
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.lastErr != nil {
		st.Detail = "unreachable"
		st.Error = redact(a.lastErr.Error(), a.apiKey)
		return st
	}
	if a.fields == nil {
		st.Detail = "not contacted yet"
		return st
	}
	st.Available = true
	st.Detail = fmt.Sprintf("connected, %d query capabilities", len(a.fields))
	return st
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// redact keeps secrets out of logs and API responses.
func redact(s, secret string) string {
	if secret == "" {
		return s
	}
	return strings.ReplaceAll(s, secret, "[redacted]")
}
