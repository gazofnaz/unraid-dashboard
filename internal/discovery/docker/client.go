// Package docker is a minimal Docker Engine API adapter. It speaks the small
// read-only subset ArrayDeck needs (list, inspect, events, version) over a
// unix socket or a tcp socket-proxy, keeping the dependency surface tiny and
// making the hardened DOCKER_HOST=tcp://docker-proxy:2375 mode first-class.
package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const apiVersion = "v1.24"

// Client is a thin HTTP client for the Docker Engine API.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient builds a client for host, accepting unix:///path and tcp://host:port.
func NewClient(host string) (*Client, error) {
	u, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("parse DOCKER_HOST: %w", err)
	}
	transport := &http.Transport{}
	base := "http://docker"
	switch u.Scheme {
	case "unix", "":
		socketPath := u.Path
		if u.Scheme == "" {
			socketPath = host
		}
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socketPath)
		}
	case "tcp", "http":
		base = "http://" + u.Host
	default:
		return nil, fmt.Errorf("unsupported DOCKER_HOST scheme %q", u.Scheme)
	}
	return &Client{
		http:    &http.Client{Transport: transport},
		baseURL: base,
	}, nil
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/"+apiVersion+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("docker api %s: %s: %s", path, resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Ping reports whether the Docker daemon is reachable.
func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/_ping", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("docker ping: %s", resp.Status)
	}
	return nil
}

// ListContainerIDs returns the IDs of all containers, including stopped ones.
func (c *Client) ListContainerIDs(ctx context.Context) ([]string, error) {
	var list []struct {
		ID string `json:"Id"`
	}
	if err := c.get(ctx, "/containers/json?all=1", &list); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(list))
	for _, item := range list {
		ids = append(ids, item.ID)
	}
	return ids, nil
}

// Inspect returns the full inspect document for one container.
func (c *Client) Inspect(ctx context.Context, id string) (*InspectResponse, error) {
	var doc InspectResponse
	if err := c.get(ctx, "/containers/"+url.PathEscape(id)+"/json", &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

// Event is one Docker container lifecycle event.
type Event struct {
	Type   string `json:"Type"`
	Action string `json:"Action"`
	Actor  struct {
		ID string `json:"ID"`
	} `json:"Actor"`
}

// StreamEvents delivers container events on the returned channel until ctx is
// cancelled or the stream breaks; the channel is closed on exit.
func (c *Client) StreamEvents(ctx context.Context) (<-chan Event, error) {
	filters := url.QueryEscape(`{"type":["container"]}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/"+apiVersion+"/events?filters="+filters, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("docker events: %s", resp.Status)
	}
	ch := make(chan Event, 16)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		dec := json.NewDecoder(resp.Body)
		for {
			var ev Event
			if err := dec.Decode(&ev); err != nil {
				return
			}
			select {
			case ch <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}
