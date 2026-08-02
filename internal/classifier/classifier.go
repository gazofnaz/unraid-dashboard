// Package classifier maps containers to top-level UI groups using rule-based,
// inspectable steps: Unraid category metadata first, then labels, the app
// catalog, name/image tokens, port fingerprints, and finally Other.
package classifier

import (
	"strings"

	"github.com/gazofnaz/unraid-dashboard/internal/catalog"
	"github.com/gazofnaz/unraid-dashboard/internal/model"
)

// Groups is the fixed set of top-level UI groups from DISCOVERY_ENGINE.md §10.
var Groups = []string{
	"Media", "Downloads", "Automation", "Home", "Network", "Security",
	"Development", "Storage", "Monitoring", "Databases", "Infrastructure", "Other",
}

var validGroup = func() map[string]bool {
	m := map[string]bool{}
	for _, g := range Groups {
		m[g] = true
	}
	return m
}()

// unraidPrefixes maps dockerMan category prefixes to UI groups.
var unraidPrefixes = map[string]string{
	"mediaapp":       "Media",
	"mediaserver":    "Media",
	"downloaders":    "Downloads",
	"homeautomation": "Home",
	"network":        "Network",
	"security":       "Security",
	"backup":         "Storage",
	"cloud":          "Storage",
	"tools":          "Infrastructure",
	"drivers":        "Infrastructure",
	"productivity":   "Other",
	"gameservers":    "Other",
	"crypto":         "Other",
	"ai":             "Other",
	"other":          "Other",
}

// tokenRules map name/image substrings to groups. First match wins; order is
// most-specific first.
var tokenRules = []struct {
	Group  string
	Tokens []string
}{
	{"Automation", []string{"sonarr", "radarr", "lidarr", "readarr", "prowlarr", "bazarr", "overseerr", "jellyseerr", "ombi", "tautulli", "autobrr", "cross-seed"}},
	{"Databases", []string{"postgres", "mariadb", "mysql", "redis", "mongo", "influxdb", "elasticsearch", "memcached", "clickhouse"}},
	{"Media", []string{"plex", "jellyfin", "emby", "immich", "photoprism", "navidrome", "audiobookshelf", "calibre", "kavita", "komga", "tdarr", "unmanic"}},
	{"Downloads", []string{"qbittorrent", "transmission", "deluge", "sabnzbd", "nzbget", "rutorrent", "rtorrent", "slskd"}},
	{"Home", []string{"homeassistant", "home-assistant", "esphome", "zigbee2mqtt", "zwave", "mosquitto", "mqtt", "node-red", "nodered", "frigate"}},
	{"Network", []string{"pihole", "adguard", "unbound", "wireguard", "tailscale", "traefik", "caddy", "swag", "haproxy", "cloudflared", "unifi", "omada", "nginx"}},
	{"Security", []string{"vaultwarden", "bitwarden", "authelia", "authentik", "keycloak", "crowdsec"}},
	{"Development", []string{"gitea", "gitlab", "jenkins", "code-server", "drone", "woodpecker", "verdaccio"}},
	{"Storage", []string{"nextcloud", "syncthing", "duplicati", "restic", "kopia", "minio", "filebrowser", "seafile", "paperless", "rclone"}},
	{"Monitoring", []string{"grafana", "prometheus", "telegraf", "uptime-kuma", "netdata", "scrutiny", "librespeed", "speedtest", "glances", "beszel"}},
	{"Infrastructure", []string{"portainer", "watchtower", "dozzle", "diun", "homarr", "heimdall", "homer", "dashy", "organizr", "flame"}},
}

// portGroups fingerprint well-known service ports.
var portGroups = map[int]string{
	5432: "Databases", 3306: "Databases", 6379: "Databases", 27017: "Databases",
	9200: "Databases", 8086: "Databases",
	53:   "Network",
	1883: "Home",
}

// Classify returns the primary group and the rule source that produced it.
func Classify(rec model.ContainerRecord, override *model.Override) (string, string) {
	if override != nil && override.Category != "" {
		return sanitize(override.Category), "override"
	}
	if rec.Template != nil && rec.Template.Category != "" {
		if g := NormalizeUnraidCategory(rec.Template.Category); g != "" {
			return g, "unraid-template"
		}
	}
	if v := rec.Labels["io.arraydeck.category"]; v != "" {
		return sanitize(v), "label"
	}
	imageBase := catalog.ImageBase(catalog.NormalizeImageRef(rec.Image))
	if entry := catalog.Lookup(imageBase, rec.Key()); entry != nil && entry.Category != "" {
		return entry.Category, "catalog"
	}
	haystack := strings.ToLower(rec.Name + " " + rec.Image)
	for _, rule := range tokenRules {
		for _, tok := range rule.Tokens {
			if strings.Contains(haystack, tok) {
				return rule.Group, "name-token"
			}
		}
	}
	ports := map[int]bool{}
	for _, p := range rec.Ports {
		ports[p.ContainerPort] = true
	}
	for _, p := range rec.Exposed {
		ports[p.Port] = true
	}
	for port, group := range portGroups {
		if ports[port] {
			return group, "port-fingerprint"
		}
	}
	return "Other", "default"
}

// NormalizeUnraidCategory maps strings like `MediaApp:Video Downloaders:Other`
// to the first recognizable top-level group.
func NormalizeUnraidCategory(raw string) string {
	for _, field := range strings.Fields(raw) {
		prefix := field
		if i := strings.IndexAny(field, ":-"); i > 0 {
			prefix = field[:i]
		}
		if g, ok := unraidPrefixes[strings.ToLower(strings.TrimSpace(prefix))]; ok {
			return g
		}
	}
	return ""
}

// sanitize keeps user-provided categories within the known set, preserving
// unknown labels as-is but trimmed (custom groups are allowed).
func sanitize(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "Other"
	}
	for _, g := range Groups {
		if strings.EqualFold(g, s) {
			return g
		}
	}
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

// IsKnownGroup reports whether g is one of the standard groups.
func IsKnownGroup(g string) bool { return validGroup[g] }
