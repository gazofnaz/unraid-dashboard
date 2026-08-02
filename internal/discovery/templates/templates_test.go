package templates

import "testing"

func newTestAdapter(t *testing.T) *Adapter {
	t.Helper()
	a := New("testdata")
	a.Reload()
	if !a.Status().Available || a.Count() != 3 {
		t.Fatalf("expected 3 fixture templates, status=%+v count=%d", a.Status(), a.Count())
	}
	return a
}

func TestMatchByNameCaseInsensitive(t *testing.T) {
	a := newTestAdapter(t)
	m := a.Match("Sonarr", "some/other-image")
	if m == nil || m.File != "my-sonarr.xml" {
		t.Fatalf("expected name match, got %+v", m)
	}
	if m.WebUI != "http://[IP]:[PORT:8989]/" || m.Category != "MediaApp:Video" {
		t.Errorf("template fields lost: %+v", m)
	}
}

func TestMatchByRepositoryIgnoresTag(t *testing.T) {
	a := newTestAdapter(t)
	m := a.Match("my-renamed-sonarr", "lscr.io/linuxserver/sonarr:4.0.0@sha256:abcdef")
	if m == nil || m.File != "my-sonarr.xml" {
		t.Fatalf("expected repository match, got %+v", m)
	}
}

func TestAmbiguousRepositoryDoesNotMatch(t *testing.T) {
	a := newTestAdapter(t)
	// Two templates share lscr.io/linuxserver/radarr; a name miss must not
	// guess between them.
	if m := a.Match("radarr-hdr", "lscr.io/linuxserver/radarr:latest"); m != nil {
		t.Fatalf("ambiguous repository must not match, got %+v", m)
	}
	// But exact names still resolve each template.
	if m := a.Match("radarr-4k", "lscr.io/linuxserver/radarr:latest"); m == nil || m.File != "my-radarr-4k.xml" {
		t.Fatalf("exact name must still match, got %+v", m)
	}
}

func TestMissingDirectoryReportsUnavailable(t *testing.T) {
	a := New("testdata/does-not-exist")
	a.Reload()
	if a.Status().Available {
		t.Fatal("missing directory must mark the source unavailable")
	}
}
