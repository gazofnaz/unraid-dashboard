package linkresolver

import "testing"

func TestParseWebUI(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr bool
		want    WebUIPattern
	}{
		{
			name: "standard unraid pattern",
			raw:  "http://[IP]:[PORT:8080]/",
			want: WebUIPattern{Scheme: "http", HostIsToken: true, PortKind: PortContainerToken, Port: 8080, Path: "/"},
		},
		{
			name: "https with path",
			raw:  "https://[IP]:[PORT:32400]/web",
			want: WebUIPattern{Scheme: "https", HostIsToken: true, PortKind: PortContainerToken, Port: 32400, Path: "/web"},
		},
		{
			name: "no scheme defaults to http",
			raw:  "[IP]:[PORT:8123]",
			want: WebUIPattern{Scheme: "http", HostIsToken: true, PortKind: PortContainerToken, Port: 8123, Path: "/"},
		},
		{
			name: "literal port",
			raw:  "http://[IP]:8080/admin",
			want: WebUIPattern{Scheme: "http", HostIsToken: true, PortKind: PortLiteral, Port: 8080, Path: "/admin"},
		},
		{
			name: "ip token without port",
			raw:  "http://[IP]/",
			want: WebUIPattern{Scheme: "http", HostIsToken: true, PortKind: PortNone, Path: "/"},
		},
		{
			name: "explicit host",
			raw:  "https://photos.home.example/",
			want: WebUIPattern{Scheme: "https", ExplicitHost: "photos.home.example", PortKind: PortNone, Path: "/"},
		},
		{
			name: "explicit host with port token",
			raw:  "https://nas.lan:[PORT:8443]/ui",
			want: WebUIPattern{Scheme: "https", ExplicitHost: "nas.lan", PortKind: PortContainerToken, Port: 8443, Path: "/ui"},
		},
		{
			name: "fragment preserved",
			raw:  "http://[IP]:[PORT:2283]/#/login",
			want: WebUIPattern{Scheme: "http", HostIsToken: true, PortKind: PortContainerToken, Port: 2283, Path: "/#/login"},
		},
		{name: "empty", raw: "  ", wantErr: true},
		{name: "bad scheme", raw: "ftp://[IP]:21/", wantErr: true},
		{name: "bad port token", raw: "http://[IP]:[PORT:abc]/", wantErr: true},
		{name: "port out of range", raw: "http://[IP]:99999/", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseWebUI(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Scheme != tc.want.Scheme || got.HostIsToken != tc.want.HostIsToken ||
				got.ExplicitHost != tc.want.ExplicitHost || got.PortKind != tc.want.PortKind ||
				got.Port != tc.want.Port || got.Path != tc.want.Path {
				t.Errorf("ParseWebUI(%q)\n got %+v\nwant %+v", tc.raw, *got, tc.want)
			}
		})
	}
}
