package linkresolver

import (
	"github.com/gazofnaz/unraid-dashboard/internal/model"
)

// nonHTTPPorts are ports strongly associated with non-HTTP services. They are
// never auto-linked unless an HTTP probe confirms a web interface.
var nonHTTPPorts = map[int]string{
	22:   "ssh",
	25:   "smtp",
	53:   "dns",
	465:  "smtp",
	587:  "smtp",
	1883: "mqtt",
	3306: "mysql",
	5432: "postgresql",
	6379: "redis",
}

// likelyWebPorts influence ranking of probe-based inference. Presence in this
// list never creates a link by itself; a probe must confirm.
var likelyWebPorts = map[int]bool{
	80: true, 443: true, 3000: true, 5000: true, 8000: true, 8080: true,
	8081: true, 8096: true, 8123: true, 8181: true, 8443: true, 8989: true,
}

// BindingChoice is the outcome of selecting a host binding for a container port.
type BindingChoice struct {
	Binding   model.PortBinding
	LocalOnly bool // bound to loopback: unreachable for remote browsers
}

// SelectHostBinding picks the best published host binding for a container
// port. Preference: the selected LAN address, wildcard IPv4, wildcard IPv6,
// any other specific address, and loopback last (flagged LocalOnly).
// Protocol must match; UDP never yields a web binding here.
func SelectHostBinding(bindings []model.PortBinding, containerPort int, proto, preferredAddr string) (BindingChoice, bool) {
	if proto == "" {
		proto = "tcp"
	}
	var matches []model.PortBinding
	for _, b := range bindings {
		if b.ContainerPort == containerPort && b.Protocol == proto && b.HostPort > 0 {
			matches = append(matches, b)
		}
	}
	if len(matches) == 0 {
		return BindingChoice{}, false
	}
	rank := func(b model.PortBinding) int {
		switch {
		case preferredAddr != "" && b.HostIP == preferredAddr:
			return 0
		case b.HostIP == "" || b.HostIP == "0.0.0.0":
			return 1
		case b.HostIP == "::":
			return 2
		case isLoopback(b.HostIP):
			return 4
		default:
			return 3
		}
	}
	best := matches[0]
	for _, m := range matches[1:] {
		if rank(m) < rank(best) {
			best = m
		}
	}
	return BindingChoice{Binding: best, LocalOnly: isLoopback(best.HostIP)}, true
}

func isLoopback(ip string) bool {
	return ip == "127.0.0.1" || ip == "::1" || ip == "localhost"
}

// NonHTTPService returns the service name for a port known to be non-HTTP.
func NonHTTPService(port int) (string, bool) {
	name, ok := nonHTTPPorts[port]
	return name, ok
}

// IsLikelyWebPort reports whether a port commonly serves HTTP.
func IsLikelyWebPort(port int) bool {
	return likelyWebPorts[port]
}
