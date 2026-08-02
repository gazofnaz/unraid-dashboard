package docker

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gazofnaz/unraid-dashboard/internal/model"
)

// InspectResponse mirrors the subset of `GET /containers/{id}/json` ArrayDeck reads.
type InspectResponse struct {
	ID      string `json:"Id"`
	Name    string `json:"Name"`
	Created string `json:"Created"`
	State   struct {
		Status    string `json:"Status"`
		StartedAt string `json:"StartedAt"`
		Health    *struct {
			Status string `json:"Status"`
		} `json:"Health"`
	} `json:"State"`
	Config struct {
		Image        string              `json:"Image"`
		Labels       map[string]string   `json:"Labels"`
		ExposedPorts map[string]struct{} `json:"ExposedPorts"`
	} `json:"Config"`
	HostConfig struct {
		NetworkMode string `json:"NetworkMode"`
	} `json:"HostConfig"`
	NetworkSettings struct {
		Ports map[string][]struct {
			HostIP   string `json:"HostIp"`
			HostPort string `json:"HostPort"`
		} `json:"Ports"`
		Networks map[string]struct {
			IPAddress string `json:"IPAddress"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
}

// Normalize converts a Docker inspect document into the shared record shape.
// selfHint is the running process's hostname; inside a container that is the
// short container ID, which lets ArrayDeck recognize itself without name
// special-casing.
func Normalize(doc *InspectResponse, selfHint string) model.ContainerRecord {
	rec := model.ContainerRecord{
		ID:          doc.ID,
		Name:        strings.TrimPrefix(doc.Name, "/"),
		Image:       doc.Config.Image,
		State:       doc.State.Status,
		NetworkMode: doc.HostConfig.NetworkMode,
		Labels:      doc.Config.Labels,
	}
	if doc.State.Health != nil {
		rec.Health = doc.State.Health.Status
	}
	rec.CreatedAt = parseTime(doc.Created)
	rec.StartedAt = parseTime(doc.State.StartedAt)
	if selfHint != "" && len(selfHint) >= 12 && strings.HasPrefix(doc.ID, selfHint) {
		rec.IsSelf = true
	}

	for portProto, bindings := range doc.NetworkSettings.Ports {
		port, proto := splitPortProto(portProto)
		if port == 0 {
			continue
		}
		if len(bindings) == 0 {
			rec.Exposed = append(rec.Exposed, model.ExposedPort{Port: port, Protocol: proto})
			continue
		}
		for _, b := range bindings {
			hostPort, _ := strconv.Atoi(b.HostPort)
			rec.Ports = append(rec.Ports, model.PortBinding{
				ContainerPort: port,
				Protocol:      proto,
				HostIP:        b.HostIP,
				HostPort:      hostPort,
			})
		}
	}
	for portProto := range doc.Config.ExposedPorts {
		port, proto := splitPortProto(portProto)
		if port != 0 && !hasPort(rec, port, proto) {
			rec.Exposed = append(rec.Exposed, model.ExposedPort{Port: port, Protocol: proto})
		}
	}
	sort.Slice(rec.Ports, func(i, j int) bool {
		return rec.Ports[i].ContainerPort < rec.Ports[j].ContainerPort
	})
	sort.Slice(rec.Exposed, func(i, j int) bool {
		return rec.Exposed[i].Port < rec.Exposed[j].Port
	})

	for name, netw := range doc.NetworkSettings.Networks {
		rec.Networks = append(rec.Networks, model.NetworkAttachment{Name: name, IPAddress: netw.IPAddress})
	}
	sort.Slice(rec.Networks, func(i, j int) bool { return rec.Networks[i].Name < rec.Networks[j].Name })
	return rec
}

func hasPort(rec model.ContainerRecord, port int, proto string) bool {
	for _, p := range rec.Ports {
		if p.ContainerPort == port && p.Protocol == proto {
			return true
		}
	}
	for _, p := range rec.Exposed {
		if p.Port == port && p.Protocol == proto {
			return true
		}
	}
	return false
}

func splitPortProto(s string) (int, string) {
	port, proto, ok := strings.Cut(s, "/")
	if !ok {
		proto = "tcp"
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return 0, proto
	}
	return n, proto
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil || t.Year() <= 1 {
		return time.Time{}
	}
	return t
}
