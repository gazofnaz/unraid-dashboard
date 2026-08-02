// Package templates parses Unraid dockerMan user templates read-only and
// matches them to containers. The flash device is never written.
package templates

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gazofnaz/unraid-dashboard/internal/catalog"
	"github.com/gazofnaz/unraid-dashboard/internal/model"
)

// Template is the subset of a dockerMan XML template ArrayDeck reads.
type Template struct {
	XMLName    xml.Name `xml:"Container"`
	Name       string   `xml:"Name"`
	Repository string   `xml:"Repository"`
	WebUI      string   `xml:"WebUI"`
	Icon       string   `xml:"Icon"`
	Category   string   `xml:"Category"`
	Project    string   `xml:"Project"`
	File       string   `xml:"-"`
}

// Adapter loads templates from a directory and answers match queries.
type Adapter struct {
	dir       string
	templates []Template
	err       error
}

// New creates an adapter rooted at dir (normally /unraid/templates).
func New(dir string) *Adapter {
	return &Adapter{dir: dir}
}

// Reload re-parses every XML file in the directory. Individual malformed
// files are skipped; only a missing/unreadable directory marks the source
// unavailable.
func (a *Adapter) Reload() {
	entries, err := os.ReadDir(a.dir)
	if err != nil {
		a.templates, a.err = nil, err
		return
	}
	var parsed []Template
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".xml") {
			continue
		}
		path := filepath.Join(a.dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var t Template
		if err := xml.Unmarshal(raw, &t); err != nil {
			continue
		}
		t.File = entry.Name()
		parsed = append(parsed, t)
	}
	a.templates, a.err = parsed, nil
}

// Status reports source availability for the UI.
func (a *Adapter) Status() model.SourceStatus {
	st := model.SourceStatus{Name: "templates"}
	if a.err != nil {
		st.Detail = "dockerMan templates directory not readable"
		st.Error = a.err.Error()
		return st
	}
	st.Available = true
	st.Detail = pluralize(len(a.templates), "template")
	return st
}

// Count returns the number of parsed templates.
func (a *Adapter) Count() int { return len(a.templates) }

// Match finds the best template for a container: an exact (case-insensitive)
// name match wins, then a unique repository match. Repository comparison
// ignores tags, digests and the docker.io/library prefixes.
func (a *Adapter) Match(containerName, image string) *model.TemplateMeta {
	name := model.ContainerKey(containerName)
	for i := range a.templates {
		if model.ContainerKey(a.templates[i].Name) == name {
			return a.templates[i].meta()
		}
	}
	imageRef := catalog.NormalizeImageRef(image)
	var found *Template
	for i := range a.templates {
		if catalog.NormalizeImageRef(a.templates[i].Repository) == imageRef && imageRef != "" {
			if found != nil {
				return nil // ambiguous: two templates share the repository
			}
			found = &a.templates[i]
		}
	}
	if found != nil {
		return found.meta()
	}
	return nil
}

func (t *Template) meta() *model.TemplateMeta {
	return &model.TemplateMeta{
		File:       t.File,
		Name:       t.Name,
		Repository: t.Repository,
		WebUI:      strings.TrimSpace(t.WebUI),
		Icon:       strings.TrimSpace(t.Icon),
		Category:   strings.TrimSpace(t.Category),
		Project:    strings.TrimSpace(t.Project),
	}
}

func pluralize(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return strconv.Itoa(n) + " " + unit + "s"
}
