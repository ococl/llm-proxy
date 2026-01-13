package main

import (
	"testing"
	"time"
)

func TestCooldownManager_Key(t *testing.T) {
	cm := NewCooldownManager()

	tests := []struct {
		backend  string
		model    string
		expected CooldownKey
	}{
		{"backend1", "model1", "backend1/model1"},
		{"", "model", "/model"},
		{"backend", "", "backend/"},
	}

	for _, tt := range tests {
		got := cm.Key(tt.backend, tt.model)
		if got != tt.expected {
			t.Errorf("Key(%q, %q) = %q, want %q", tt.backend, tt.model, got, tt.expected)
		}
	}
}

func TestCooldownManager_SetAndCheck(t *testing.T) {
	cm := NewCooldownManager()
	key := cm.Key("backend", "model")

	if cm.IsCoolingDown(key) {
		t.Error("New key should not be cooling down")
	}

	cm.SetCooldown(key, 100*time.Millisecond)

	if !cm.IsCoolingDown(key) {
		t.Error("Key should be cooling down after SetCooldown")
	}

	time.Sleep(150 * time.Millisecond)

	if cm.IsCoolingDown(key) {
		t.Error("Key should not be cooling down after expiry")
	}
}

func TestCooldownManager_ClearExpired(t *testing.T) {
	cm := NewCooldownManager()
	key1 := cm.Key("backend1", "model1")
	key2 := cm.Key("backend2", "model2")

	cm.SetCooldown(key1, 50*time.Millisecond)
	cm.SetCooldown(key2, 500*time.Millisecond)

	time.Sleep(100 * time.Millisecond)
	cm.ClearExpired()

	if cm.IsCoolingDown(key1) {
		t.Error("Expired key1 should be cleared")
	}
	if !cm.IsCoolingDown(key2) {
		t.Error("Non-expired key2 should still be cooling down")
	}
}

func TestCooldownManager_Concurrent(t *testing.T) {
	cm := NewCooldownManager()
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(id int) {
			key := cm.Key("backend", "model")
			cm.SetCooldown(key, time.Second)
			cm.IsCoolingDown(key)
			cm.ClearExpired()
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
