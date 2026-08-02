// Package catalog is a small curated map of well-known self-hosted
// applications to their default web port, path and category. It is a
// fallback ranking signal only — live port mappings always take precedence.
package catalog

import "strings"

// Entry describes one known application.
type Entry struct {
	App      string
	Aliases  []string // normalized image basenames and common container names
	Port     int      // default internal (container) web port; 0 = none known
	Path     string
	Scheme   string
	Category string
}

var entries = []Entry{
	{App: "Plex", Aliases: []string{"plex", "pms-docker", "plexmediaserver"}, Port: 32400, Path: "/web", Category: "Media"},
	{App: "Jellyfin", Aliases: []string{"jellyfin"}, Port: 8096, Category: "Media"},
	{App: "Emby", Aliases: []string{"emby", "embyserver"}, Port: 8096, Category: "Media"},
	{App: "Immich", Aliases: []string{"immich", "immich-server", "immich-app"}, Port: 2283, Category: "Media"},
	{App: "PhotoPrism", Aliases: []string{"photoprism"}, Port: 2342, Category: "Media"},
	{App: "Navidrome", Aliases: []string{"navidrome"}, Port: 4533, Category: "Media"},
	{App: "Audiobookshelf", Aliases: []string{"audiobookshelf"}, Port: 80, Category: "Media"},
	{App: "Calibre-Web", Aliases: []string{"calibre-web"}, Port: 8083, Category: "Media"},
	{App: "Tdarr", Aliases: []string{"tdarr"}, Port: 8265, Category: "Media"},

	{App: "Sonarr", Aliases: []string{"sonarr"}, Port: 8989, Category: "Automation"},
	{App: "Radarr", Aliases: []string{"radarr"}, Port: 7878, Category: "Automation"},
	{App: "Lidarr", Aliases: []string{"lidarr"}, Port: 8686, Category: "Automation"},
	{App: "Readarr", Aliases: []string{"readarr"}, Port: 8787, Category: "Automation"},
	{App: "Prowlarr", Aliases: []string{"prowlarr"}, Port: 9696, Category: "Automation"},
	{App: "Bazarr", Aliases: []string{"bazarr"}, Port: 6767, Category: "Automation"},
	{App: "Overseerr", Aliases: []string{"overseerr"}, Port: 5055, Category: "Automation"},
	{App: "Jellyseerr", Aliases: []string{"jellyseerr"}, Port: 5055, Category: "Automation"},
	{App: "Tautulli", Aliases: []string{"tautulli"}, Port: 8181, Category: "Automation"},

	{App: "qBittorrent", Aliases: []string{"qbittorrent"}, Port: 8080, Category: "Downloads"},
	{App: "Transmission", Aliases: []string{"transmission"}, Port: 9091, Category: "Downloads"},
	{App: "Deluge", Aliases: []string{"deluge"}, Port: 8112, Category: "Downloads"},
	{App: "SABnzbd", Aliases: []string{"sabnzbd"}, Port: 8080, Category: "Downloads"},
	{App: "NZBGet", Aliases: []string{"nzbget"}, Port: 6789, Category: "Downloads"},

	{App: "Home Assistant", Aliases: []string{"home-assistant", "homeassistant"}, Port: 8123, Category: "Home"},
	{App: "ESPHome", Aliases: []string{"esphome"}, Port: 6052, Category: "Home"},
	{App: "Zigbee2MQTT", Aliases: []string{"zigbee2mqtt"}, Port: 8080, Category: "Home"},
	{App: "Node-RED", Aliases: []string{"node-red", "nodered"}, Port: 1880, Category: "Home"},

	{App: "Pi-hole", Aliases: []string{"pihole"}, Port: 80, Path: "/admin", Category: "Network"},
	{App: "AdGuard Home", Aliases: []string{"adguardhome", "adguard-home"}, Port: 80, Category: "Network"},
	{App: "Nginx Proxy Manager", Aliases: []string{"nginx-proxy-manager", "nginxproxymanager"}, Port: 81, Category: "Network"},
	{App: "Traefik", Aliases: []string{"traefik"}, Port: 8080, Category: "Network"},
	{App: "SWAG", Aliases: []string{"swag"}, Port: 80, Category: "Network"},
	{App: "Unifi Controller", Aliases: []string{"unifi-controller", "unifi-network-application"}, Port: 8443, Scheme: "https", Category: "Network"},

	{App: "Vaultwarden", Aliases: []string{"vaultwarden"}, Port: 80, Category: "Security"},
	{App: "Authelia", Aliases: []string{"authelia"}, Port: 9091, Category: "Security"},
	{App: "Authentik", Aliases: []string{"authentik"}, Port: 9000, Category: "Security"},

	{App: "Portainer", Aliases: []string{"portainer", "portainer-ce"}, Port: 9000, Category: "Infrastructure"},
	{App: "Dozzle", Aliases: []string{"dozzle"}, Port: 8080, Category: "Infrastructure"},
	{App: "Watchtower", Aliases: []string{"watchtower"}, Category: "Infrastructure"},
	{App: "Diun", Aliases: []string{"diun"}, Category: "Infrastructure"},

	{App: "Grafana", Aliases: []string{"grafana"}, Port: 3000, Category: "Monitoring"},
	{App: "Prometheus", Aliases: []string{"prometheus"}, Port: 9090, Category: "Monitoring"},
	{App: "Uptime Kuma", Aliases: []string{"uptime-kuma"}, Port: 3001, Category: "Monitoring"},
	{App: "Netdata", Aliases: []string{"netdata"}, Port: 19999, Category: "Monitoring"},
	{App: "Scrutiny", Aliases: []string{"scrutiny", "scrutiny-web"}, Port: 8080, Category: "Monitoring"},

	{App: "Nextcloud", Aliases: []string{"nextcloud"}, Port: 80, Category: "Storage"},
	{App: "Syncthing", Aliases: []string{"syncthing"}, Port: 8384, Category: "Storage"},
	{App: "Duplicati", Aliases: []string{"duplicati"}, Port: 8200, Category: "Storage"},
	{App: "MinIO", Aliases: []string{"minio"}, Port: 9001, Category: "Storage"},
	{App: "FileBrowser", Aliases: []string{"filebrowser"}, Port: 80, Category: "Storage"},
	{App: "Paperless-ngx", Aliases: []string{"paperless-ngx", "paperless-ngx-server"}, Port: 8000, Category: "Storage"},

	{App: "code-server", Aliases: []string{"code-server"}, Port: 8443, Scheme: "https", Category: "Development"},
	{App: "Gitea", Aliases: []string{"gitea"}, Port: 3000, Category: "Development"},

	{App: "PostgreSQL", Aliases: []string{"postgres", "postgresql"}, Category: "Databases"},
	{App: "MariaDB", Aliases: []string{"mariadb"}, Category: "Databases"},
	{App: "MySQL", Aliases: []string{"mysql"}, Category: "Databases"},
	{App: "Redis", Aliases: []string{"redis"}, Category: "Databases"},
	{App: "MongoDB", Aliases: []string{"mongo", "mongodb"}, Category: "Databases"},
	{App: "InfluxDB", Aliases: []string{"influxdb"}, Category: "Databases"},

	{App: "Homarr", Aliases: []string{"homarr"}, Port: 7575, Category: "Infrastructure"},
	{App: "Heimdall", Aliases: []string{"heimdall"}, Port: 80, Category: "Infrastructure"},
	{App: "Homer", Aliases: []string{"homer"}, Port: 8080, Category: "Infrastructure"},
	{App: "Dashy", Aliases: []string{"dashy"}, Port: 8080, Category: "Infrastructure"},
	{App: "FreshRSS", Aliases: []string{"freshrss"}, Port: 80, Category: "Other"},
}

var index = buildIndex()

func buildIndex() map[string]*Entry {
	m := make(map[string]*Entry)
	for i := range entries {
		for _, alias := range entries[i].Aliases {
			m[alias] = &entries[i]
		}
	}
	return m
}

// Lookup matches by normalized image basename first, then container name.
func Lookup(imageBase, containerName string) *Entry {
	if e, ok := index[strings.ToLower(imageBase)]; ok {
		return e
	}
	if e, ok := index[strings.ToLower(containerName)]; ok {
		return e
	}
	return nil
}

// ImageBase extracts the final path segment of an image reference already
// stripped of tag and digest, e.g. `ghcr.io/hotio/qbittorrent` → `qbittorrent`.
func ImageBase(normalizedRef string) string {
	if i := strings.LastIndex(normalizedRef, "/"); i >= 0 {
		return normalizedRef[i+1:]
	}
	return normalizedRef
}

// NormalizeImageRef strips tag/digest and default registry prefixes so
// `lscr.io/linuxserver/sonarr:latest` and `lscr.io/linuxserver/sonarr@sha256:x`
// compare equal, and `library/nginx` equals `nginx`.
func NormalizeImageRef(ref string) string {
	ref = strings.TrimSpace(strings.ToLower(ref))
	if ref == "" {
		return ""
	}
	if at := strings.Index(ref, "@"); at >= 0 {
		ref = ref[:at]
	}
	// Strip the tag: the last colon, but only after the final slash so
	// registry ports (host:5000/img) survive.
	if colon := strings.LastIndex(ref, ":"); colon > strings.LastIndex(ref, "/") {
		ref = ref[:colon]
	}
	ref = strings.TrimPrefix(ref, "docker.io/")
	ref = strings.TrimPrefix(ref, "library/")
	return ref
}
