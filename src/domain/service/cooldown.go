package service

import (
	"sync"
	"time"

	"llm-proxy/domain/port"
)

const (
	FieldCooldownKey      = "cooldown_key"
	FieldBackend          = "backend"
	FieldModel            = "model"
	FieldCooldownDuration = "cooldown_duration_seconds"
	FieldRemainingSeconds = "remaining_seconds"
	FieldIsCoolingDown    = "is_cooling_down"
	FieldActiveCount      = "active_count"
	FieldTotalCount       = "total_count"
	FieldExpiredCount     = "expired_count"
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
	logger    port.Logger
}

func NewCooldownManager(ttl time.Duration) *CooldownManager {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &CooldownManager{
		cooldowns: make(map[CooldownKey]time.Time),
		ttl:       ttl,
		logger:    &port.NopLogger{},
	}
}

func NewCooldownManagerWithSeconds(seconds int) *CooldownManager {
	return NewCooldownManager(time.Duration(seconds) * time.Second)
}

func (cm *CooldownManager) WithLogger(logger port.Logger) *CooldownManager {
	cm.logger = logger
	return cm
}

func (cm *CooldownManager) IsCoolingDown(backend, model string) bool {
	key := NewCooldownKey(backend, model)
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	until, exists := cm.cooldowns[key]
	isCooling := exists && time.Now().Before(until)

	if isCooling {
		remaining := time.Until(until).Seconds()
		cm.logger.Debug("冷却检查：后端冷却中",
			port.String(FieldBackend, backend),
			port.String(FieldModel, model),
			port.Bool(FieldIsCoolingDown, true),
			port.Int64(FieldRemainingSeconds, int64(remaining)))
	}

	return isCooling
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

	cm.logger.Info("后端进入冷却",
		port.String(FieldBackend, backend),
		port.String(FieldModel, model),
		port.Int64(FieldCooldownDuration, int64(duration.Seconds())))
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

	if _, exists := cm.cooldowns[key]; exists {
		delete(cm.cooldowns, key)
		cm.logger.Info("冷却已移除",
			port.String(FieldBackend, backend),
			port.String(FieldModel, model))
	}
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
	expiredCount := 0
	for key, expiry := range cm.cooldowns {
		if now.After(expiry) {
			delete(cm.cooldowns, key)
			expiredCount++
		}
	}

	if expiredCount > 0 {
		cm.logger.Debug("清理冷却完成",
			port.Int(FieldExpiredCount, expiredCount),
			port.Int(FieldActiveCount, len(cm.cooldowns)))
	}
}

// ClearExpired is an alias for Cleanup for port.CooldownProvider compatibility.
func (cm *CooldownManager) ClearExpired() {
	cm.Cleanup()
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
