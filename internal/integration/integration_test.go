// Package integration runs fixture Docker inspect documents and fixture
// Unraid templates through the full normalize → match → resolve → classify
// pipeline, per the CODEX_PROMPT test matrix.
package integration

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/gazofnaz/unraid-dashboard/internal/classifier"
	"github.com/gazofnaz/unraid-dashboard/internal/discovery/docker"
	"github.com/gazofnaz/unraid-dashboard/internal/discovery/templates"
	"github.com/gazofnaz/unraid-dashboard/internal/linkresolver"
	"github.com/gazofnaz/unraid-dashboard/internal/model"
)

const hostLANIP = "192.168.0.253"

type stubProber struct{ ok map[string]bool }

func (s stubProber) Probe(_ context.Context, target string) model.ProbeResult {
	if s.ok[target] {
		return model.ProbeResult{Attempted: true, OK: true, StatusClass: "2xx", StatusCode: 200}
	}
	return model.ProbeResult{Attempted: true, OK: false, Error: "connection refused"}
}

func loadRecord(t *testing.T, name string, tmpl *templates.Adapter) model.ContainerRecord {
	t.Helper()
	raw, err := os.ReadFile("testdata/docker/" + name + ".json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var doc docker.InspectResponse
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}
	rec := docker.Normalize(&doc, "")
	rec.Template = tmpl.Match(rec.Name, rec.Image)
	return rec
}

func resolveWith(t *testing.T, rec model.ContainerRecord, prober linkresolver.Prober) model.Decision {
	t.Helper()
	r := linkresolver.New(prober)
	return r.Resolve(context.Background(), linkresolver.Inputs{
		Record:       rec,
		HostLANIP:    hostLANIP,
		ProbeEnabled: true,
	})
}

func newTemplates(t *testing.T) *templates.Adapter {
	t.Helper()
	tmpl := templates.New("testdata/templates")
	tmpl.Reload()
	if !tmpl.Status().Available {
		t.Fatal("template fixtures not readable")
	}
	return tmpl
}

func TestPlexHostNetwork(t *testing.T) {
	tmpl := newTemplates(t)
	rec := loadRecord(t, "plex", tmpl)
	if rec.NetworkMode != "host" {
		t.Fatalf("fixture wrong: %s", rec.NetworkMode)
	}
	dec := resolveWith(t, rec, stubProber{ok: map[string]bool{"http://192.168.0.253:32400/web": true}})
	w := dec.Winner
	if w == nil {
		t.Fatal("expected a winner")
	}
	if !strings.HasPrefix(w.Source, "label:net.unraid.docker.webui") {
		t.Errorf("winner source = %s", w.Source)
	}
	e := w.Endpoint
	if e.AddressPolicy != model.PolicyUnraidHost || e.HostPort != 32400 || e.Path != "/web" {
		t.Errorf("endpoint %+v", e)
	}
	if e.Confidence != 0.98 {
		t.Errorf("confidence = %v, want 0.98 (label + mapping + probe)", e.Confidence)
	}
	group, source := classifier.Classify(rec, nil)
	if group != "Media" || source != "catalog" {
		t.Errorf("classified %q via %q", group, source)
	}
}

func TestQbittorrentBridgeWithTemplate(t *testing.T) {
	tmpl := newTemplates(t)
	rec := loadRecord(t, "qbittorrent", tmpl)
	if rec.Template == nil || rec.Template.File != "my-qbittorrent.xml" {
		t.Fatalf("template not matched by name: %+v", rec.Template)
	}
	dec := resolveWith(t, rec, stubProber{ok: map[string]bool{"http://192.168.0.253:8080/": true}})
	w := dec.Winner
	if w == nil {
		t.Fatal("expected a winner")
	}
	// The Docker label (P90) must beat the template (P85) even though both exist.
	if !strings.HasPrefix(w.Source, "label:") {
		t.Errorf("winner source = %s, want the docker label", w.Source)
	}
	if w.Endpoint.HostPort != 8080 || w.Endpoint.Confidence != 0.98 {
		t.Errorf("endpoint %+v", w.Endpoint)
	}
	// The template candidate must still be visible in the trace.
	hasTemplateCandidate := false
	for _, c := range dec.Candidates {
		if strings.HasPrefix(c.Source, "template:") {
			hasTemplateCandidate = true
		}
	}
	if !hasTemplateCandidate {
		t.Error("template candidate missing from the decision")
	}
	group, source := classifier.Classify(rec, nil)
	if group != "Downloads" || source != "unraid-template" {
		t.Errorf("classified %q via %q", group, source)
	}
}

func TestHomeAssistantTemplateFallback(t *testing.T) {
	tmpl := newTemplates(t)
	rec := loadRecord(t, "homeassistant", tmpl)
	// No name match ("Home Assistant" != "homeassistant"): repository match.
	if rec.Template == nil || rec.Template.File != "my-homeassistant.xml" {
		t.Fatalf("template not matched by repository: %+v", rec.Template)
	}
	dec := resolveWith(t, rec, stubProber{ok: map[string]bool{"http://192.168.0.253:8123/": true}})
	w := dec.Winner
	if w == nil {
		t.Fatal("expected a winner")
	}
	if !strings.HasPrefix(w.Source, "template:") {
		t.Errorf("winner source = %s, want the dockerMan template", w.Source)
	}
	if w.Endpoint.HostPort != 8123 || w.Endpoint.AddressPolicy != model.PolicyUnraidHost {
		t.Errorf("endpoint %+v", w.Endpoint)
	}
	if w.Endpoint.Confidence != 0.92 {
		t.Errorf("confidence = %v, want 0.92 (dockerMan + valid mapping)", w.Endpoint.Confidence)
	}
	group, _ := classifier.Classify(rec, nil)
	if group != "Home" {
		t.Errorf("classified %q", group)
	}
}

func TestPostgresHasNoWebInterface(t *testing.T) {
	tmpl := newTemplates(t)
	rec := loadRecord(t, "postgres", tmpl)
	dec := resolveWith(t, rec, stubProber{ok: map[string]bool{}})
	if dec.Winner != nil {
		t.Fatalf("postgres must have no endpoint, got %+v", dec.Winner)
	}
	var rejectReason string
	for _, c := range dec.Candidates {
		if c.Rejected {
			rejectReason = c.RejectReason
		}
	}
	if !strings.Contains(rejectReason, "postgresql") {
		t.Errorf("expected the postgresql exclusion in the trace, got %q", rejectReason)
	}
	group, _ := classifier.Classify(rec, nil)
	if group != "Databases" {
		t.Errorf("classified %q", group)
	}
	if rec.Health != "healthy" {
		t.Errorf("health lost in normalization: %q", rec.Health)
	}
}

func TestReverseProxiedComposeApp(t *testing.T) {
	tmpl := newTemplates(t)
	rec := loadRecord(t, "whoami", tmpl)
	if rec.Template != nil {
		t.Fatalf("compose app must not match any template, got %+v", rec.Template)
	}
	dec := resolveWith(t, rec, stubProber{ok: map[string]bool{}})
	w := dec.Winner
	if w == nil {
		t.Fatal("expected the traefik route to win")
	}
	if w.Source != "proxy:traefik" {
		t.Errorf("winner source = %s", w.Source)
	}
	e := w.Endpoint
	if e.AddressPolicy != model.PolicyExplicitHost || e.ExplicitHost != "whoami.home.example" {
		t.Errorf("endpoint %+v", e)
	}
	if e.Confidence != 0.88 {
		t.Errorf("confidence = %v, want 0.88 (reverse-proxy host rule)", e.Confidence)
	}
}
