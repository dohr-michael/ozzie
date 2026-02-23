package scheduler

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dohr-michael/ozzie/internal/events"
	"github.com/dohr-michael/ozzie/internal/tasks"
)

// DefaultCooldown is the minimum interval between two triggers of the same entry.
const DefaultCooldown = 60 * time.Second

// Config holds dependencies for the scheduler.
type Config struct {
	Pool   tasks.TaskSubmitter
	Bus    *events.Bus
	Skills []SkillScheduleInfo // skill-based schedule entries (converted by caller)
	Store  *ScheduleStore      // nil-safe: dynamic entries are not persisted without a store
}

// Entry represents a scheduled skill trigger (legacy, kept for Entries() compat).
type Entry struct {
	SkillName string
	Cron      *CronExpr
	OnEvent   *EventTrigger
	LastRun   time.Time
	Cooldown  time.Duration
}

// runtimeEntry is the unified internal representation for all schedule entries.
type runtimeEntry struct {
	id          string
	source      string // "skill" or "dynamic"
	sessionID   string
	title       string
	description string
	skillName   string
	cron        *CronExpr
	intervalSec int
	onEvent     *EventTrigger
	tmpl        *TaskTemplate
	cooldown    time.Duration
	maxRuns     int
	runCount    int
	enabled     bool
	lastRun     time.Time
}

// toEntry converts to the legacy Entry type for backward compat.
func (r *runtimeEntry) toEntry() Entry {
	return Entry{
		SkillName: r.skillName,
		Cron:      r.cron,
		OnEvent:   r.onEvent,
		LastRun:   r.lastRun,
		Cooldown:  r.cooldown,
	}
}

// Scheduler manages cron-based, interval-based, and event-triggered execution.
type Scheduler struct {
	pool   tasks.TaskSubmitter
	bus    *events.Bus
	skills []SkillScheduleInfo
	store  *ScheduleStore

	mu      sync.Mutex
	entries map[string]*runtimeEntry

	done        chan struct{}
	unsubscribe func()
}

// New creates a new Scheduler.
func New(cfg Config) *Scheduler {
	return &Scheduler{
		pool:    cfg.Pool,
		bus:     cfg.Bus,
		skills:  cfg.Skills,
		store:   cfg.Store,
		entries: make(map[string]*runtimeEntry),
		done:    make(chan struct{}),
	}
}

// Start loads entries from the skill registry (and persisted store) and begins
// the cron/interval tickers and event subscription.
func (s *Scheduler) Start() {
	s.loadSkillEntries()
	s.loadPersistedEntries()

	slog.Info("scheduler started", "entries", len(s.entries))

	// Always start loops and event subscription — entries can be added dynamically.
	s.unsubscribe = s.bus.Subscribe(s.handleEvent)
	go s.cronLoop()
	go s.intervalLoop()
}

// Stop halts the scheduler.
func (s *Scheduler) Stop() {
	close(s.done)
	if s.unsubscribe != nil {
		s.unsubscribe()
	}
	slog.Info("scheduler stopped")
}

// Entries returns a snapshot of all scheduler entries (legacy compat).
func (s *Scheduler) Entries() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]Entry, 0, len(s.entries))
	for _, e := range s.entries {
		result = append(result, e.toEntry())
	}
	return result
}

// AddEntry registers a dynamic schedule entry at runtime.
func (s *Scheduler) AddEntry(se *ScheduleEntry) error {
	if se.CronSpec == "" && se.IntervalSec == 0 && se.OnEvent == nil {
		return fmt.Errorf("schedule entry must have cron, interval, or on_event trigger")
	}
	if se.IntervalSec > 0 && se.IntervalSec < 5 {
		return fmt.Errorf("interval must be at least 5 seconds")
	}

	if se.ID == "" {
		se.ID = GenerateScheduleID()
	}

	re := &runtimeEntry{
		id:          se.ID,
		source:      se.Source,
		sessionID:   se.SessionID,
		title:       se.Title,
		description: se.Description,
		skillName:   se.SkillName,
		intervalSec: se.IntervalSec,
		onEvent:     se.OnEvent,
		tmpl:        se.TaskTemplate,
		cooldown:    time.Duration(se.CooldownSec) * time.Second,
		maxRuns:     se.MaxRuns,
		runCount:    se.RunCount,
		enabled:     se.Enabled,
	}

	if se.CronSpec != "" {
		expr, err := ParseCron(se.CronSpec)
		if err != nil {
			return fmt.Errorf("parse cron: %w", err)
		}
		re.cron = expr
	}

	if re.cooldown == 0 {
		re.cooldown = DefaultCooldown
	}

	// Persist if store is available
	if s.store != nil && se.Source == "dynamic" {
		if err := s.store.Create(se); err != nil {
			return fmt.Errorf("persist schedule: %w", err)
		}
	}

	s.mu.Lock()
	s.entries[se.ID] = re
	s.mu.Unlock()

	slog.Info("scheduler: added entry", "id", se.ID, "title", se.Title, "source", se.Source)
	return nil
}

// RemoveEntry removes a schedule entry by ID.
func (s *Scheduler) RemoveEntry(id string) error {
	s.mu.Lock()
	re, ok := s.entries[id]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("schedule entry not found: %s", id)
	}
	delete(s.entries, id)
	s.mu.Unlock()

	// Remove from persistent store
	if s.store != nil && re.source == "dynamic" {
		if err := s.store.Delete(id); err != nil {
			slog.Warn("scheduler: failed to delete persisted entry", "id", id, "error", err)
		}
	}

	slog.Info("scheduler: removed entry", "id", id)
	return nil
}

// GetEntry returns a schedule entry by ID.
func (s *Scheduler) GetEntry(id string) (*ScheduleEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	re, ok := s.entries[id]
	if !ok {
		return nil, false
	}
	return runtimeToScheduleEntry(re), true
}

// ListEntries returns all schedule entries as ScheduleEntry structs.
func (s *Scheduler) ListEntries() []*ScheduleEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]*ScheduleEntry, 0, len(s.entries))
	for _, re := range s.entries {
		result = append(result, runtimeToScheduleEntry(re))
	}
	return result
}

func runtimeToScheduleEntry(re *runtimeEntry) *ScheduleEntry {
	se := &ScheduleEntry{
		ID:           re.id,
		Source:       re.source,
		SessionID:    re.sessionID,
		Title:        re.title,
		Description:  re.description,
		SkillName:    re.skillName,
		IntervalSec:  re.intervalSec,
		OnEvent:      re.onEvent,
		TaskTemplate: re.tmpl,
		CooldownSec:  int(re.cooldown / time.Second),
		MaxRuns:      re.maxRuns,
		RunCount:     re.runCount,
		Enabled:      re.enabled,
	}
	if re.cron != nil {
		se.CronSpec = re.cron.String()
	}
	if !re.lastRun.IsZero() {
		t := re.lastRun
		se.LastRunAt = &t
	}
	return se
}

// loadSkillEntries populates entries from the pre-extracted skill schedule info.
func (s *Scheduler) loadSkillEntries() {
	for _, sk := range s.skills {
		id := "skill_" + sk.Name
		re := &runtimeEntry{
			id:        id,
			source:    "skill",
			title:     sk.Name,
			skillName: sk.Name,
			onEvent:   sk.OnEvent,
			cooldown:  DefaultCooldown,
			enabled:   true,
		}

		if sk.Cron != "" {
			expr, err := ParseCron(sk.Cron)
			if err != nil {
				slog.Warn("scheduler: invalid cron for skill", "skill", sk.Name, "error", err)
				continue
			}
			re.cron = expr
		}

		s.entries[id] = re

		slog.Info("scheduler: registered skill entry", "skill", sk.Name,
			"cron", sk.Cron, "has_event", sk.OnEvent != nil)
	}
}

// loadPersistedEntries loads dynamic entries from the store (if available).
func (s *Scheduler) loadPersistedEntries() {
	if s.store == nil {
		return
	}

	entries, err := s.store.List()
	if err != nil {
		slog.Warn("scheduler: failed to load persisted entries", "error", err)
		return
	}

	for _, se := range entries {
		if !se.Enabled {
			continue
		}

		re := &runtimeEntry{
			id:          se.ID,
			source:      se.Source,
			sessionID:   se.SessionID,
			title:       se.Title,
			description: se.Description,
			skillName:   se.SkillName,
			intervalSec: se.IntervalSec,
			onEvent:     se.OnEvent,
			tmpl:        se.TaskTemplate,
			cooldown:    time.Duration(se.CooldownSec) * time.Second,
			maxRuns:     se.MaxRuns,
			runCount:    se.RunCount,
			enabled:     true,
		}

		if se.CronSpec != "" {
			expr, err := ParseCron(se.CronSpec)
			if err != nil {
				slog.Warn("scheduler: invalid cron in persisted entry", "id", se.ID, "error", err)
				continue
			}
			re.cron = expr
		}

		if re.cooldown == 0 {
			re.cooldown = DefaultCooldown
		}

		s.entries[se.ID] = re
		slog.Info("scheduler: loaded persisted entry", "id", se.ID, "title", se.Title)
	}
}

func (s *Scheduler) cronLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case now := <-ticker.C:
			s.checkCron(now)
		}
	}
}

func (s *Scheduler) intervalLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case now := <-ticker.C:
			s.checkIntervals(now)
		}
	}
}

func (s *Scheduler) checkCron(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, entry := range s.entries {
		if entry.cron == nil || !entry.enabled {
			continue
		}
		if !entry.cron.Matches(now) {
			continue
		}
		if now.Sub(entry.lastRun) < entry.cooldown {
			continue
		}

		s.triggerEntry(entry, "cron")
	}
}

func (s *Scheduler) checkIntervals(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, entry := range s.entries {
		if entry.intervalSec <= 0 || !entry.enabled {
			continue
		}
		interval := time.Duration(entry.intervalSec) * time.Second
		if now.Sub(entry.lastRun) < interval {
			continue
		}

		s.triggerEntry(entry, "interval")
	}
}

func (s *Scheduler) handleEvent(e events.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, entry := range s.entries {
		if entry.onEvent == nil || !entry.enabled {
			continue
		}
		if !MatchEvent(e, entry.onEvent) {
			continue
		}
		if now.Sub(entry.lastRun) < entry.cooldown {
			continue
		}

		s.triggerEntry(entry, "event:"+string(e.Type))
	}
}

// triggerEntry submits a task for the given entry. Caller must hold s.mu.
func (s *Scheduler) triggerEntry(re *runtimeEntry, trigger string) {
	re.lastRun = time.Now()
	re.runCount++

	var task *tasks.Task

	if re.tmpl != nil {
		// Dynamic entry: create task from template
		task = &tasks.Task{
			SessionID:   re.sessionID,
			Title:       re.tmpl.Title,
			Description: re.tmpl.Description,
			Config: tasks.TaskConfig{
				Tools:   re.tmpl.Tools,
				WorkDir: re.tmpl.WorkDir,
				Env:     re.tmpl.Env,
			},
		}
	} else {
		// Skill entry
		task = &tasks.Task{
			Title:       "scheduled: " + re.skillName,
			Description: "Triggered by scheduler (" + trigger + ")",
			Config: tasks.TaskConfig{
				Skill: re.skillName,
			},
		}
	}

	if err := s.pool.Submit(task); err != nil {
		slog.Error("scheduler: submit task", "id", re.id, "error", err)
		return
	}

	// Update persistent store
	if s.store != nil && re.source == "dynamic" {
		s.updateStoredEntry(re)
	}

	// Auto-disable at max runs
	if re.maxRuns > 0 && re.runCount >= re.maxRuns {
		re.enabled = false
		slog.Info("scheduler: entry reached max runs, disabled", "id", re.id, "runs", re.runCount)
		if s.store != nil && re.source == "dynamic" {
			s.updateStoredEntry(re)
		}
	}

	// Emit schedule trigger event
	s.bus.Publish(events.NewTypedEvent(events.SourceScheduler, events.ScheduleTriggerPayload{
		EntryID:   re.id,
		SkillName: re.skillName,
		Trigger:   trigger,
		TaskID:    task.ID,
	}))

	slog.Info("scheduler: triggered", "id", re.id, "trigger", trigger, "task_id", task.ID)
}

// updateStoredEntry persists runtime state back to store. Caller must hold s.mu.
func (s *Scheduler) updateStoredEntry(re *runtimeEntry) {
	se := runtimeToScheduleEntry(re)
	if err := s.store.Update(se); err != nil {
		slog.Warn("scheduler: failed to update persisted entry", "id", re.id, "error", err)
	}
}
