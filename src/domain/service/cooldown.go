package domain

import (
	"sync"
	"time"
)

// CooldownKey is a composite key for cooldown entries.
type CooldownKey string

// NewCooldownKey creates a new cooldown key.
func NewCooldownKey(backend, model string) CooldownKey {
	return CooldownKey(backend + "/" + model)
}

// String returns the string representation.
func (k CooldownKey) String() string {
	return string(k)
}

// CooldownManager manages backend cooldown states.
// Thread-safe implementation using read-write mutex.
type CooldownManager struct {
	cooldowns map[CooldownKey]time.Time
	mu        sync.RWMutex
	ttl       time.Duration
}

// NewCooldownManager creates a new cooldown manager.
func NewCooldownManager(ttl time.Duration) *CooldownManager {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &CooldownManager{
		cooldowns: make(map[CooldownKey]time.Time),
		ttl:       ttl,
	}
}

// NewCooldownManagerWithSeconds creates a new cooldown manager with TTL in seconds.
func NewCooldownManagerWithSeconds(seconds int) *CooldownManager {
	return NewCooldownManager(time.Duration(seconds) * time.Second)
}

// IsCoolingDown returns true if the backend is in cooldown for the given model.
func (cm *CooldownManager) IsCoolingDown(backend, model string) bool {
	key := NewCooldownKey(backend, model)
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	until, exists := cm.cooldowns[key]
	return exists && time.Now().Before(until)
}

// IsCoolingDownKey returns true if the key is in cooldown.
func (cm *CooldownManager) IsCoolingDownKey(key CooldownKey) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	until, exists := cm.cooldowns[key]
	return exists && time.Now().Before(until)
}

// SetCooldown sets a cooldown period for a backend/model combination.
func (cm *CooldownManager) SetCooldown(backend, model string, duration time.Duration) {
	key := NewCooldownKey(backend, model)
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.cooldowns[key] = time.Now().Add(duration)
}

// SetCooldownKey sets a cooldown for a key.
func (cm *CooldownManager) SetCooldownKey(key CooldownKey, duration time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.cooldowns[key] = time.Now().Add(duration)
}

// RemoveCooldown removes the cooldown for a backend/model combination.
func (cm *CooldownManager) RemoveCooldown(backend, model string) {
	key := NewCooldownKey(backend, model)
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.cooldowns, key)
}

// RemoveCooldownKey removes the cooldown for a key.
func (cm *CooldownManager) RemoveCooldownKey(key CooldownKey) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.cooldowns, key)
}

// Cleanup removes all expired cooldown entries.
func (cm *CooldownManager) Cleanup() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	now := time.Now()
	for key, expiry := range cm.cooldowns {
		if now.After(expiry) {
			delete(cm.cooldowns, key)
		}
	}
}

// Clear removes all cooldown entries.
func (cm *CooldownManager) Clear() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.cooldowns = make(map[CooldownKey]time.Time)
}

// Len returns the number of cooldown entries.
func (cm *CooldownManager) Len() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.cooldowns)
}

// ActiveCount returns the number of active (non-expired) cooldown entries.
func (cm *CooldownManager) ActiveCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	now := time.Now()
	count := 0
	for _, expiry := range cm.cooldowns {
		if now.Before(expiry) {
			count++
		}
	}
	return count
}

// GetRemainingTTL returns the remaining TTL for a key.
func (cm *CooldownManager) GetRemainingTTL(backend, model string) time.Duration {
	key := NewCooldownKey(backend, model)
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	until, exists := cm.cooldowns[key]
	if !exists {
		return 0
	}
	remaining := time.Until(until)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Snapshot returns a snapshot of current cooldown states.
func (cm *CooldownManager) Snapshot() map[string]time.Time {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	snapshot := make(map[string]time.Time)
	now := time.Now()
	for key, expiry := range cm.cooldowns {
		if now.Before(expiry) {
			snapshot[string(key)] = expiry
		}
	}
	return snapshot
}