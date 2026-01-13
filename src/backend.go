package main

import (
	"sync"
	"time"
)

type CooldownKey string

type CooldownManager struct {
	cooldowns map[CooldownKey]time.Time
	mu        sync.RWMutex
}

func NewCooldownManager() *CooldownManager {
	return &CooldownManager{
		cooldowns: make(map[CooldownKey]time.Time),
	}
}

func (cm *CooldownManager) Key(backend, model string) CooldownKey {
	return CooldownKey(backend + "/" + model)
}

func (cm *CooldownManager) IsCoolingDown(key CooldownKey) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	until, exists := cm.cooldowns[key]
	return exists && time.Now().Before(until)
}

func (cm *CooldownManager) SetCooldown(key CooldownKey, duration time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.cooldowns[key] = time.Now().Add(duration)
	LogGeneral("INFO", "设置冷却: %s 直到 %v", key, cm.cooldowns[key].Format(time.RFC3339))
}

func (cm *CooldownManager) ClearExpired() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	now := time.Now()
	for key, until := range cm.cooldowns {
		if now.After(until) {
			delete(cm.cooldowns, key)
		}
	}
}
