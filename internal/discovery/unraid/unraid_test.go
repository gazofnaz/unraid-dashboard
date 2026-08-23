package unraid

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeServer answers GraphQL queries by keyword, so a test can disable
// introspection independently of the fields it serves.
func fakeServer(t *testing.T, handle func(query string) (status int, body string)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var req gqlRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Errorf("malformed request body: %v", err)
		}
		status, body := handle(req.Query)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
}

const introspectionDisabled = `{"errors":[{"message":"GraphQL introspection is not allowed, but the current request is for introspection.","extensions":{"code":"INTROSPECTION_DISABLED"}}]}`

// Apollo disables introspection in production builds. The server still answers
// ordinary queries, so the adapter must stay available on those alone.
func TestRefreshSurvivesDisabledIntrospection(t *testing.T) {
	srv := fakeServer(t, func(q string) (int, string) {
		switch {
		case strings.Contains(q, "__schema"):
			return http.StatusBadRequest, introspectionDisabled
		case strings.Contains(q, "info"):
			return http.StatusOK, `{"data":{"info":{"os":{"hostname":"Tower"},"versions":{"unraid":"7.0.0"}}}}`
		case strings.Contains(q, "network"):
			return http.StatusOK, `{"data":{"network":{"iface":[{"ifaceName":"br0","ipv4":"192.168.1.10/24"}]}}}`
		}
		return http.StatusOK, `{"data":{}}`
	})
	defer srv.Close()

	a := New(srv.URL, "unraid_secret")
	a.Refresh(context.Background())

	st := a.Status()
	if !st.Available {
		t.Fatalf("want available, got %+v", st)
	}
	if st.Detail != "connected, introspection disabled" {
		t.Errorf("detail = %q", st.Detail)
	}
	id := a.Identity()
	if id.Hostname != "Tower" {
		t.Errorf("hostname = %q, want Tower", id.Hostname)
	}
	if id.UnraidVersion != "7.0.0" {
		t.Errorf("version = %q, want 7.0.0", id.UnraidVersion)
	}
	if len(id.LANAddresses) != 1 || id.LANAddresses[0] != "192.168.1.10" {
		t.Errorf("LAN addresses = %v", id.LANAddresses)
	}
}

// A non-200 must carry the server's explanation; a bare status code is
// indistinguishable from a rejected key.
func TestStatusErrorCarriesServerReason(t *testing.T) {
	srv := fakeServer(t, func(string) (int, string) {
		return http.StatusBadRequest, introspectionDisabled
	})
	defer srv.Close()

	a := New(srv.URL, "unraid_secret")
	a.Refresh(context.Background())

	st := a.Status()
	if st.Available {
		t.Fatal("want unavailable when every query fails")
	}
	if !strings.Contains(st.Error, "introspection is not allowed") {
		t.Errorf("error lacks server reason: %q", st.Error)
	}
	if !strings.Contains(st.Error, "400") {
		t.Errorf("error lacks status: %q", st.Error)
	}
}

func TestStatusRedactsAPIKey(t *testing.T) {
	srv := fakeServer(t, func(string) (int, string) {
		return http.StatusUnauthorized, `{"errors":[{"message":"bad key unraid_secret"}]}`
	})
	defer srv.Close()

	a := New(srv.URL, "unraid_secret")
	a.Refresh(context.Background())

	if st := a.Status(); strings.Contains(st.Error, "unraid_secret") {
		t.Errorf("api key leaked into status: %q", st.Error)
	}
}

// With introspection allowed the capability map still gates queries, so a
// field the schema omits is never requested.
func TestIntrospectionGatesUnknownFields(t *testing.T) {
	var asked []string
	srv := fakeServer(t, func(q string) (int, string) {
		switch {
		case strings.Contains(q, "__schema"):
			return http.StatusOK, `{"data":{"__schema":{"queryType":{"fields":[{"name":"info"}]}}}}`
		case strings.Contains(q, "info"):
			asked = append(asked, "info")
			return http.StatusOK, `{"data":{"info":{"os":{"hostname":"Tower"}}}}`
		case strings.Contains(q, "network"):
			asked = append(asked, "network")
			return http.StatusOK, `{"data":{}}`
		}
		return http.StatusOK, `{"data":{}}`
	})
	defer srv.Close()

	a := New(srv.URL, "")
	a.Refresh(context.Background())

	for _, q := range asked {
		if q == "network" {
			t.Error("queried network despite the schema omitting it")
		}
	}
	if st := a.Status(); st.Detail != "connected, 1 query capabilities" {
		t.Errorf("detail = %q", st.Detail)
	}
}

func TestNilAdapterNotConfigured(t *testing.T) {
	a := New("", "key")
	if a != nil {
		t.Fatal("empty endpoint should yield a nil adapter")
	}
	if st := a.Status(); st.Available || st.Detail != "not configured" {
		t.Errorf("status = %+v", st)
	}
}
