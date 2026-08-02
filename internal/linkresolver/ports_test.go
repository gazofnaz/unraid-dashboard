package linkresolver

import (
	"testing"

	"github.com/gazofnaz/unraid-dashboard/internal/model"
)

func TestSelectHostBinding(t *testing.T) {
	bindings := []model.PortBinding{
		{ContainerPort: 8080, Protocol: "tcp", HostIP: "0.0.0.0", HostPort: 8081},
		{ContainerPort: 8080, Protocol: "tcp", HostIP: "::", HostPort: 8081},
		{ContainerPort: 9000, Protocol: "tcp", HostIP: "127.0.0.1", HostPort: 9000},
		{ContainerPort: 9000, Protocol: "udp", HostIP: "0.0.0.0", HostPort: 9000},
		{ContainerPort: 7000, Protocol: "tcp", HostIP: "192.168.0.253", HostPort: 7100},
		{ContainerPort: 7000, Protocol: "tcp", HostIP: "0.0.0.0", HostPort: 7200},
	}

	t.Run("wildcard ipv4 preferred over ipv6", func(t *testing.T) {
		choice, ok := SelectHostBinding(bindings, 8080, "tcp", "")
		if !ok || choice.Binding.HostPort != 8081 || choice.Binding.HostIP != "0.0.0.0" {
			t.Fatalf("got %+v ok=%v", choice, ok)
		}
		if choice.LocalOnly {
			t.Error("wildcard binding must not be local-only")
		}
	})

	t.Run("preferred LAN address wins over wildcard", func(t *testing.T) {
		choice, ok := SelectHostBinding(bindings, 7000, "tcp", "192.168.0.253")
		if !ok || choice.Binding.HostPort != 7100 {
			t.Fatalf("expected the 192.168.0.253 binding, got %+v ok=%v", choice, ok)
		}
	})

	t.Run("loopback flagged local-only", func(t *testing.T) {
		choice, ok := SelectHostBinding(bindings, 9000, "tcp", "")
		if !ok || !choice.LocalOnly {
			t.Fatalf("expected local-only loopback, got %+v ok=%v", choice, ok)
		}
	})

	t.Run("udp never yields a web binding", func(t *testing.T) {
		udpOnly := []model.PortBinding{{ContainerPort: 9000, Protocol: "udp", HostIP: "0.0.0.0", HostPort: 9000}}
		if _, ok := SelectHostBinding(udpOnly, 9000, "tcp", ""); ok {
			t.Fatal("udp binding must not satisfy a tcp request")
		}
	})

	t.Run("unpublished port has no binding", func(t *testing.T) {
		if _, ok := SelectHostBinding(bindings, 1234, "tcp", ""); ok {
			t.Fatal("expected no binding")
		}
	})
}

func TestNonHTTPService(t *testing.T) {
	for port, want := range map[int]string{5432: "postgresql", 6379: "redis", 3306: "mysql", 53: "dns", 22: "ssh", 1883: "mqtt"} {
		if got, ok := NonHTTPService(port); !ok || got != want {
			t.Errorf("NonHTTPService(%d) = %q, %v; want %q", port, got, ok, want)
		}
	}
	if _, ok := NonHTTPService(8080); ok {
		t.Error("8080 must not be classified non-HTTP")
	}
}
