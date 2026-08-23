package model

import "testing"

func TestLinkstackNormalize(t *testing.T) {
	stack := Linkstack{
		Address: "nonsense",
		Entries: []LinkstackEntry{
			{ContainerKey: "/Plex", Address: LinkModeLANIP},
			{ContainerKey: "plex"},                     // duplicate of the entry above
			{ContainerKey: ""},                         // blank keys are not addressable
			{ContainerKey: "sonarr", Address: "smart"}, // not a launcher address
			{ContainerKey: "radarr", Hidden: true},
		},
	}

	got := stack.Normalize()

	if got.Address != LinkModeHostname {
		t.Errorf("unknown page address kept: %q", got.Address)
	}
	if len(got.Entries) != 3 {
		t.Fatalf("expected 3 entries after dedupe, got %d: %+v", len(got.Entries), got.Entries)
	}
	if got.Entries[0].ContainerKey != "plex" {
		t.Errorf("container key not normalized: %q", got.Entries[0].ContainerKey)
	}
	if got.Entries[0].Address != LinkModeLANIP {
		t.Errorf("per-entry address lost: %q", got.Entries[0].Address)
	}
	if got.Entries[1].ContainerKey != "sonarr" || got.Entries[1].Address != "" {
		t.Errorf("unknown entry address kept: %+v", got.Entries[1])
	}
	if !got.Entries[2].Hidden {
		t.Errorf("hidden flag lost: %+v", got.Entries[2])
	}
}

func TestLinkstackAddressFor(t *testing.T) {
	stack := Linkstack{
		Address: LinkModeHostname,
		Entries: []LinkstackEntry{
			{ContainerKey: "plex", Address: LinkModeLANIP},
			{ContainerKey: "sonarr"},
		},
	}

	if got := stack.AddressFor("plex"); got != LinkModeLANIP {
		t.Errorf("per-entry override ignored: %q", got)
	}
	if got := stack.AddressFor("sonarr"); got != LinkModeHostname {
		t.Errorf("entry without an override should inherit the page: %q", got)
	}
	if got := stack.AddressFor("unknown"); got != LinkModeHostname {
		t.Errorf("unlisted container should inherit the page: %q", got)
	}
	if got := (Linkstack{}).AddressFor("plex"); got != LinkModeHostname {
		t.Errorf("empty launcher should fall back to hostname: %q", got)
	}
}
