// Package linkresolver derives normalized browser endpoints for containers.
// It favors precision over guessing: a missing link is preferable to a
// confident-looking wrong link. Every decision keeps its evidence.
package linkresolver

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gazofnaz/unraid-dashboard/internal/catalog"
	"github.com/gazofnaz/unraid-dashboard/internal/model"
)

// Candidate priorities from DISCOVERY_ENGINE.md §3.
const (
	prioOverride = 100
	prioLabel    = 90
	prioTemplate = 85
	prioProxy    = 75
	prioCatalog  = 60
	prioProbe    = 40
)

// urlLabels are labels whose value is an application URL, highest priority
// first. Only labels that clearly contain a URL pattern are recognized.
var urlLabels = []string{
	"io.arraydeck.url",
	"net.unraid.docker.webui",
	"homepage.href",
	"flame.url",
}

// Inputs is everything the resolver needs for one container.
type Inputs struct {
	Record model.ContainerRecord
	// NetworkTarget is the container whose network namespace this one shares
	// (network_mode: container:<name>), when applicable.
	NetworkTarget *model.ContainerRecord
	Override      *model.Override
	// HostLANIP is the detected Unraid LAN address, used for binding
	// preference and probe targets.
	HostLANIP    string
	ProbeEnabled bool
}

// Resolver generates, scores and ranks endpoint candidates.
type Resolver struct {
	prober Prober
}

// New builds a resolver around a prober.
func New(p Prober) *Resolver {
	if p == nil {
		p = NoopProber{}
	}
	return &Resolver{prober: p}
}

type netClass int

const (
	netBridge netClass = iota
	netHost
	netLAN // macvlan/ipvlan with a LAN-routable container address
)

func (n netClass) String() string {
	switch n {
	case netHost:
		return "host"
	case netLAN:
		return "container-lan"
	default:
		return "bridge"
	}
}

// Resolve produces the full decision for one container.
func (r *Resolver) Resolve(ctx context.Context, in Inputs) model.Decision {
	rec := in.Record
	dec := model.Decision{
		ContainerKey: rec.Key(),
		ContainerID:  rec.ID,
		ResolvedAt:   time.Now().UTC(),
	}

	// network_mode: container:<name> inherits the target's network identity
	// while keeping this container's own WebUI metadata and path.
	netRec := rec
	inherited := ""
	if target := strings.TrimPrefix(rec.NetworkMode, "container:"); target != rec.NetworkMode {
		if in.NetworkTarget != nil {
			netRec.NetworkMode = in.NetworkTarget.NetworkMode
			netRec.Ports = in.NetworkTarget.Ports
			netRec.Networks = in.NetworkTarget.Networks
			inherited = in.NetworkTarget.Name
		}
	}
	class, lanIP := classifyNetwork(netRec, in.HostLANIP)

	var candidates []model.Candidate
	signals := []model.SignalUse{}

	// Priority 100 — user override.
	if in.Override != nil && in.Override.URL != "" {
		if c, err := overrideCandidate(in.Override.URL); err == nil {
			candidates = append(candidates, c)
			signals = append(signals, model.SignalUse{Name: "Manual override", Value: in.Override.URL, Status: "used"})
		} else {
			signals = append(signals, model.SignalUse{Name: "Manual override", Value: in.Override.URL, Status: "rejected"})
		}
	} else {
		signals = append(signals, model.SignalUse{Name: "Manual override", Value: "None", Status: "absent"})
	}

	// Priority 90 — explicit application URL labels.
	labelFound := false
	for _, key := range urlLabels {
		raw, ok := rec.Labels[key]
		if !ok || strings.TrimSpace(raw) == "" {
			continue
		}
		labelFound = true
		c := r.patternCandidate(raw, "label:"+key, prioLabel, class, lanIP, netRec)
		signals = append(signals, model.SignalUse{Name: "Docker label " + key, Value: raw, Status: signalStatus(c)})
		candidates = append(candidates, c)
	}
	if !labelFound {
		signals = append(signals, model.SignalUse{Name: "Docker label net.unraid.docker.webui", Status: "absent"})
	}

	// Priority 85 — dockerMan template <WebUI>.
	if t := rec.Template; t != nil && t.WebUI != "" {
		c := r.patternCandidate(t.WebUI, "template:"+t.File, prioTemplate, class, lanIP, netRec)
		signals = append(signals, model.SignalUse{Name: "Template <WebUI> (" + t.File + ")", Value: t.WebUI, Status: signalStatus(c)})
		candidates = append(candidates, c)
	} else if rec.Template != nil {
		signals = append(signals, model.SignalUse{Name: "Template " + rec.Template.File, Value: "no <WebUI>", Status: "present"})
	} else {
		signals = append(signals, model.SignalUse{Name: "Template <WebUI>", Status: "absent"})
	}

	// Priority 75 — reverse-proxy routing labels.
	proxyHosts := proxyHostRules(rec.Labels)
	for _, ph := range proxyHosts {
		c := model.Candidate{
			Priority: prioProxy,
			Source:   ph.source,
			Endpoint: model.Endpoint{
				Scheme:        "https",
				AddressPolicy: model.PolicyExplicitHost,
				ExplicitHost:  ph.host,
				Path:          "/",
				Source:        ph.source,
				Confidence:    0.88,
			},
			Explanation: "Reverse-proxy route " + ph.host + " configured via " + ph.source,
			Evidence:    []string{ph.rule},
		}
		signals = append(signals, model.SignalUse{Name: "Reverse-proxy labels", Value: ph.host, Status: "used"})
		candidates = append(candidates, c)
	}
	if len(proxyHosts) == 0 {
		signals = append(signals, model.SignalUse{Name: "Reverse-proxy labels", Status: "absent"})
	}

	// Priority 60 — known application catalog.
	imageBase := catalog.ImageBase(catalog.NormalizeImageRef(rec.Image))
	if entry := catalog.Lookup(imageBase, rec.Key()); entry != nil && entry.Port > 0 {
		c := r.catalogCandidate(entry, class, lanIP, netRec)
		signals = append(signals, model.SignalUse{Name: "App catalog", Value: entry.App, Status: signalStatus(c)})
		candidates = append(candidates, c)
	} else if entry != nil {
		signals = append(signals, model.SignalUse{Name: "App catalog", Value: entry.App + " (no web port)", Status: "present"})
	} else {
		signals = append(signals, model.SignalUse{Name: "App catalog", Status: "absent"})
	}

	// Priority 40 — published HTTP-port inference, confirmed by probe only.
	candidates = append(candidates, r.portInference(class, lanIP, netRec, candidates)...)

	portSummary := summarizePorts(netRec)
	signals = append(signals, model.SignalUse{Name: "Published ports", Value: portSummary, Status: presentAbsent(portSummary != "none")})

	// Probe confirmation for locally-addressable, not-yet-rejected candidates.
	probed := r.probeCandidates(ctx, in, class, lanIP, candidates)
	signals = append(signals, model.SignalUse{Name: "HTTP probe", Value: probed, Status: presentAbsent(probed != "not attempted")})

	// Finalize probe-only inference: anything a probe did not confirm is
	// rejected with an explanation rather than left dangling.
	for i := range candidates {
		c := &candidates[i]
		if c.Priority != prioProbe || c.Rejected || c.Endpoint.Confidence >= 0.5 {
			continue
		}
		c.Rejected = true
		if svc, bad := NonHTTPService(c.Endpoint.ContainerPort); bad {
			c.RejectReason = fmt.Sprintf("port %d is associated with %s, not HTTP", c.Endpoint.ContainerPort, svc)
		} else if c.Probe != nil && c.Probe.Attempted {
			c.RejectReason = "probe found no HTTP service: " + probeSummary(*c.Probe)
		} else {
			c.RejectReason = "no probe confirmation for inferred port"
		}
	}

	// Stamp identities and apply dismissals.
	for i := range candidates {
		candidates[i].Identity = CandidateIdentity(candidates[i])
		if in.Override != nil {
			for _, d := range in.Override.DismissedURLs {
				if d == candidates[i].Identity {
					candidates[i].Dismissed = true
				}
			}
		}
	}

	// Rank: priority, then confidence, then probe success.
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.Priority != b.Priority {
			return a.Priority > b.Priority
		}
		if a.Endpoint.Confidence != b.Endpoint.Confidence {
			return a.Endpoint.Confidence > b.Endpoint.Confidence
		}
		return probeOK(a) && !probeOK(b)
	})

	for i := range candidates {
		c := &candidates[i]
		if !c.Rejected && !c.Dismissed && c.Endpoint.Confidence >= 0.5 {
			dec.Winner = c
			break
		}
	}

	dec.Candidates = candidates
	dec.Signals = signals
	dec.Steps = buildSteps(dec, class, inherited, in)
	return dec
}

// classifyNetwork decides the address policy family for a container. A LAN
// classification requires a private, LAN-routable container address — either
// on the host's own /24 or on an Unraid-style macvlan parent (br0, bond0,
// eth0) — and specifically not a Docker internal 172.16/12 bridge address.
func classifyNetwork(rec model.ContainerRecord, hostLANIP string) (netClass, string) {
	mode := rec.NetworkMode
	if mode == "host" {
		return netHost, ""
	}
	if mode == "none" || mode == "bridge" || mode == "default" || strings.HasPrefix(mode, "container:") {
		return netBridge, ""
	}
	for _, att := range rec.Networks {
		ip := net.ParseIP(att.IPAddress)
		if ip == nil || ip.To4() == nil || !ip.IsPrivate() || inDockerRange(ip) {
			continue
		}
		if sameSubnet24(att.IPAddress, hostLANIP) || lanBridgeName(att.Name) {
			return netLAN, att.IPAddress
		}
	}
	return netBridge, ""
}

func inDockerRange(ip net.IP) bool {
	_, dockerNet, _ := net.ParseCIDR("172.16.0.0/12")
	return dockerNet.Contains(ip)
}

func sameSubnet24(a, b string) bool {
	ipa, ipb := net.ParseIP(a), net.ParseIP(b)
	if ipa == nil || ipb == nil || ipa.To4() == nil || ipb.To4() == nil {
		return false
	}
	return ipa.Mask(net.CIDRMask(24, 32)).Equal(ipb.Mask(net.CIDRMask(24, 32)))
}

func lanBridgeName(name string) bool {
	return strings.HasPrefix(name, "br") || strings.HasPrefix(name, "bond") || strings.HasPrefix(name, "eth")
}

// patternCandidate resolves a WebUI pattern against the container's actual
// networking, per DISCOVERY_ENGINE.md §4–6.
func (r *Resolver) patternCandidate(raw, source string, priority int, class netClass, lanIP string, netRec model.ContainerRecord) model.Candidate {
	c := model.Candidate{Priority: priority, Source: source}
	pattern, err := ParseWebUI(raw)
	if err != nil {
		c.Rejected = true
		c.RejectReason = "unparseable pattern: " + err.Error()
		c.Explanation = fmt.Sprintf("%s could not be parsed", raw)
		return c
	}
	ep := model.Endpoint{Scheme: pattern.Scheme, Path: pattern.Path, Source: source}
	c.Evidence = append(c.Evidence, "pattern "+raw)

	if pattern.ExplicitHost != "" {
		// Explicit hostnames are preserved for Smart mode.
		ep.AddressPolicy = model.PolicyExplicitHost
		ep.ExplicitHost = pattern.ExplicitHost
		if pattern.PortKind == PortContainerToken {
			if choice, ok := SelectHostBinding(netRec.Ports, pattern.Port, "tcp", lanIP); ok {
				ep.ContainerPort = pattern.Port
				ep.HostPort = choice.Binding.HostPort
			} else {
				ep.HostPort = pattern.Port
			}
		} else if pattern.PortKind == PortLiteral {
			ep.HostPort = pattern.Port
		}
		c.Explanation = "Explicit host " + pattern.ExplicitHost + " from " + source
	} else {
		switch class {
		case netHost:
			// Host networking: mappings are meaningless, the metadata port is
			// already a host port.
			ep.AddressPolicy = model.PolicyUnraidHost
			if pattern.PortKind != PortNone {
				ep.HostPort = pattern.Port
				ep.ContainerPort = pattern.Port
			}
			c.Evidence = append(c.Evidence, "host networking: pattern port used as host port")
		case netLAN:
			// The container has its own LAN address; ports are not remapped.
			ep.AddressPolicy = model.PolicyContainerLANIP
			ep.ExplicitHost = lanIP
			if pattern.PortKind != PortNone {
				ep.ContainerPort = pattern.Port
				ep.HostPort = pattern.Port
			}
			c.Evidence = append(c.Evidence, "container has LAN address "+lanIP)
		default: // bridge
			ep.AddressPolicy = model.PolicyUnraidHost
			switch pattern.PortKind {
			case PortContainerToken:
				choice, ok := SelectHostBinding(netRec.Ports, pattern.Port, "tcp", lanIP)
				if !ok {
					c.Rejected = true
					c.RejectReason = fmt.Sprintf("container port %d/tcp is not published", pattern.Port)
					c.Endpoint = ep
					c.Explanation = fmt.Sprintf("%s references container port %d which has no host mapping", source, pattern.Port)
					return c
				}
				ep.ContainerPort = pattern.Port
				ep.HostPort = choice.Binding.HostPort
				c.Evidence = append(c.Evidence, fmt.Sprintf("[PORT:%d] → host port %d", pattern.Port, choice.Binding.HostPort))
				if choice.LocalOnly {
					// Loopback bindings are unreachable for remote browsers:
					// keep the candidate visible but below auto-selection.
					c.Evidence = append(c.Evidence, "binding is loopback-only (127.0.0.1)")
					ep.Confidence = 0.45
				}
			case PortLiteral:
				// Prefer an exact host-port binding, then treat the literal
				// as a container port and map it.
				if hasHostPort(netRec.Ports, pattern.Port) {
					ep.HostPort = pattern.Port
					c.Evidence = append(c.Evidence, fmt.Sprintf("host port %d is published", pattern.Port))
				} else if choice, ok := SelectHostBinding(netRec.Ports, pattern.Port, "tcp", lanIP); ok {
					ep.ContainerPort = pattern.Port
					ep.HostPort = choice.Binding.HostPort
					c.Evidence = append(c.Evidence, fmt.Sprintf("port %d mapped to host port %d", pattern.Port, choice.Binding.HostPort))
				} else {
					c.Rejected = true
					c.RejectReason = fmt.Sprintf("port %d is not published", pattern.Port)
					c.Endpoint = ep
					c.Explanation = fmt.Sprintf("%s references port %d which is not published", source, pattern.Port)
					return c
				}
			case PortNone:
				c.Rejected = true
				c.RejectReason = "pattern has no port and container is bridged; internal IPs are not offered"
				c.Endpoint = ep
				c.Explanation = "No port in pattern for a bridged container"
				return c
			}
		}
		c.Explanation = fmt.Sprintf("WebUI pattern from %s resolved via %s networking", source, class)
	}

	if ep.Confidence == 0 {
		if priority == prioLabel {
			ep.Confidence = 0.95
		} else {
			ep.Confidence = 0.92
		}
	}
	c.Endpoint = ep
	return c
}

func hasHostPort(bindings []model.PortBinding, hostPort int) bool {
	for _, b := range bindings {
		if b.HostPort == hostPort && b.Protocol == "tcp" && !isLoopback(b.HostIP) {
			return true
		}
	}
	return false
}

// catalogCandidate builds the known-application fallback candidate.
func (r *Resolver) catalogCandidate(entry *catalog.Entry, class netClass, lanIP string, netRec model.ContainerRecord) model.Candidate {
	scheme := entry.Scheme
	if scheme == "" {
		scheme = "http"
	}
	path := entry.Path
	if path == "" {
		path = "/"
	}
	c := model.Candidate{
		Priority: prioCatalog,
		Source:   "catalog:" + strings.ToLower(entry.App),
		Evidence: []string{fmt.Sprintf("%s default web port %d", entry.App, entry.Port)},
	}
	ep := model.Endpoint{Scheme: scheme, Path: path, Source: c.Source}

	switch class {
	case netHost:
		ep.AddressPolicy = model.PolicyUnraidHost
		ep.HostPort = entry.Port
		ep.ContainerPort = entry.Port
	case netLAN:
		ep.AddressPolicy = model.PolicyContainerLANIP
		ep.ExplicitHost = lanIP
		ep.HostPort = entry.Port
		ep.ContainerPort = entry.Port
	default:
		choice, ok := SelectHostBinding(netRec.Ports, entry.Port, "tcp", lanIP)
		if !ok {
			c.Rejected = true
			c.RejectReason = fmt.Sprintf("catalog port %d/tcp is not published", entry.Port)
			c.Explanation = fmt.Sprintf("%s normally serves on %d, but that port is not published", entry.App, entry.Port)
			c.Endpoint = ep
			return c
		}
		ep.AddressPolicy = model.PolicyUnraidHost
		ep.ContainerPort = entry.Port
		ep.HostPort = choice.Binding.HostPort
		c.Evidence = append(c.Evidence, fmt.Sprintf("published as host port %d", choice.Binding.HostPort))
	}
	// Base confidence without probe confirmation; probeCandidates raises it.
	ep.Confidence = 0.65
	c.Explanation = fmt.Sprintf("Known application %s, default port %d", entry.App, entry.Port)
	c.Endpoint = ep
	return c
}

// portInference proposes probe-confirmed candidates from plausible published
// HTTP ports, skipping ports already covered by stronger candidates and
// well-known non-HTTP service ports.
func (r *Resolver) portInference(class netClass, lanIP string, netRec model.ContainerRecord, existing []model.Candidate) []model.Candidate {
	if class == netHost {
		// Host mode publishes nothing; inference would be guessing.
		return nil
	}
	covered := map[int]bool{}
	for _, c := range existing {
		if !c.Rejected && c.Endpoint.HostPort > 0 {
			covered[c.Endpoint.HostPort] = true
		}
	}
	var out []model.Candidate
	seen := map[int]bool{}
	for _, b := range netRec.Ports {
		if b.Protocol != "tcp" || b.HostPort == 0 || covered[b.HostPort] || seen[b.HostPort] {
			continue
		}
		seen[b.HostPort] = true
		c := model.Candidate{
			Priority: prioProbe,
			Source:   "probe:port-inference",
			Evidence: []string{fmt.Sprintf("published port %d/tcp → host port %d", b.ContainerPort, b.HostPort)},
		}
		ep := model.Endpoint{
			Scheme:        schemeForPort(b.ContainerPort),
			AddressPolicy: model.PolicyUnraidHost,
			ContainerPort: b.ContainerPort,
			HostPort:      b.HostPort,
			Path:          "/",
			Source:        c.Source,
		}
		if class == netLAN {
			ep.AddressPolicy = model.PolicyContainerLANIP
			ep.ExplicitHost = lanIP
		}
		// All inference candidates start below the selection threshold and
		// only a successful probe lifts them; ports conventionally used by
		// non-HTTP services start at zero and need the same confirmation.
		if svc, bad := NonHTTPService(b.ContainerPort); bad {
			ep.Confidence = 0
			c.Evidence = append(c.Evidence, fmt.Sprintf("port %d is conventionally %s", b.ContainerPort, svc))
			c.Explanation = fmt.Sprintf("Port %d looks like %s; only a confirming HTTP probe can offer it", b.ContainerPort, svc)
		} else {
			ep.Confidence = 0.30
			if IsLikelyWebPort(b.ContainerPort) {
				c.Evidence = append(c.Evidence, fmt.Sprintf("%d is a common web port", b.ContainerPort))
			}
			c.Explanation = fmt.Sprintf("Published TCP port %d might serve HTTP; requires probe confirmation", b.HostPort)
		}
		c.Endpoint = ep
		out = append(out, c)
	}
	return out
}

func schemeForPort(port int) string {
	if port == 443 || port == 8443 {
		return "https"
	}
	return "http"
}

// probeCandidates probes locally-addressable candidates for a running
// container and adjusts confidence per the scoring table. Explicit hosts are
// never probed: they may resolve to public networks.
func (r *Resolver) probeCandidates(ctx context.Context, in Inputs, class netClass, lanIP string, candidates []model.Candidate) string {
	if !in.ProbeEnabled {
		return "disabled"
	}
	if in.Record.State != model.StateRunning {
		return "skipped: container not running"
	}
	attempted := 0
	confirmed := 0
	for i := range candidates {
		c := &candidates[i]
		if c.Rejected || c.Priority == prioOverride {
			continue
		}
		var host string
		switch c.Endpoint.AddressPolicy {
		case model.PolicyUnraidHost:
			host = in.HostLANIP
		case model.PolicyContainerLANIP:
			host = lanIP
		default:
			continue // never probe explicit hosts
		}
		if host == "" || attempted >= 4 {
			continue
		}
		target := c.Endpoint.Scheme + "://" + host
		if c.Endpoint.HostPort > 0 {
			target += fmt.Sprintf(":%d", c.Endpoint.HostPort)
		}
		target += c.Endpoint.Path
		attempted++
		res := r.prober.Probe(ctx, target)
		c.Probe = &res
		if !res.Attempted {
			continue
		}
		if res.OK {
			confirmed++
			switch c.Priority {
			case prioLabel:
				c.Endpoint.Confidence = 0.98
			case prioCatalog:
				c.Endpoint.Confidence = 0.75
			case prioProbe:
				c.Endpoint.Confidence = 0.60
			}
			c.Evidence = append(c.Evidence, "HTTP probe succeeded ("+res.StatusClass+")")
		} else {
			c.Evidence = append(c.Evidence, "HTTP probe inconclusive: "+probeSummary(res))
		}
	}
	if attempted == 0 {
		return "not attempted"
	}
	return fmt.Sprintf("%d target(s) probed, %d confirmed", attempted, confirmed)
}

func probeSummary(res model.ProbeResult) string {
	if res.Error != "" {
		return res.Error
	}
	return res.StatusClass
}

// overrideCandidate parses a user-supplied full URL into an explicit endpoint.
func overrideCandidate(raw string) (model.Candidate, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return model.Candidate{}, fmt.Errorf("override must be a full http(s) URL")
	}
	port := 0
	if p := u.Port(); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}
	path := u.Path
	if path == "" {
		path = "/"
	}
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	return model.Candidate{
		Priority: prioOverride,
		Source:   "override",
		Endpoint: model.Endpoint{
			Scheme:        u.Scheme,
			AddressPolicy: model.PolicyExplicitHost,
			ExplicitHost:  u.Hostname(),
			HostPort:      port,
			Path:          path,
			Confidence:    1.0,
			Source:        "override",
		},
		Explanation: "Manual override set by the user",
		Evidence:    []string{"override " + raw},
	}, nil
}

type proxyHost struct {
	host   string
	source string
	rule   string
}

// proxyHostRules extracts explicit hosts from common reverse-proxy labels
// (traefik Host(...) rules and caddy address labels).
func proxyHostRules(labels map[string]string) []proxyHost {
	var out []proxyHost
	for key, val := range labels {
		if strings.HasPrefix(key, "traefik.http.routers.") && strings.HasSuffix(key, ".rule") {
			for _, h := range extractTraefikHosts(val) {
				out = append(out, proxyHost{host: h, source: "proxy:traefik", rule: key + "=" + val})
			}
		}
		if key == "caddy" {
			h := strings.TrimSpace(val)
			h = strings.TrimPrefix(h, "https://")
			h = strings.TrimPrefix(h, "http://")
			if h != "" && !strings.ContainsAny(h, " {}*") {
				out = append(out, proxyHost{host: h, source: "proxy:caddy", rule: "caddy=" + val})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].host < out[j].host })
	return out
}

func extractTraefikHosts(rule string) []string {
	var hosts []string
	rest := rule
	for {
		idx := strings.Index(rest, "Host(`")
		if idx < 0 {
			break
		}
		rest = rest[idx+len("Host(`"):]
		end := strings.Index(rest, "`")
		if end < 0 {
			break
		}
		host := rest[:end]
		if host != "" {
			hosts = append(hosts, host)
		}
		rest = rest[end:]
	}
	return hosts
}

// CandidateIdentity is the stable identity used for dismissals.
func CandidateIdentity(c model.Candidate) string {
	e := c.Endpoint
	return fmt.Sprintf("%s|%s|%s|%s|%d|%s", c.Source, e.Scheme, e.AddressPolicy, e.ExplicitHost, e.HostPort, e.Path)
}

func probeOK(c model.Candidate) bool {
	return c.Probe != nil && c.Probe.OK
}

func signalStatus(c model.Candidate) string {
	if c.Rejected {
		return "rejected"
	}
	return "used"
}

func presentAbsent(present bool) string {
	if present {
		return "present"
	}
	return "absent"
}

func summarizePorts(rec model.ContainerRecord) string {
	if len(rec.Ports) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(rec.Ports))
	for _, p := range rec.Ports {
		host := p.HostIP
		if host == "" {
			host = "0.0.0.0"
		}
		parts = append(parts, fmt.Sprintf("%d/%s → %s:%d", p.ContainerPort, p.Protocol, host, p.HostPort))
	}
	if len(parts) > 6 {
		parts = append(parts[:6], fmt.Sprintf("+%d more", len(parts)-6))
	}
	return strings.Join(parts, ", ")
}

// buildSteps renders the ordered, human-readable resolution trace shown in
// the discovery inspector.
func buildSteps(dec model.Decision, class netClass, inheritedFrom string, in Inputs) []model.TraceStep {
	var steps []model.TraceStep
	if inheritedFrom != "" {
		steps = append(steps, model.TraceStep{
			Title:  "Inherit network namespace",
			Detail: "network_mode container:" + inheritedFrom + " — addresses and ports come from that container.",
			Value:  inheritedFrom,
		})
	}
	w := dec.Winner
	if w == nil {
		reason := "No source produced a selectable endpoint."
		for _, c := range dec.Candidates {
			if c.Rejected && c.RejectReason != "" {
				reason = "Best candidate rejected: " + c.RejectReason
				break
			}
		}
		steps = append(steps, model.TraceStep{Title: "Collect endpoint sources", Detail: reason})
		steps = append(steps, model.TraceStep{Title: "Result", Detail: "No web interface detected.", Value: "no endpoint"})
		return steps
	}

	steps = append(steps, model.TraceStep{
		Title:  "Read " + sourceTitle(w.Source),
		Detail: sourceDetail(w.Source),
		Value:  firstEvidence(w),
	})
	if w.Endpoint.ContainerPort > 0 && w.Endpoint.HostPort > 0 && w.Endpoint.ContainerPort != w.Endpoint.HostPort {
		steps = append(steps, model.TraceStep{
			Title:  "Resolve the mapped port",
			Detail: fmt.Sprintf("Container port %d is published as host port %d.", w.Endpoint.ContainerPort, w.Endpoint.HostPort),
			Value:  fmt.Sprintf("[PORT:%d] → %d", w.Endpoint.ContainerPort, w.Endpoint.HostPort),
		})
	} else if w.Endpoint.HostPort > 0 {
		steps = append(steps, model.TraceStep{
			Title:  "Resolve the port",
			Detail: fmt.Sprintf("Port %d is used directly (%s networking).", w.Endpoint.HostPort, class),
			Value:  fmt.Sprintf("%d", w.Endpoint.HostPort),
		})
	}
	switch w.Endpoint.AddressPolicy {
	case model.PolicyUnraidHost:
		steps = append(steps, model.TraceStep{
			Title:  "Resolve the address token",
			Detail: fmt.Sprintf("%s networking uses the Unraid host address, not the internal container IP.", capitalize(class.String())),
			Value:  "[IP] → Unraid host address",
		})
	case model.PolicyContainerLANIP:
		steps = append(steps, model.TraceStep{
			Title:  "Resolve the address token",
			Detail: "The container has its own LAN-routable address on a custom network.",
			Value:  "[IP] → " + w.Endpoint.ExplicitHost,
		})
	case model.PolicyExplicitHost:
		steps = append(steps, model.TraceStep{
			Title:  "Preserve the explicit host",
			Detail: "The source names a specific host; Smart mode keeps it.",
			Value:  w.Endpoint.ExplicitHost,
		})
	}
	if w.Probe != nil && w.Probe.Attempted {
		detail := "A lightweight HTTP probe returned a valid response."
		if !w.Probe.OK {
			detail = "The HTTP probe did not confirm the endpoint: " + probeSummary(*w.Probe) + "."
		}
		steps = append(steps, model.TraceStep{Title: "Confirm service reachability", Detail: detail, Value: probeSummary(*w.Probe)})
	} else if !in.ProbeEnabled {
		steps = append(steps, model.TraceStep{Title: "Confirm service reachability", Detail: "Probing is disabled in settings."})
	}
	return steps
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func firstEvidence(c *model.Candidate) string {
	if len(c.Evidence) > 0 {
		return strings.TrimPrefix(c.Evidence[0], "pattern ")
	}
	return ""
}

func sourceTitle(source string) string {
	switch {
	case strings.HasPrefix(source, "label:net.unraid"):
		return "Unraid WebUI metadata"
	case strings.HasPrefix(source, "label:"):
		return "application URL label"
	case strings.HasPrefix(source, "template:"):
		return "dockerMan template <WebUI>"
	case strings.HasPrefix(source, "proxy:"):
		return "reverse-proxy route"
	case strings.HasPrefix(source, "catalog:"):
		return "application catalog entry"
	case source == "override":
		return "manual override"
	default:
		return "published ports"
	}
}

func sourceDetail(source string) string {
	switch {
	case strings.HasPrefix(source, "label:net.unraid"):
		return "Highest-priority source because it reflects the configured application entry point."
	case strings.HasPrefix(source, "template:"):
		return "The Unraid template's WebUI pattern is used as a fallback to Docker labels."
	case strings.HasPrefix(source, "proxy:"):
		return "A reverse proxy explicitly routes a hostname to this container."
	case strings.HasPrefix(source, "catalog:"):
		return "A curated catalog knows this application's default web port."
	case source == "override":
		return "A manual override always wins; discovered candidates remain listed."
	default:
		return "A published TCP port answered an HTTP probe."
	}
}
