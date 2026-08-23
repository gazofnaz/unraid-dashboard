// Package app orchestrates the discovery sources, resolver, classifier and
// store into a live container inventory, and publishes changes to the bus.
package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gazofnaz/unraid-dashboard/internal/classifier"
	"github.com/gazofnaz/unraid-dashboard/internal/config"
	"github.com/gazofnaz/unraid-dashboard/internal/discovery/docker"
	"github.com/gazofnaz/unraid-dashboard/internal/discovery/templates"
	"github.com/gazofnaz/unraid-dashboard/internal/discovery/unraid"
	"github.com/gazofnaz/unraid-dashboard/internal/events"
	"github.com/gazofnaz/unraid-dashboard/internal/linkresolver"
	"github.com/gazofnaz/unraid-dashboard/internal/model"
	"github.com/gazofnaz/unraid-dashboard/internal/store"
)

// IconRef tells the frontend how to render a container icon: a proxied image
// with generated initials as fallback, or initials alone.
type IconRef struct {
	Kind     string `json:"kind"` // proxy | initials
	URL      string `json:"url,omitempty"`
	Initials string `json:"initials"`
	Hue      int    `json:"hue"`
}

// View is the API payload for one container.
type View struct {
	model.ContainerRecord
	Key            string          `json:"key"`
	DisplayName    string          `json:"displayName"`
	Category       string          `json:"category"`
	CategorySource string          `json:"categorySource"`
	Endpoint       *model.Endpoint `json:"endpoint,omitempty"`
	Confidence     float64         `json:"confidence,omitempty"`
	LowConfidence  bool            `json:"lowConfidence,omitempty"`
	Source         string          `json:"source,omitempty"`
	Icon           IconRef         `json:"icon"`
	Favorite       bool            `json:"favorite,omitempty"`
	Hidden         bool            `json:"hidden,omitempty"`
	HasOverride    bool            `json:"hasOverride,omitempty"`
	CandidateCount int             `json:"candidateCount"`
}

// Stats summarizes the inventory for the dashboard cards.
type Stats struct {
	Total          int       `json:"total"`
	Running        int       `json:"running"`
	Stopped        int       `json:"stopped"`
	WithWebUI      int       `json:"withWebUi"`
	Exact          int       `json:"exact"`
	Inferred       int       `json:"inferred"`
	LowConfidence  int       `json:"lowConfidence"`
	NoWebUI        int       `json:"noWebUi"`
	DiscoveryScore int       `json:"discoveryScore"` // percent of containers needing no manual link
	LastReconcile  time.Time `json:"lastReconcile"`
	DurationMS     int64     `json:"durationMs"`
}

// StatusPayload is the /api/v1/status response and the SSE "status" event.
type StatusPayload struct {
	Version   string               `json:"version"`
	Sources   []model.SourceStatus `json:"sources"`
	Identity  model.HostIdentity   `json:"identity"`
	Server    model.ServerInfo     `json:"server"`
	Stats     Stats                `json:"stats"`
	Settings  model.Settings       `json:"settings"`
	Linkstack model.Linkstack      `json:"linkstack"`
}

// Patch is the SSE "patch" event: compact diffs, not full snapshots.
type Patch struct {
	Upserts  []View   `json:"upserts,omitempty"`
	Removals []string `json:"removals,omitempty"`
	Stats    Stats    `json:"stats"`
}

// App wires every component together.
type App struct {
	cfg      config.Config
	log      *slog.Logger
	store    *store.Store
	bus      *events.Bus
	docker   *docker.Client
	unraid   *unraid.Adapter
	tmpl     *templates.Adapter
	resolver *linkresolver.Resolver
	selfHint string

	mu           sync.RWMutex
	views        map[string]*View // by container ID
	decisions    map[string]model.Decision
	overrides    map[string]model.Override
	settings     model.Settings
	linkstack    model.Linkstack
	dockerStatus model.SourceStatus
	identity     model.HostIdentity
	server       model.ServerInfo
	stats        Stats
	iconURLs     map[string]string

	reconcileMu sync.Mutex
	kick        chan []string // event-driven partial reconciles (nil = full)
}

// New builds the application. A failing Docker connection is not fatal: the
// status is reported and reconnection is retried by the reconcile loop.
func New(cfg config.Config, log *slog.Logger) (*App, error) {
	st, err := store.Open(cfg.DataDir + "/arraydeck.db")
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	settings, _, err := st.Settings()
	if err != nil {
		return nil, err
	}
	overrides, err := st.Overrides()
	if err != nil {
		return nil, err
	}
	decisions, err := st.Decisions()
	if err != nil {
		return nil, err
	}
	linkstack, err := st.Linkstack()
	if err != nil {
		return nil, err
	}

	a := &App{
		cfg:       cfg,
		log:       log,
		store:     st,
		bus:       events.NewBus(),
		unraid:    unraid.New(cfg.UnraidAPIURL, cfg.UnraidAPIKey),
		tmpl:      templates.New(cfg.TemplatesDir),
		resolver:  linkresolver.New(linkresolver.NewHTTPProber(cfg.ProbeConnectTO, cfg.ProbeTotalTO, cfg.ProbeConcurrency)),
		views:     map[string]*View{},
		decisions: decisions,
		overrides: overrides,
		settings:  settings,
		linkstack: linkstack,
		iconURLs:  map[string]string{},
		kick:      make(chan []string, 16),
	}
	a.selfHint, _ = os.Hostname()

	client, err := docker.NewClient(cfg.DockerHost)
	if err != nil {
		a.dockerStatus = model.SourceStatus{Name: "docker", Detail: "invalid DOCKER_HOST", Error: err.Error()}
	} else {
		a.docker = client
	}
	return a, nil
}

// Bus exposes the event bus for the SSE handler.
func (a *App) Bus() *events.Bus { return a.bus }

// Close releases resources.
func (a *App) Close() { a.store.Close() }

// Run starts the reconcile loop, Docker event subscription and periodic
// Unraid refresh. It blocks until ctx is cancelled.
func (a *App) Run(ctx context.Context) {
	a.tmpl.Reload()
	if a.unraid != nil {
		a.unraid.Refresh(ctx)
	}
	a.Reconcile(ctx, nil)

	go a.eventLoop(ctx)

	reconcile := time.NewTicker(a.cfg.ReconcileInterval)
	unraidRefresh := time.NewTicker(5 * time.Minute)
	defer reconcile.Stop()
	defer unraidRefresh.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case ids := <-a.kick:
			a.Reconcile(ctx, ids)
		case <-reconcile.C:
			a.Reconcile(ctx, nil)
		case <-unraidRefresh.C:
			if a.unraid != nil {
				a.unraid.Refresh(ctx)
			}
		}
	}
}

// eventLoop subscribes to Docker events, debounces bursts and requests
// partial reconciles for the affected containers only.
func (a *App) eventLoop(ctx context.Context) {
	relevant := map[string]bool{
		"create": true, "start": true, "stop": true, "die": true, "destroy": true,
		"rename": true, "pause": true, "unpause": true, "restart": true,
		"health_status": true,
	}
	for ctx.Err() == nil {
		if a.docker == nil {
			return
		}
		stream, err := a.docker.StreamEvents(ctx)
		if err != nil {
			a.log.Warn("docker event stream unavailable, retrying", "error", err)
			select {
			case <-time.After(5 * time.Second):
				continue
			case <-ctx.Done():
				return
			}
		}
		a.log.Info("docker event stream connected")
		pending := map[string]bool{}
		var timer *time.Timer
		var timerC <-chan time.Time
		flush := func() {
			ids := make([]string, 0, len(pending))
			for id := range pending {
				ids = append(ids, id)
			}
			pending = map[string]bool{}
			timerC = nil
			select {
			case a.kick <- ids:
			default: // a full reconcile is already queued; it will cover this
			}
		}
	consume:
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-stream:
				if !ok {
					break consume
				}
				action, _, _ := strings.Cut(ev.Action, ":")
				if !relevant[action] || ev.Actor.ID == "" {
					continue
				}
				pending[ev.Actor.ID] = true
				if timerC == nil {
					timer = time.NewTimer(a.cfg.EventDebounce)
					timerC = timer.C
				}
			case <-timerC:
				flush()
			}
		}
		if timer != nil {
			timer.Stop()
		}
		a.log.Warn("docker event stream closed, reconnecting")
	}
}

// Reconcile rebuilds the inventory. With ids it re-inspects only those
// containers; with nil it does a full pass. Every path publishes compact
// patches and refreshed status.
func (a *App) Reconcile(ctx context.Context, ids []string) {
	a.reconcileMu.Lock()
	defer a.reconcileMu.Unlock()
	started := time.Now()

	if a.docker == nil {
		client, err := docker.NewClient(a.cfg.DockerHost)
		if err == nil {
			a.docker = client
		}
	}
	if a.docker == nil || a.docker.Ping(ctx) != nil {
		a.setDockerStatus(false, "Docker is unreachable")
		a.publishStatus()
		return
	}

	full := ids == nil
	if full {
		a.tmpl.Reload()
		listed, err := a.docker.ListContainerIDs(ctx)
		if err != nil {
			a.setDockerStatus(false, err.Error())
			a.publishStatus()
			return
		}
		ids = listed
	}
	a.setDockerStatus(true, "")

	// Inspect with bounded parallelism.
	type inspectResult struct {
		id  string
		rec *model.ContainerRecord
	}
	results := make([]inspectResult, len(ids))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			doc, err := a.docker.Inspect(ctx, id)
			if err != nil {
				results[i] = inspectResult{id: id, rec: nil} // gone → removal
				return
			}
			rec := docker.Normalize(doc, a.selfHint)
			results[i] = inspectResult{id: id, rec: &rec}
		}(i, id)
	}
	wg.Wait()

	identity := a.computeIdentity()
	serverInfo := a.serverInfo()

	a.mu.RLock()
	settings := a.settings
	overridesCopy := make(map[string]model.Override, len(a.overrides))
	for k, v := range a.overrides {
		overridesCopy[k] = v
	}
	// For partial reconciles the namespace targets may not be in this batch.
	recordsByName := map[string]*model.ContainerRecord{}
	for _, v := range a.views {
		rec := v.ContainerRecord
		recordsByName[rec.Name] = &rec
	}
	a.mu.RUnlock()

	var records []*model.ContainerRecord
	removals := []string{}
	for _, res := range results {
		if res.rec == nil {
			removals = append(removals, res.id)
			continue
		}
		res.rec.Template = a.tmpl.Match(res.rec.Name, res.rec.Image)
		records = append(records, res.rec)
		recordsByName[res.rec.Name] = res.rec
	}

	// Resolve + classify + build views.
	upserts := []View{}
	changedDecisions := map[string]model.Decision{}
	for _, rec := range records {
		var override *model.Override
		if o, ok := overridesCopy[rec.Key()]; ok {
			override = &o
		}
		var netTarget *model.ContainerRecord
		if target := strings.TrimPrefix(rec.NetworkMode, "container:"); target != rec.NetworkMode {
			// Docker stores container:<id> or container:<name>.
			if t, ok := recordsByName[target]; ok {
				netTarget = t
			} else {
				for _, r := range recordsByName {
					if strings.HasPrefix(r.ID, target) {
						netTarget = r
						break
					}
				}
			}
		}
		dec := a.resolver.Resolve(ctx, linkresolver.Inputs{
			Record:        *rec,
			NetworkTarget: netTarget,
			Override:      override,
			HostLANIP:     identity.LANAddress,
			ProbeEnabled:  settings.ProbeEnabled && rec.State == model.StateRunning,
		})
		group, groupSource := classifier.Classify(*rec, override)
		view := a.buildView(rec, dec, override, group, groupSource)
		upserts = append(upserts, view)
		if decisionChanged(a.decisionFor(rec.Key()), dec) {
			changedDecisions[rec.Key()] = dec
		}
	}

	// Apply state under lock, compute the real diff.
	a.mu.Lock()
	if full {
		seen := map[string]bool{}
		for _, v := range upserts {
			seen[v.ID] = true
		}
		for id := range a.views {
			if !seen[id] {
				removals = append(removals, id)
			}
		}
	}
	realUpserts := make([]View, 0, len(upserts))
	for _, v := range upserts {
		prev, ok := a.views[v.ID]
		if !ok || !viewsEqual(*prev, v) {
			vv := v
			a.views[v.ID] = &vv
			realUpserts = append(realUpserts, v)
		}
	}
	realRemovals := make([]string, 0, len(removals))
	for _, id := range removals {
		if _, ok := a.views[id]; ok {
			delete(a.views, id)
			realRemovals = append(realRemovals, id)
		}
	}
	for key, dec := range changedDecisions {
		a.decisions[key] = dec
	}
	a.identity = identity
	a.server = serverInfo
	stats := computeStats(a.views)
	stats.LastReconcile = time.Now().UTC()
	stats.DurationMS = time.Since(started).Milliseconds()
	a.stats = stats
	liveKeys := map[string]bool{}
	for _, v := range a.views {
		liveKeys[v.Key] = true
	}
	a.mu.Unlock()

	for key, dec := range changedDecisions {
		if err := a.store.SaveDecision(key, dec); err != nil {
			a.log.Warn("persist decision failed", "container", key, "error", err)
		}
	}
	if full {
		if err := a.store.PruneDecisions(liveKeys); err != nil {
			a.log.Warn("prune decisions failed", "error", err)
		}
		if err := a.store.RecordRun(started, time.Now(), stats); err != nil {
			a.log.Warn("record run failed", "error", err)
		}
	}

	if len(realUpserts) > 0 || len(realRemovals) > 0 {
		a.bus.Publish("patch", Patch{Upserts: realUpserts, Removals: realRemovals, Stats: stats})
	}
	a.publishStatus()
	a.log.Info("reconcile complete", "full", full,
		"containers", len(records), "changed", len(realUpserts), "removed", len(realRemovals),
		"took", time.Since(started).Round(time.Millisecond).String())
}

func (a *App) decisionFor(key string) *model.Decision {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if d, ok := a.decisions[key]; ok {
		return &d
	}
	return nil
}

// decisionChanged ignores timestamps so unchanged outcomes don't churn SQLite.
func decisionChanged(old *model.Decision, next model.Decision) bool {
	if old == nil {
		return true
	}
	a, b := *old, next
	a.ResolvedAt, b.ResolvedAt = time.Time{}, time.Time{}
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	return string(ja) != string(jb)
}

func viewsEqual(a, b View) bool {
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	return string(ja) == string(jb)
}

func (a *App) buildView(rec *model.ContainerRecord, dec model.Decision, override *model.Override, group, groupSource string) View {
	v := View{
		ContainerRecord: *rec,
		Key:             rec.Key(),
		DisplayName:     displayName(rec, override),
		Category:        group,
		CategorySource:  groupSource,
		Icon:            a.iconRef(rec, override),
		CandidateCount:  len(dec.Candidates),
	}
	if override != nil {
		v.HasOverride = !override.IsZero()
		if override.Favorite != nil {
			v.Favorite = *override.Favorite
		}
		if override.Hidden != nil {
			v.Hidden = *override.Hidden
		}
	}
	if dec.Winner != nil {
		ep := dec.Winner.Endpoint
		v.Endpoint = &ep
		v.Confidence = ep.Confidence
		v.LowConfidence = ep.Confidence < 0.75
		v.Source = dec.Winner.Source
	}
	return v
}

func displayName(rec *model.ContainerRecord, override *model.Override) string {
	if override != nil && override.Name != "" {
		return override.Name
	}
	if rec.Template != nil && rec.Template.Name != "" {
		return rec.Template.Name
	}
	return rec.Name
}

func (a *App) iconRef(rec *model.ContainerRecord, override *model.Override) IconRef {
	ref := IconRef{Kind: "initials", Initials: initials(displayName(rec, override)), Hue: nameHue(rec.Name)}
	var src string
	switch {
	case override != nil && override.Icon != "":
		src = override.Icon
	case rec.Labels["net.unraid.docker.icon"] != "":
		src = rec.Labels["net.unraid.docker.icon"]
	case rec.Template != nil && rec.Template.Icon != "":
		src = rec.Template.Icon
	}
	if src != "" && (strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://")) {
		sum := sha256.Sum256([]byte(src))
		key := hex.EncodeToString(sum[:8])
		a.mu.Lock()
		a.iconURLs[key] = src
		a.mu.Unlock()
		ref.Kind = "proxy"
		ref.URL = "/api/v1/icons/" + key
	}
	return ref
}

func initials(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "?"
	}
	words := strings.FieldsFunc(name, func(r rune) bool {
		return r == ' ' || r == '-' || r == '_' || r == '.'
	})
	if len(words) >= 2 {
		return strings.ToUpper(words[0][:1] + words[1][:1])
	}
	if len(name) >= 2 {
		return strings.ToUpper(name[:2])
	}
	return strings.ToUpper(name)
}

func nameHue(name string) int {
	h := fnv.New32a()
	h.Write([]byte(name))
	return int(h.Sum32() % 360)
}

func computeStats(views map[string]*View) Stats {
	s := Stats{}
	for _, v := range views {
		if v.IsSelf {
			continue
		}
		s.Total++
		if v.State == model.StateRunning {
			s.Running++
		} else {
			s.Stopped++
		}
		if v.Endpoint != nil {
			s.WithWebUI++
			if v.Confidence >= 0.9 {
				s.Exact++
			} else {
				s.Inferred++
			}
			if v.LowConfidence {
				s.LowConfidence++
			}
		} else {
			s.NoWebUI++
		}
	}
	if s.Total > 0 {
		// Share of containers that either have a solid link or legitimately
		// have no web interface — i.e. nothing needs manual attention.
		ok := s.Total - s.LowConfidence
		s.DiscoveryScore = int(float64(ok) / float64(s.Total) * 100)
	}
	return s
}

// computeIdentity follows ARCHITECTURE.md §4: override, Unraid API data,
// API endpoint hostname, then empty (browser origin is the runtime fallback).
func (a *App) computeIdentity() model.HostIdentity {
	a.mu.RLock()
	s := a.settings
	a.mu.RUnlock()
	ident := model.HostIdentity{}
	uid := a.unraid.Identity()

	ident.Candidates = uid.LANAddresses
	switch {
	case s.LANAddress != "":
		ident.LANAddress, ident.LANSource = s.LANAddress, "override"
	case s.PreferredInterface != "" && len(uid.Interfaces[s.PreferredInterface]) > 0:
		ident.LANAddress = uid.Interfaces[s.PreferredInterface][0]
		ident.LANSource = "unraid-api:" + s.PreferredInterface
	case len(uid.LANAddresses) > 0:
		ident.LANAddress, ident.LANSource = uid.LANAddresses[0], "unraid-api"
	}

	switch {
	case s.ServerHostname != "":
		ident.Hostname, ident.HostnameSource = s.ServerHostname, "override"
	case uid.Hostname != "":
		host := strings.ToLower(uid.Hostname)
		if !strings.Contains(host, ".") {
			host += ".local"
		}
		ident.Hostname, ident.HostnameSource = host, "unraid-api"
	default:
		if h := a.unraid.EndpointHostname(); h != "" && !isIPLiteral(h) {
			ident.Hostname, ident.HostnameSource = h, "api-endpoint"
		}
	}
	return ident
}

func isIPLiteral(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && r != '.' && r != ':' {
			return false
		}
	}
	return true
}

func (a *App) serverInfo() model.ServerInfo {
	uid := a.unraid.Identity()
	return model.ServerInfo{
		Name:          uid.Hostname,
		UnraidVersion: uid.UnraidVersion,
		UptimeSeconds: uid.UptimeSeconds,
	}
}

func (a *App) setDockerStatus(available bool, detail string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if available {
		a.dockerStatus = model.SourceStatus{Name: "docker", Available: true, Detail: "connected"}
	} else {
		a.dockerStatus = model.SourceStatus{Name: "docker", Available: false, Detail: "unavailable", Error: detail}
	}
}

func (a *App) publishStatus() {
	a.bus.Publish("status", a.Status())
}

// Status assembles the full status payload.
func (a *App) Status() StatusPayload {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return StatusPayload{
		Version:   a.cfg.Version,
		Sources:   []model.SourceStatus{a.dockerStatus, a.unraid.Status(), a.tmpl.Status()},
		Identity:  a.identity,
		Server:    a.server,
		Stats:     a.stats,
		Settings:  a.settings,
		Linkstack: a.linkstack,
	}
}

// Snapshot returns all views sorted by display name.
func (a *App) Snapshot() []View {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]View, 0, len(a.views))
	for _, v := range a.views {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].DisplayName) < strings.ToLower(out[j].DisplayName)
	})
	return out
}

// Find locates a view by container ID or key.
func (a *App) Find(idOrKey string) (View, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if v, ok := a.views[idOrKey]; ok {
		return *v, true
	}
	for _, v := range a.views {
		if v.Key == model.ContainerKey(idOrKey) || strings.HasPrefix(v.ID, idOrKey) {
			return *v, true
		}
	}
	return View{}, false
}

// Discovery returns the stored decision for a container.
func (a *App) Discovery(idOrKey string) (model.Decision, bool) {
	v, ok := a.Find(idOrKey)
	if !ok {
		// The container may have disappeared but the decision can remain.
		a.mu.RLock()
		defer a.mu.RUnlock()
		d, ok := a.decisions[model.ContainerKey(idOrKey)]
		return d, ok
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	d, ok := a.decisions[v.Key]
	return d, ok
}

// Override returns the stored override for a container key.
func (a *App) Override(key string) (model.Override, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	o, ok := a.overrides[key]
	return o, ok
}

// SetOverride persists an override and triggers a re-resolve.
func (a *App) SetOverride(idOrKey string, o model.Override) error {
	v, ok := a.Find(idOrKey)
	if !ok {
		return fmt.Errorf("unknown container %q", idOrKey)
	}
	if err := a.store.SaveOverride(v.Key, o); err != nil {
		return err
	}
	a.mu.Lock()
	a.overrides[v.Key] = o
	a.mu.Unlock()
	a.requestReconcile([]string{v.ID})
	return nil
}

// ClearOverride removes an override and triggers a re-resolve.
func (a *App) ClearOverride(idOrKey string) error {
	v, ok := a.Find(idOrKey)
	if !ok {
		return fmt.Errorf("unknown container %q", idOrKey)
	}
	if err := a.store.DeleteOverride(v.Key); err != nil {
		return err
	}
	a.mu.Lock()
	delete(a.overrides, v.Key)
	a.mu.Unlock()
	a.requestReconcile([]string{v.ID})
	return nil
}

// Settings returns current settings.
func (a *App) Settings() model.Settings {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.settings
}

// SaveSettings persists settings and triggers a full reconcile because the
// address policy or probe behavior may have changed.
func (a *App) SaveSettings(s model.Settings) error {
	if s.LinkMode != model.LinkModeHostname && s.LinkMode != model.LinkModeLANIP && s.LinkMode != model.LinkModeSmart {
		return fmt.Errorf("invalid linkMode %q", s.LinkMode)
	}
	if err := a.store.SaveSettings(s); err != nil {
		return err
	}
	a.mu.Lock()
	a.settings = s
	a.mu.Unlock()
	a.requestReconcile(nil)
	return nil
}

// Linkstack returns the curated launcher configuration.
func (a *App) Linkstack() model.Linkstack {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.linkstack
}

// SaveLinkstack persists the launcher configuration and publishes it to every
// connected browser. No rediscovery is needed: ordering and address form are
// presentation over endpoints that are already resolved.
func (a *App) SaveLinkstack(l model.Linkstack) error {
	l = l.Normalize()
	if err := a.store.SaveLinkstack(l); err != nil {
		return err
	}
	a.mu.Lock()
	a.linkstack = l
	a.mu.Unlock()
	a.publishStatus()
	return nil
}

// RequestReconcile queues a full reconcile (used by POST /discovery/reconcile).
func (a *App) RequestReconcile() { a.requestReconcile(nil) }

func (a *App) requestReconcile(ids []string) {
	select {
	case a.kick <- ids:
	default:
	}
}

// IconSource maps an icon cache key back to its upstream URL.
func (a *App) IconSource(key string) (string, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	url, ok := a.iconURLs[key]
	return url, ok
}

// Store exposes the persistence layer to the API for icon caching.
func (a *App) Store() *store.Store { return a.store }
