// Package api exposes the REST API, the SSE stream, the icon proxy and the
// embedded frontend. The API is read-only toward Docker by design: no
// endpoint can mutate containers, and no generic Docker passthrough exists.
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gazofnaz/unraid-dashboard/internal/app"
	"github.com/gazofnaz/unraid-dashboard/internal/model"
)

// Server hosts the HTTP API.
type Server struct {
	app *app.App
	log *slog.Logger
	mux *http.ServeMux
}

// New builds the handler tree.
func New(a *app.App, log *slog.Logger) *Server {
	s := &Server{app: a, log: log, mux: http.NewServeMux()}

	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /api/v1/status", s.handleStatus)
	s.mux.HandleFunc("GET /api/v1/containers", s.handleContainers)
	s.mux.HandleFunc("GET /api/v1/containers/{id}", s.handleContainer)
	s.mux.HandleFunc("GET /api/v1/containers/{id}/discovery", s.handleDiscovery)
	s.mux.HandleFunc("POST /api/v1/containers/{id}/override", s.handleSetOverride)
	s.mux.HandleFunc("DELETE /api/v1/containers/{id}/override", s.handleClearOverride)
	s.mux.HandleFunc("GET /api/v1/settings", s.handleGetSettings)
	s.mux.HandleFunc("PUT /api/v1/settings", s.handlePutSettings)
	s.mux.HandleFunc("GET /api/v1/linkstack", s.handleGetLinkstack)
	s.mux.HandleFunc("PUT /api/v1/linkstack", s.handlePutLinkstack)
	s.mux.HandleFunc("POST /api/v1/discovery/reconcile", s.handleReconcile)
	s.mux.HandleFunc("GET /api/v1/events", s.handleEvents)
	s.mux.HandleFunc("GET /api/v1/icons/{key}", s.handleIcon)
	s.mux.Handle("/", staticHandler())

	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintln(w, "ok")
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.app.Status())
}

func (s *Server) handleContainers(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"containers": s.app.Snapshot()})
}

func (s *Server) handleContainer(w http.ResponseWriter, r *http.Request) {
	v, ok := s.app.Find(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "container not found")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleDiscovery(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	dec, ok := s.app.Discovery(id)
	if !ok {
		writeError(w, http.StatusNotFound, "no discovery decision for container")
		return
	}
	payload := map[string]any{"decision": dec}
	if v, found := s.app.Find(id); found {
		payload["container"] = v
		if o, has := s.app.Override(v.Key); has {
			payload["override"] = o
		}
	}
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleSetOverride(w http.ResponseWriter, r *http.Request) {
	var o model.Override
	if err := decodeJSON(r, &o); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.app.SetOverride(r.PathValue("id"), o); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleClearOverride(w http.ResponseWriter, r *http.Request) {
	if err := s.app.ClearOverride(r.PathValue("id")); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleGetSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.app.Settings())
}

func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var settings model.Settings
	if err := decodeJSON(r, &settings); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.app.SaveSettings(settings); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.app.Settings())
}

func (s *Server) handleGetLinkstack(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.app.Linkstack())
}

func (s *Server) handlePutLinkstack(w http.ResponseWriter, r *http.Request) {
	var stack model.Linkstack
	if err := decodeJSON(r, &stack); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.app.SaveLinkstack(stack); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.app.Linkstack())
}

func (s *Server) handleReconcile(w http.ResponseWriter, _ *http.Request) {
	s.app.RequestReconcile()
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
}

// handleEvents is the SSE stream. On connect the client receives the current
// status immediately, then patches and status updates as they happen.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	writeSSE := func(event string, data []byte) bool {
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	if initial, err := json.Marshal(s.app.Status()); err == nil {
		if !writeSSE("status", initial) {
			return
		}
	}

	sub, cancel := s.app.Bus().Subscribe()
	defer cancel()
	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg, open := <-sub:
			if !open {
				return
			}
			if !writeSSE(msg.Event, msg.Data) {
				return
			}
		case <-keepalive.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// handleIcon proxies and caches container icons so the browser never loads
// mixed-content or CORS-restricted images directly.
func (s *Server) handleIcon(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if mime, body, ok := s.app.Store().IconGet(key); ok {
		serveIcon(w, mime, body)
		return
	}
	src, ok := s.app.IconSource(key)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown icon")
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, src, nil)
	if err != nil {
		writeError(w, http.StatusNotFound, "bad icon source")
		return
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "icon fetch failed")
		return
	}
	defer resp.Body.Close()
	contentType := resp.Header.Get("Content-Type")
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(contentType, "image/") {
		writeError(w, http.StatusBadGateway, "not an image")
		return
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil || len(body) == 0 {
		writeError(w, http.StatusBadGateway, "icon fetch failed")
		return
	}
	if err := s.app.Store().IconPut(key, contentType, body, resp.Header.Get("ETag")); err != nil {
		s.log.Warn("icon cache write failed", "error", err)
	}
	serveIcon(w, contentType, body)
}

func serveIcon(w http.ResponseWriter, mime string, body []byte) {
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(body)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	return nil
}
