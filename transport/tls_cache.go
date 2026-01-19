package transport

import (
	"encoding/base64"
	"sync"
	"time"

	tls "github.com/sardanioss/utls"
)

// TLSSessionMaxAge is the maximum age for TLS sessions (24 hours)
// TLS session tickets typically expire after 24-48 hours
const TLSSessionMaxAge = 24 * time.Hour

// TLSSessionCacheMaxSize is the maximum number of sessions to cache
// Matches the size used by pool/pool.go for LRU session cache
const TLSSessionCacheMaxSize = 32

// TLSSessionState represents a serializable TLS session
type TLSSessionState struct {
	Ticket    string    `json:"ticket"`     // base64 encoded
	State     string    `json:"state"`      // base64 encoded
	CreatedAt time.Time `json:"created_at"`
}

// PersistableSessionCache implements tls.ClientSessionCache
// with export/import capabilities for session persistence and LRU eviction
type PersistableSessionCache struct {
	mu          sync.RWMutex
	sessions    map[string]*cachedSession
	accessOrder []string // LRU order: oldest at front, newest at back
}

type cachedSession struct {
	state     *tls.ClientSessionState
	createdAt time.Time
}

// NewPersistableSessionCache creates a new persistable session cache
func NewPersistableSessionCache() *PersistableSessionCache {
	return &PersistableSessionCache{
		sessions: make(map[string]*cachedSession),
	}
}

// Get implements tls.ClientSessionCache
func (c *PersistableSessionCache) Get(sessionKey string) (*tls.ClientSessionState, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if cached, ok := c.sessions[sessionKey]; ok {
		// Move to end of accessOrder (most recently used)
		c.moveToEnd(sessionKey)
		return cached.state, true
	}
	return nil, false
}

// moveToEnd moves a key to the end of accessOrder (must be called with lock held)
func (c *PersistableSessionCache) moveToEnd(key string) {
	for i, k := range c.accessOrder {
		if k == key {
			c.accessOrder = append(c.accessOrder[:i], c.accessOrder[i+1:]...)
			c.accessOrder = append(c.accessOrder, key)
			return
		}
	}
}

// Put implements tls.ClientSessionCache
func (c *PersistableSessionCache) Put(sessionKey string, cs *tls.ClientSessionState) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if key already exists
	if _, exists := c.sessions[sessionKey]; exists {
		// Update existing entry and move to end
		c.sessions[sessionKey] = &cachedSession{
			state:     cs,
			createdAt: time.Now(),
		}
		c.moveToEnd(sessionKey)
		return
	}

	// Evict oldest if at capacity
	if len(c.sessions) >= TLSSessionCacheMaxSize && len(c.accessOrder) > 0 {
		oldest := c.accessOrder[0]
		c.accessOrder = c.accessOrder[1:]
		delete(c.sessions, oldest)
	}

	// Add new entry
	c.sessions[sessionKey] = &cachedSession{
		state:     cs,
		createdAt: time.Now(),
	}
	c.accessOrder = append(c.accessOrder, sessionKey)
}

// Export serializes all TLS sessions for persistence
// Returns a map of session keys to serialized TLS session state
func (c *PersistableSessionCache) Export() (map[string]TLSSessionState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]TLSSessionState)

	for key, cached := range c.sessions {
		if cached.state == nil {
			continue
		}

		// Get resumption state from ClientSessionState
		ticket, state, err := cached.state.ResumptionState()
		if err != nil {
			continue // Skip invalid sessions
		}

		if state == nil || ticket == nil {
			continue
		}

		// Serialize the SessionState to bytes
		stateBytes, err := state.Bytes()
		if err != nil {
			continue // Skip sessions that can't be serialized
		}

		result[key] = TLSSessionState{
			Ticket:    base64.StdEncoding.EncodeToString(ticket),
			State:     base64.StdEncoding.EncodeToString(stateBytes),
			CreatedAt: cached.createdAt,
		}
	}

	return result, nil
}

// Import loads TLS sessions from serialized state
// Sessions older than TLSSessionMaxAge are skipped
func (c *PersistableSessionCache) Import(sessions map[string]TLSSessionState) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, serialized := range sessions {
		// Skip expired sessions
		if time.Since(serialized.CreatedAt) > TLSSessionMaxAge {
			continue
		}

		// Decode ticket
		ticket, err := base64.StdEncoding.DecodeString(serialized.Ticket)
		if err != nil {
			continue
		}

		// Decode state
		stateBytes, err := base64.StdEncoding.DecodeString(serialized.State)
		if err != nil {
			continue
		}

		// Parse session state
		state, err := tls.ParseSessionState(stateBytes)
		if err != nil {
			continue
		}

		// Create resumption state
		clientState, err := tls.NewResumptionState(ticket, state)
		if err != nil {
			continue
		}

		c.sessions[key] = &cachedSession{
			state:     clientState,
			createdAt: serialized.CreatedAt,
		}
		c.accessOrder = append(c.accessOrder, key)
	}

	// Enforce max size limit after import (evict oldest if over limit)
	for len(c.sessions) > TLSSessionCacheMaxSize && len(c.accessOrder) > 0 {
		oldest := c.accessOrder[0]
		c.accessOrder = c.accessOrder[1:]
		delete(c.sessions, oldest)
	}

	return nil
}

// Clear removes all cached sessions
func (c *PersistableSessionCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessions = make(map[string]*cachedSession)
	c.accessOrder = nil
}

// Count returns the number of cached sessions
func (c *PersistableSessionCache) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.sessions)
}
