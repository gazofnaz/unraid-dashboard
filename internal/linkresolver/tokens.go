package linkresolver

import (
	"fmt"
	"strconv"
	"strings"
)

// PortKind classifies the port component of a WebUI pattern.
type PortKind int

const (
	// PortNone means the pattern carries no port (scheme default applies).
	PortNone PortKind = iota
	// PortContainerToken is `[PORT:n]`: n is a container port that must be
	// mapped through the actual published bindings.
	PortContainerToken
	// PortLiteral is a plain numeric port written into the pattern.
	PortLiteral
)

// WebUIPattern is a parsed Unraid/dockerMan WebUI value such as
// `http://[IP]:[PORT:8080]/web`. Unraid token syntax breaks net/url, so the
// pattern is parsed by hand.
type WebUIPattern struct {
	Raw          string
	Scheme       string
	HostIsToken  bool   // host was the `[IP]` token
	ExplicitHost string // set when the pattern names a real host
	PortKind     PortKind
	Port         int
	Path         string
}

// ParseWebUI parses a WebUI pattern. It accepts absent schemes (http is
// assumed) and preserves path, query and fragment.
func ParseWebUI(raw string) (*WebUIPattern, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, fmt.Errorf("empty pattern")
	}
	p := &WebUIPattern{Raw: raw, Scheme: "http", Path: "/"}

	if scheme, rest, found := strings.Cut(s, "://"); found {
		scheme = strings.ToLower(scheme)
		if scheme != "http" && scheme != "https" {
			return nil, fmt.Errorf("unsupported scheme %q", scheme)
		}
		p.Scheme = scheme
		s = rest
	}

	hostport := s
	if slash := strings.IndexAny(s, "/?#"); slash >= 0 {
		hostport = s[:slash]
		p.Path = s[slash:]
		if !strings.HasPrefix(p.Path, "/") {
			p.Path = "/" + p.Path
		}
	}
	if hostport == "" {
		return nil, fmt.Errorf("no host in pattern %q", raw)
	}

	// Split host from port, respecting the [IP] and [PORT:n] token forms.
	host := hostport
	portPart := ""
	if strings.HasPrefix(hostport, "[IP]") {
		host = "[IP]"
		rest := strings.TrimPrefix(hostport, "[IP]")
		if rest != "" {
			if !strings.HasPrefix(rest, ":") {
				return nil, fmt.Errorf("malformed pattern %q", raw)
			}
			portPart = rest[1:]
		}
	} else if colon := strings.LastIndex(hostport, ":"); colon >= 0 && !strings.Contains(hostport, "]") {
		host, portPart = hostport[:colon], hostport[colon+1:]
	} else if colon := strings.Index(hostport, ":["); colon >= 0 {
		host, portPart = hostport[:colon], hostport[colon+1:]
	}

	if host == "[IP]" {
		p.HostIsToken = true
	} else {
		p.ExplicitHost = host
	}

	switch {
	case portPart == "":
		p.PortKind = PortNone
	case strings.HasPrefix(portPart, "[PORT:") && strings.HasSuffix(portPart, "]"):
		n, err := strconv.Atoi(portPart[len("[PORT:") : len(portPart)-1])
		if err != nil || n <= 0 || n > 65535 {
			return nil, fmt.Errorf("bad port token %q", portPart)
		}
		p.PortKind = PortContainerToken
		p.Port = n
	default:
		n, err := strconv.Atoi(portPart)
		if err != nil || n <= 0 || n > 65535 {
			return nil, fmt.Errorf("bad port %q", portPart)
		}
		p.PortKind = PortLiteral
		p.Port = n
	}
	return p, nil
}
