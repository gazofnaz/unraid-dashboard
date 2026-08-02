package store

import (
	"path/filepath"
	"testing"

	"github.com/gazofnaz/unraid-dashboard/internal/model"
)

func TestPersistenceSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "arraydeck.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	settings := model.DefaultSettings()
	settings.LinkMode = model.LinkModeLANIP
	settings.ServerHostname = "tower.lan"
	if err := s.SaveSettings(settings); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	hidden := true
	if err := s.SaveOverride("qbittorrent", model.Override{URL: "https://qb.home.example/", Hidden: &hidden}); err != nil {
		t.Fatalf("save override: %v", err)
	}

	dec := model.Decision{ContainerKey: "qbittorrent", ContainerID: "abc"}
	if err := s.SaveDecision("qbittorrent", dec); err != nil {
		t.Fatalf("save decision: %v", err)
	}

	if err := s.IconPut("icon1", "image/png", []byte{1, 2, 3}, ""); err != nil {
		t.Fatalf("icon put: %v", err)
	}
	s.Close()

	// Reopen: everything must still be there (container recreation survives).
	s2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()

	got, found, err := s2.Settings()
	if err != nil || !found {
		t.Fatalf("settings after reopen: found=%v err=%v", found, err)
	}
	if got.LinkMode != model.LinkModeLANIP || got.ServerHostname != "tower.lan" {
		t.Errorf("settings lost: %+v", got)
	}

	overrides, err := s2.Overrides()
	if err != nil {
		t.Fatal(err)
	}
	o, ok := overrides["qbittorrent"]
	if !ok || o.URL != "https://qb.home.example/" || o.Hidden == nil || !*o.Hidden {
		t.Errorf("override lost: %+v", o)
	}

	decisions, err := s2.Decisions()
	if err != nil {
		t.Fatal(err)
	}
	if d, ok := decisions["qbittorrent"]; !ok || d.ContainerID != "abc" {
		t.Errorf("decision lost: %+v", d)
	}

	if mime, body, ok := s2.IconGet("icon1"); !ok || mime != "image/png" || len(body) != 3 {
		t.Errorf("icon cache lost: %s %v %v", mime, body, ok)
	}

	// Deleting an override works and persists.
	if err := s2.DeleteOverride("qbittorrent"); err != nil {
		t.Fatal(err)
	}
	overrides, _ = s2.Overrides()
	if _, ok := overrides["qbittorrent"]; ok {
		t.Error("override not deleted")
	}
}
