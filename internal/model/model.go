// Package model defines the normalized data types shared by the discovery
// sources, link resolver, classifier, store and API.
package model

import (
	"strings"
	"time"
)

// Container states as reported by Docker.
const (
	StateRunning    = "running"
	StateExited     = "exited"
	StatePaused     = "paused"
	StateRestarting = "restarting"
	StateCreated    = "created"
	StateDead       = "dead"
)

// Address policies for normalized endpoints.
const (
	PolicyUnraidHost     = "unraid-host"
	PolicyContainerLANIP = "container-lan-ip"
	PolicyExplicitHost   = "explicit-host"
)

// Link modes selectable in the UI.
const (
	LinkModeHostname = "hostname"
	LinkModeLANIP    = "lan-ip"
	LinkModeSmart    = "smart"
)

// PortBinding is one published mapping from a container port to a host port.
type PortBinding struct {
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol"`
	HostIP        string `json:"hostIp"`
	HostPort      int    `json:"hostPort"`
}

// ExposedPort is a port declared by the image but not necessarily published.
type ExposedPort struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}

// NetworkAttachment is one network the container is attached to.
type NetworkAttachment struct {
	Name      string `json:"name"`
	IPAddress string `json:"ipAddress,omitempty"`
}

// TemplateMeta is dockerMan template metadata matched to a container.
type TemplateMeta struct {
	File       string `json:"file"`
	Name       string `json:"name"`
	Repository string `json:"repository"`
	WebUI      string `json:"webUi,omitempty"`
	Icon       string `json:"icon,omitempty"`
	Category   string `json:"category,omitempty"`
	Project    string `json:"project,omitempty"`
}

// ContainerRecord is the normalized merge of all sources for one container.
type ContainerRecord struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Image       string              `json:"image"`
	State       string              `json:"state"`
	Health      string              `json:"health,omitempty"`
	NetworkMode string              `json:"networkMode"`
	Labels      map[string]string   `json:"labels,omitempty"`
	Ports       []PortBinding       `json:"ports,omitempty"`
	Exposed     []ExposedPort       `json:"exposed,omitempty"`
	Networks    []NetworkAttachment `json:"networks,omitempty"`
	CreatedAt   time.Time           `json:"createdAt"`
	StartedAt   time.Time           `json:"startedAt,omitempty"`
	IsSelf      bool                `json:"isSelf,omitempty"`
	Template    *TemplateMeta       `json:"template,omitempty"`
}

// Key returns the stable identity used for overrides and decisions. Container
// IDs change on recreate, names normally survive it.
func (c *ContainerRecord) Key() string {
	return ContainerKey(c.Name)
}

// ContainerKey normalizes a container name into an override/decision key.
func ContainerKey(name string) string {
	return strings.ToLower(strings.TrimPrefix(name, "/"))
}

// Endpoint is a normalized browser endpoint. The address is a policy, not a
// final hostname, so switching link modes never requires rediscovery.
type Endpoint struct {
	Scheme        string  `json:"scheme"`
	AddressPolicy string  `json:"addressPolicy"`
	ExplicitHost  string  `json:"explicitHost,omitempty"`
	ContainerPort int     `json:"containerPort,omitempty"`
	HostPort      int     `json:"hostPort,omitempty"`
	Path          string  `json:"path"`
	Confidence    float64 `json:"confidence"`
	Source        string  `json:"source"`
}

// ProbeResult stores only the response class and shape, never content.
type ProbeResult struct {
	Attempted   bool          `json:"attempted"`
	OK          bool          `json:"ok"`
	StatusClass string        `json:"statusClass,omitempty"` // 2xx, 3xx, 4xx, 5xx
	StatusCode  int           `json:"statusCode,omitempty"`
	RedirectTo  string        `json:"redirectTo,omitempty"`
	ContentType string        `json:"contentType,omitempty"`
	Elapsed     time.Duration `json:"elapsedNs,omitempty"`
	Error       string        `json:"error,omitempty"`
}

// Candidate is one possible endpoint with evidence for the inspector.
type Candidate struct {
	Endpoint     Endpoint     `json:"endpoint"`
	Priority     int          `json:"priority"`
	Source       string       `json:"source"`
	Evidence     []string     `json:"evidence,omitempty"`
	Explanation  string       `json:"explanation"`
	Probe        *ProbeResult `json:"probe,omitempty"`
	Rejected     bool         `json:"rejected,omitempty"`
	RejectReason string       `json:"rejectReason,omitempty"`
	Dismissed    bool         `json:"dismissed,omitempty"`
	// Identity is the stable string the UI passes back to dismiss this
	// candidate via an override.
	Identity string `json:"identity,omitempty"`
}

// TraceStep is one ordered step in the resolution trace.
type TraceStep struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Value  string `json:"value,omitempty"`
}

// SignalUse reports whether an input signal existed and whether it was used.
type SignalUse struct {
	Name   string `json:"name"`
	Value  string `json:"value,omitempty"`
	Status string `json:"status"` // used | present | absent | rejected
}

// Decision is the stored, explainable outcome of resolving one container.
type Decision struct {
	ContainerKey string      `json:"containerKey"`
	ContainerID  string      `json:"containerId"`
	Winner       *Candidate  `json:"winner,omitempty"`
	Candidates   []Candidate `json:"candidates,omitempty"`
	Steps        []TraceStep `json:"steps,omitempty"`
	Signals      []SignalUse `json:"signals,omitempty"`
	ResolvedAt   time.Time   `json:"resolvedAt"`
}

// Override holds user exceptions layered over discovery. All fields optional.
type Override struct {
	URL           string   `json:"url,omitempty"`
	Name          string   `json:"name,omitempty"`
	Icon          string   `json:"icon,omitempty"`
	Category      string   `json:"category,omitempty"`
	Hidden        *bool    `json:"hidden,omitempty"`
	Favorite      *bool    `json:"favorite,omitempty"`
	DismissedURLs []string `json:"dismissedUrls,omitempty"`
}

// IsZero reports whether the override carries no user choices.
func (o Override) IsZero() bool {
	return o.URL == "" && o.Name == "" && o.Icon == "" && o.Category == "" &&
		o.Hidden == nil && o.Favorite == nil && len(o.DismissedURLs) == 0
}

// Settings is the persisted global configuration.
type Settings struct {
	ServerHostname     string `json:"serverHostname"`
	LANAddress         string `json:"lanAddress"`
	PreferredInterface string `json:"preferredInterface"`
	LinkMode           string `json:"linkMode"`
	ShowStopped        bool   `json:"showStopped"`
	ShowSelf           bool   `json:"showSelf"`
	OpenStoppedLinks   bool   `json:"openStoppedLinks"`
	ProbeEnabled       bool   `json:"probeEnabled"`
	Theme              string `json:"theme"` // light | dark | system
}

// DefaultSettings returns the initial configuration for a fresh install.
func DefaultSettings() Settings {
	return Settings{
		LinkMode:         LinkModeHostname,
		ShowStopped:      true,
		ShowSelf:         false,
		OpenStoppedLinks: false,
		ProbeEnabled:     true,
		Theme:            "light",
	}
}

// SourceStatus describes the availability of one discovery source.
type SourceStatus struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Detail    string `json:"detail,omitempty"`
	Error     string `json:"error,omitempty"`
}

// HostIdentity is the resolved server hostname and LAN address, with
// provenance and all discovered candidates.
type HostIdentity struct {
	Hostname       string   `json:"hostname,omitempty"`
	HostnameSource string   `json:"hostnameSource,omitempty"`
	LANAddress     string   `json:"lanAddress,omitempty"`
	LANSource      string   `json:"lanSource,omitempty"`
	Candidates     []string `json:"candidates,omitempty"`
}

// ServerInfo is optional Unraid metadata for the summary cards.
type ServerInfo struct {
	Name          string `json:"name,omitempty"`
	UnraidVersion string `json:"unraidVersion,omitempty"`
	UptimeSeconds int64  `json:"uptimeSeconds,omitempty"`
}

// LinkstackEntry is one row of the launcher page. It is keyed by container
// key rather than ID so a curated order survives container recreation.
type LinkstackEntry struct {
	ContainerKey string `json:"containerKey"`
	// Address overrides the page-wide address form for this link only.
	// Empty inherits Linkstack.Address.
	Address string `json:"address,omitempty"`
	Hidden  bool   `json:"hidden,omitempty"`
}

// Linkstack is the curated launcher: an explicit order over the containers
// that resolved to a URL, plus the address form their links use. Containers
// absent from Entries are appended alphabetically, so newly deployed
// applications appear on the page without being added by hand.
type Linkstack struct {
	Address string           `json:"address"` // hostname | lan-ip
	Entries []LinkstackEntry `json:"entries"`
}

// DefaultLinkstack returns the launcher for a fresh install: nothing curated
// yet, links addressed by hostname.
func DefaultLinkstack() Linkstack {
	return Linkstack{Address: LinkModeHostname, Entries: []LinkstackEntry{}}
}

// Normalize drops blank and duplicate keys and replaces unknown address forms
// with the inherited default, so a malformed request cannot corrupt the page.
func (l Linkstack) Normalize() Linkstack {
	out := Linkstack{Address: l.Address, Entries: make([]LinkstackEntry, 0, len(l.Entries))}
	if !isLinkstackAddress(out.Address) {
		out.Address = LinkModeHostname
	}
	seen := make(map[string]bool, len(l.Entries))
	for _, e := range l.Entries {
		key := ContainerKey(e.ContainerKey)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		if !isLinkstackAddress(e.Address) {
			e.Address = ""
		}
		e.ContainerKey = key
		out.Entries = append(out.Entries, e)
	}
	return out
}

// AddressFor returns the address form one container's link should render with.
func (l Linkstack) AddressFor(containerKey string) string {
	for _, e := range l.Entries {
		if e.ContainerKey == containerKey && e.Address != "" {
			return e.Address
		}
	}
	if isLinkstackAddress(l.Address) {
		return l.Address
	}
	return LinkModeHostname
}

func isLinkstackAddress(s string) bool {
	return s == LinkModeHostname || s == LinkModeLANIP
}
