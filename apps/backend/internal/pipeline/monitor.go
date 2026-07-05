package pipeline

import (
	"context"
	"errors"
	"sync"
	"time"
)

type Monitor struct {
	mu         sync.RWMutex
	now        func() time.Time
	staleAfter time.Duration
	pipelines  map[string]Status
}

type Status struct {
	Name         string    `json:"name"`
	Running      bool      `json:"running"`
	Stale        bool      `json:"stale"`
	StartedAt    time.Time `json:"startedAt,omitempty"`
	LastProgress time.Time `json:"lastProgress,omitempty"`
	LastError    string    `json:"lastError,omitempty"`
	LastDetail   string    `json:"lastDetail,omitempty"`
	ErrorCount   uint64    `json:"errorCount"`
	Processed    uint64    `json:"processed"`
}

func NewMonitor(staleAfter time.Duration) *Monitor {
	return &Monitor{
		now:        func() time.Time { return time.Now().UTC() },
		staleAfter: staleAfter,
		pipelines:  map[string]Status{},
	}
}

func (m *Monitor) Register(name string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	status := m.pipelines[name]
	status.Name = name
	status.Running = false
	status.Stale = false
	status.LastDetail = "registered"
	m.pipelines[name] = status
}

func (m *Monitor) Start(name string) {
	if m == nil {
		return
	}
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	status := m.pipelines[name]
	status.Name = name
	status.Running = true
	status.Stale = false
	status.StartedAt = now
	status.LastProgress = now
	status.LastDetail = "started"
	m.pipelines[name] = status
}

func (m *Monitor) Progress(name string, processed uint64, detail string) {
	if m == nil {
		return
	}
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	status := m.ensureLocked(name, now)
	status.LastProgress = now
	status.LastDetail = detail
	status.Processed += processed
	status.Stale = false
	m.pipelines[name] = status
}

func (m *Monitor) Error(name string, err error) {
	if m == nil || err == nil {
		return
	}
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	status := m.ensureLocked(name, now)
	status.LastProgress = now
	status.LastError = err.Error()
	status.LastDetail = "error"
	status.ErrorCount++
	status.Stale = false
	m.pipelines[name] = status
}

func (m *Monitor) Stop(name string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	status := m.pipelines[name]
	status.Name = name
	status.Running = false
	status.Stale = false
	status.LastDetail = "stopped"
	m.pipelines[name] = status
}

func (m *Monitor) Snapshot() []Status {
	if m == nil {
		return nil
	}
	now := m.now()
	m.mu.RLock()
	defer m.mu.RUnlock()
	statuses := make([]Status, 0, len(m.pipelines))
	for _, status := range m.pipelines {
		status.Stale = m.isStale(status, now)
		statuses = append(statuses, status)
	}
	return statuses
}

func (m *Monitor) Check(context.Context) error {
	if m == nil {
		return nil
	}
	now := m.now()
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, status := range m.pipelines {
		if !status.Running || m.isStale(status, now) {
			return errors.New("background pipeline unhealthy")
		}
	}
	return nil
}

func (m *Monitor) ensureLocked(name string, now time.Time) Status {
	status := m.pipelines[name]
	if status.Name == "" {
		status.Name = name
		status.StartedAt = now
	}
	status.Running = true
	return status
}

func (m *Monitor) isStale(status Status, now time.Time) bool {
	if !status.Running || m.staleAfter <= 0 || status.LastProgress.IsZero() {
		return false
	}
	return now.Sub(status.LastProgress) > m.staleAfter
}
