package classifier

import (
	"testing"

	"github.com/gazofnaz/unraid-dashboard/internal/model"
)

func TestNormalizeUnraidCategory(t *testing.T) {
	cases := map[string]string{
		"MediaApp:Video":                  "Media",
		"MediaServer:Video MediaApp:Music": "Media",
		"Downloaders:Other":               "Downloads",
		"HomeAutomation:":                 "Home",
		"Network:Proxy":                   "Network",
		"Backup:":                         "Storage",
		"Tools:System":                    "Infrastructure",
		"Security:Authentication":         "Security",
		"Productivity:":                   "Other",
		"":                                "",
		"SomethingUnknown:Weird":          "",
	}
	for raw, want := range cases {
		if got := NormalizeUnraidCategory(raw); got != want {
			t.Errorf("NormalizeUnraidCategory(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestClassifyPriorityOrder(t *testing.T) {
	rec := model.ContainerRecord{
		Name:  "sonarr",
		Image: "lscr.io/linuxserver/sonarr:latest",
		Template: &model.TemplateMeta{
			File: "my-sonarr.xml", Category: "Downloaders:Other",
		},
	}

	t.Run("override wins over everything", func(t *testing.T) {
		hidden := model.Override{Category: "media"}
		group, source := Classify(rec, &hidden)
		if group != "Media" || source != "override" {
			t.Errorf("got %q from %q", group, source)
		}
	})

	t.Run("unraid template category preferred", func(t *testing.T) {
		group, source := Classify(rec, nil)
		if group != "Downloads" || source != "unraid-template" {
			t.Errorf("got %q from %q", group, source)
		}
	})

	t.Run("catalog when no template category", func(t *testing.T) {
		noTemplate := rec
		noTemplate.Template = nil
		group, source := Classify(noTemplate, nil)
		if group != "Automation" || source != "catalog" {
			t.Errorf("got %q from %q", group, source)
		}
	})

	t.Run("name token fallback", func(t *testing.T) {
		unknown := model.ContainerRecord{Name: "my-gitea-fork", Image: "example/custom-gitea"}
		group, source := Classify(unknown, nil)
		if group != "Development" || source != "name-token" {
			t.Errorf("got %q from %q", group, source)
		}
	})

	t.Run("port fingerprint fallback", func(t *testing.T) {
		db := model.ContainerRecord{
			Name: "mystery", Image: "example/mystery",
			Ports: []model.PortBinding{{ContainerPort: 5432, Protocol: "tcp", HostPort: 5432}},
		}
		group, source := Classify(db, nil)
		if group != "Databases" || source != "port-fingerprint" {
			t.Errorf("got %q from %q", group, source)
		}
	})

	t.Run("other as last resort", func(t *testing.T) {
		mystery := model.ContainerRecord{Name: "zzz", Image: "example/zzz"}
		group, source := Classify(mystery, nil)
		if group != "Other" || source != "default" {
			t.Errorf("got %q from %q", group, source)
		}
	})
}
