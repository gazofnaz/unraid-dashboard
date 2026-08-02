package linkresolver

import (
	"context"
	"strings"
	"testing"

	"github.com/gazofnaz/unraid-dashboard/internal/model"
)

// stubProber answers from a fixed map; unlisted targets fail.
type stubProber struct {
	ok map[string]bool
}

func (s stubProber) Probe(_ context.Context, target string) model.ProbeResult {
	if s.ok[target] {
		return model.ProbeResult{Attempted: true, OK: true, StatusClass: "2xx", StatusCode: 200}
	}
	return model.ProbeResult{Attempted: true, OK: false, Error: "connection refused"}
}

func bridgeRecord(labels map[string]string, ports ...model.PortBinding) model.ContainerRecord {
	return model.ContainerRecord{
		ID: "abc123", Name: "app", Image: "vendor/app:latest",
		State: model.StateRunning, NetworkMode: "bridge", Labels: labels, Ports: ports,
		Networks: []model.NetworkAttachment{{Name: "bridge", IPAddress: "172.17.0.5"}},
	}
}

func TestBridgePolicyMapsContainerPort(t *testing.T) {
	rec := bridgeRecord(
		map[string]string{"net.unraid.docker.webui": "http://[IP]:[PORT:8080]/"},
		model.PortBinding{ContainerPort: 8080, Protocol: "tcp", HostIP: "0.0.0.0", HostPort: 8081},
	)
	dec := New(NoopProber{}).Resolve(context.Background(), Inputs{Record: rec, HostLANIP: "192.168.0.253"})
	w := dec.Winner
	if w == nil {
		t.Fatal("expected a winner")
	}
	e := w.Endpoint
	if e.AddressPolicy != model.PolicyUnraidHost || e.HostPort != 8081 || e.ContainerPort != 8080 {
		t.Errorf("unexpected endpoint %+v", e)
	}
	if e.Confidence != 0.95 {
		t.Errorf("label + mapping without probe should score 0.95, got %v", e.Confidence)
	}
}

func TestBridgeUnpublishedPortIsRejected(t *testing.T) {
	rec := bridgeRecord(map[string]string{"net.unraid.docker.webui": "http://[IP]:[PORT:9999]/"})
	dec := New(NoopProber{}).Resolve(context.Background(), Inputs{Record: rec})
	if dec.Winner != nil {
		t.Fatalf("unpublished port must not produce a winner, got %+v", dec.Winner)
	}
	found := false
	for _, c := range dec.Candidates {
		if c.Rejected && strings.Contains(c.RejectReason, "not published") {
			found = true
		}
	}
	if !found {
		t.Error("expected a rejected candidate explaining the missing mapping")
	}
}

func TestHostPolicyUsesPatternPortDirectly(t *testing.T) {
	rec := model.ContainerRecord{
		ID: "h1", Name: "plex", Image: "plexinc/pms-docker",
		State: model.StateRunning, NetworkMode: "host",
		Labels: map[string]string{"net.unraid.docker.webui": "http://[IP]:[PORT:32400]/web"},
	}
	dec := New(NoopProber{}).Resolve(context.Background(), Inputs{Record: rec, HostLANIP: "192.168.0.253"})
	w := dec.Winner
	if w == nil {
		t.Fatal("expected a winner")
	}
	if w.Endpoint.AddressPolicy != model.PolicyUnraidHost || w.Endpoint.HostPort != 32400 || w.Endpoint.Path != "/web" {
		t.Errorf("unexpected endpoint %+v", w.Endpoint)
	}
}

func TestCustomLANPolicyUsesContainerIP(t *testing.T) {
	rec := model.ContainerRecord{
		ID: "m1", Name: "pihole", Image: "pihole/pihole",
		State: model.StateRunning, NetworkMode: "br0",
		Labels:   map[string]string{"net.unraid.docker.webui": "http://[IP]:[PORT:80]/admin"},
		Networks: []model.NetworkAttachment{{Name: "br0", IPAddress: "192.168.0.50"}},
	}
	dec := New(NoopProber{}).Resolve(context.Background(), Inputs{Record: rec, HostLANIP: "192.168.0.253"})
	w := dec.Winner
	if w == nil {
		t.Fatal("expected a winner")
	}
	e := w.Endpoint
	if e.AddressPolicy != model.PolicyContainerLANIP || e.ExplicitHost != "192.168.0.50" || e.HostPort != 80 || e.Path != "/admin" {
		t.Errorf("unexpected endpoint %+v", e)
	}
}

func TestNamespaceSharingInheritsNetworkIdentity(t *testing.T) {
	vpn := model.ContainerRecord{
		ID: "vpn1", Name: "gluetun", Image: "qmcgaw/gluetun",
		State: model.StateRunning, NetworkMode: "bridge",
		Ports: []model.PortBinding{{ContainerPort: 8080, Protocol: "tcp", HostIP: "0.0.0.0", HostPort: 8080}},
	}
	rec := model.ContainerRecord{
		ID: "q1", Name: "qbittorrent", Image: "hotio/qbittorrent",
		State: model.StateRunning, NetworkMode: "container:gluetun",
		Labels: map[string]string{"net.unraid.docker.webui": "http://[IP]:[PORT:8080]/"},
	}
	dec := New(NoopProber{}).Resolve(context.Background(), Inputs{Record: rec, NetworkTarget: &vpn, HostLANIP: "192.168.0.253"})
	w := dec.Winner
	if w == nil {
		t.Fatal("expected a winner via the shared namespace")
	}
	if w.Endpoint.HostPort != 8080 || w.Endpoint.AddressPolicy != model.PolicyUnraidHost {
		t.Errorf("unexpected endpoint %+v", w.Endpoint)
	}
	if len(dec.Steps) == 0 || dec.Steps[0].Title != "Inherit network namespace" {
		t.Error("trace should explain the inherited namespace first")
	}
}

func TestProbeRaisesLabelConfidence(t *testing.T) {
	rec := bridgeRecord(
		map[string]string{"net.unraid.docker.webui": "http://[IP]:[PORT:8080]/"},
		model.PortBinding{ContainerPort: 8080, Protocol: "tcp", HostIP: "0.0.0.0", HostPort: 8080},
	)
	prober := stubProber{ok: map[string]bool{"http://192.168.0.253:8080/": true}}
	dec := New(prober).Resolve(context.Background(), Inputs{Record: rec, HostLANIP: "192.168.0.253", ProbeEnabled: true})
	if dec.Winner == nil || dec.Winner.Endpoint.Confidence != 0.98 {
		t.Fatalf("label + mapping + probe should score 0.98, got %+v", dec.Winner)
	}
}

func TestOverridePrecedence(t *testing.T) {
	rec := bridgeRecord(
		map[string]string{"net.unraid.docker.webui": "http://[IP]:[PORT:8080]/"},
		model.PortBinding{ContainerPort: 8080, Protocol: "tcp", HostIP: "0.0.0.0", HostPort: 8080},
	)
	o := &model.Override{URL: "https://qb.home.example/"}
	dec := New(NoopProber{}).Resolve(context.Background(), Inputs{Record: rec, Override: o, HostLANIP: "192.168.0.253"})
	w := dec.Winner
	if w == nil || w.Source != "override" || w.Endpoint.Confidence != 1.0 {
		t.Fatalf("override must win with confidence 1.0, got %+v", w)
	}
	if w.Endpoint.ExplicitHost != "qb.home.example" || w.Endpoint.Scheme != "https" {
		t.Errorf("unexpected endpoint %+v", w.Endpoint)
	}
	// The discovered candidate must remain visible for restore.
	discovered := 0
	for _, c := range dec.Candidates {
		if c.Source != "override" && !c.Rejected {
			discovered++
		}
	}
	if discovered == 0 {
		t.Error("discovered candidates must be retained alongside an override")
	}
}

func TestDismissedCandidateIsSkipped(t *testing.T) {
	rec := bridgeRecord(
		map[string]string{"net.unraid.docker.webui": "http://[IP]:[PORT:8080]/"},
		model.PortBinding{ContainerPort: 8080, Protocol: "tcp", HostIP: "0.0.0.0", HostPort: 8080},
	)
	// First resolve to learn the candidate identity.
	first := New(NoopProber{}).Resolve(context.Background(), Inputs{Record: rec})
	if first.Winner == nil {
		t.Fatal("setup: expected a winner")
	}
	id := CandidateIdentity(*first.Winner)
	o := &model.Override{DismissedURLs: []string{id}}
	second := New(NoopProber{}).Resolve(context.Background(), Inputs{Record: rec, Override: o})
	if second.Winner != nil && CandidateIdentity(*second.Winner) == id {
		t.Fatal("dismissed candidate must not be selected")
	}
}

func TestNonHTTPPortNeverAutoLinksWithoutProbe(t *testing.T) {
	rec := model.ContainerRecord{
		ID: "db1", Name: "postgres", Image: "postgres:16",
		State: model.StateRunning, NetworkMode: "bridge",
		Ports: []model.PortBinding{{ContainerPort: 5432, Protocol: "tcp", HostIP: "0.0.0.0", HostPort: 5432}},
	}
	dec := New(NoopProber{}).Resolve(context.Background(), Inputs{Record: rec, HostLANIP: "192.168.0.253"})
	if dec.Winner != nil {
		t.Fatalf("postgres must have no web endpoint, got %+v", dec.Winner)
	}
	var reason string
	for _, c := range dec.Candidates {
		if c.Rejected {
			reason = c.RejectReason
		}
	}
	if !strings.Contains(reason, "postgresql") {
		t.Errorf("rejection should name the conflicting service, got %q", reason)
	}
}

func TestNonHTTPPortRehabilitatedByProbe(t *testing.T) {
	rec := model.ContainerRecord{
		ID: "odd1", Name: "oddapp", Image: "vendor/oddapp",
		State: model.StateRunning, NetworkMode: "bridge",
		Ports: []model.PortBinding{{ContainerPort: 3306, Protocol: "tcp", HostIP: "0.0.0.0", HostPort: 3306}},
	}
	prober := stubProber{ok: map[string]bool{"http://192.168.0.253:3306/": true}}
	dec := New(prober).Resolve(context.Background(), Inputs{Record: rec, HostLANIP: "192.168.0.253", ProbeEnabled: true})
	if dec.Winner == nil || dec.Winner.Endpoint.Confidence != 0.60 {
		t.Fatalf("probe-confirmed port should score 0.60, got %+v", dec.Winner)
	}
}

func TestReverseProxyLabelPreserved(t *testing.T) {
	rec := bridgeRecord(
		map[string]string{"traefik.http.routers.whoami.rule": "Host(`whoami.home.example`)"},
		model.PortBinding{ContainerPort: 80, Protocol: "tcp", HostIP: "0.0.0.0", HostPort: 8085},
	)
	dec := New(NoopProber{}).Resolve(context.Background(), Inputs{Record: rec})
	w := dec.Winner
	if w == nil || w.Source != "proxy:traefik" {
		t.Fatalf("expected the traefik route to win, got %+v", w)
	}
	if w.Endpoint.AddressPolicy != model.PolicyExplicitHost || w.Endpoint.ExplicitHost != "whoami.home.example" {
		t.Errorf("unexpected endpoint %+v", w.Endpoint)
	}
	if w.Endpoint.Confidence != 0.88 {
		t.Errorf("proxy rule should score 0.88, got %v", w.Endpoint.Confidence)
	}
}

func TestCatalogFallback(t *testing.T) {
	rec := model.ContainerRecord{
		ID: "s1", Name: "sonarr", Image: "lscr.io/linuxserver/sonarr:latest",
		State: model.StateRunning, NetworkMode: "bridge",
		Ports: []model.PortBinding{{ContainerPort: 8989, Protocol: "tcp", HostIP: "0.0.0.0", HostPort: 8989}},
	}
	prober := stubProber{ok: map[string]bool{"http://192.168.0.253:8989/": true}}
	dec := New(prober).Resolve(context.Background(), Inputs{Record: rec, HostLANIP: "192.168.0.253", ProbeEnabled: true})
	w := dec.Winner
	if w == nil || !strings.HasPrefix(w.Source, "catalog:") {
		t.Fatalf("expected catalog winner, got %+v", w)
	}
	if w.Endpoint.Confidence != 0.75 {
		t.Errorf("catalog + mapping + probe should score 0.75, got %v", w.Endpoint.Confidence)
	}
}

func TestLoopbackBindingNotAutoSelected(t *testing.T) {
	rec := bridgeRecord(
		map[string]string{"net.unraid.docker.webui": "http://[IP]:[PORT:9000]/"},
		model.PortBinding{ContainerPort: 9000, Protocol: "tcp", HostIP: "127.0.0.1", HostPort: 9000},
	)
	dec := New(NoopProber{}).Resolve(context.Background(), Inputs{Record: rec})
	if dec.Winner != nil {
		t.Fatalf("loopback-only binding must not be auto-selected, got %+v", dec.Winner)
	}
}
