package session

import (
	"sync"
	"time"
)

// Manager manages directory-scoped sessions.
type Manager struct {
	sessions    map[string]*entry
	maxMessages int
	maxIdle     time.Duration
	mu          sync.RWMutex
	done        chan struct{}
}

type entry struct {
	session  *Session
	lastUsed time.Time
}

// NewManager creates a new session manager.
func NewManager(maxMessages int, maxIdle time.Duration) *Manager {
	m := &Manager{
		sessions:    make(map[string]*entry),
		maxMessages: maxMessages,
		maxIdle:     maxIdle,
		done:        make(chan struct{}),
	}
	go m.cleanupLoop()
	return m
}

// GetOrCreate returns the session for the given directory, creating one if needed.
func (m *Manager) GetOrCreate(dir string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if e, ok := m.sessions[dir]; ok {
		e.lastUsed = time.Now()
		return e.session
	}

	s := NewSession(dir, m.maxMessages)
	m.sessions[dir] = &entry{session: s, lastUsed: time.Now()}
	return s
}

// Stop removes and returns the session for the given directory.
func (m *Manager) Stop(dir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, dir)
}

// StopAll removes all sessions.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions = make(map[string]*entry)
}

// List returns status information for all active sessions.
func (m *Manager) List() []SessionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]SessionInfo, 0, len(m.sessions))
	for dir, e := range m.sessions {
		infos = append(infos, SessionInfo{
			Dir:          dir,
			MessageCount: e.session.MessageCount(),
			LastUsed:     e.lastUsed,
		})
	}
	return infos
}

// SessionInfo contains status information about a session.
type SessionInfo struct {
	Dir          string
	MessageCount int
	LastUsed     time.Time
}

// Close stops the cleanup goroutine.
func (m *Manager) Close() {
	close(m.done)
	m.StopAll()
}

func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanIdle()
		case <-m.done:
			return
		}
	}
}

func (m *Manager) cleanIdle() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for dir, e := range m.sessions {
		if now.Sub(e.lastUsed) > m.maxIdle {
			delete(m.sessions, dir)
		}
	}
}
