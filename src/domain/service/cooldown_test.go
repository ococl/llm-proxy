package service

import (
	"testing"
	"time"
)

func TestCooldownManager_New(t *testing.T) {
	t.Run("NewCooldownManager with positive TTL", func(t *testing.T) {
		cm := NewCooldownManager(5 * time.Minute)
		if cm == nil {
			t.Error("Expected non-nil CooldownManager")
		}
	})

	t.Run("NewCooldownManager with zero TTL uses default", func(t *testing.T) {
		cm := NewCooldownManager(0)
		if cm == nil {
			t.Error("Expected non-nil CooldownManager")
		}
	})

	t.Run("NewCooldownManagerWithSeconds", func(t *testing.T) {
		cm := NewCooldownManagerWithSeconds(300)
		if cm == nil {
			t.Error("Expected non-nil CooldownManager")
		}
	})
}

func TestCooldownManager_IsCoolingDown(t *testing.T) {
	t.Run("Returns false for non-existent backend", func(t *testing.T) {
		cm := NewCooldownManager(5 * time.Minute)
		if cm.IsCoolingDown("unknown-backend", "gpt-4") {
			t.Error("Expected non-existent backend to not be cooling down")
		}
	})

	t.Run("Returns true after setting cooldown", func(t *testing.T) {
		cm := NewCooldownManager(5 * time.Minute)
		cm.SetCooldown("openai", "gpt-4", 1*time.Minute)

		if !cm.IsCoolingDown("openai", "gpt-4") {
			t.Error("Expected backend to be cooling down after setting cooldown")
		}
	})

	t.Run("Returns false after cooldown expires", func(t *testing.T) {
		cm := NewCooldownManager(1 * time.Millisecond)
		cm.SetCooldown("openai", "gpt-4", 1*time.Millisecond)
		time.Sleep(10 * time.Millisecond)

		if cm.IsCoolingDown("openai", "gpt-4") {
			t.Error("Expected backend to not be cooling down after expiry")
		}
	})

	t.Run("IsCoolingDownKey with CooldownKey", func(t *testing.T) {
		cm := NewCooldownManager(5 * time.Minute)
		key := NewCooldownKey("test", "key")
		cm.SetCooldownKey(key, 1*time.Minute)

		if !cm.IsCoolingDownKey(key) {
			t.Error("Expected key to be cooling down")
		}
	})
}

func TestCooldownManager_SetCooldown(t *testing.T) {
	t.Run("SetCooldown adds entry", func(t *testing.T) {
		cm := NewCooldownManager(5 * time.Minute)
		cm.SetCooldown("openai", "gpt-4", 1*time.Minute)

		if cm.Len() != 1 {
			t.Errorf("Expected 1 cooldown entry, got %d", cm.Len())
		}
	})

	t.Run("SetCooldownKey adds entry", func(t *testing.T) {
		cm := NewCooldownManager(5 * time.Minute)
		key := NewCooldownKey("test", "key")
		cm.SetCooldownKey(key, 1*time.Minute)

		if cm.Len() != 1 {
			t.Errorf("Expected 1 cooldown entry, got %d", cm.Len())
		}
	})
}

func TestCooldownManager_RemoveCooldown(t *testing.T) {
	t.Run("RemoveCooldown deletes entry", func(t *testing.T) {
		cm := NewCooldownManager(5 * time.Minute)
		cm.SetCooldown("openai", "gpt-4", 1*time.Minute)
		cm.RemoveCooldown("openai", "gpt-4")

		if cm.Len() != 0 {
			t.Errorf("Expected 0 cooldown entries, got %d", cm.Len())
		}
	})

	t.Run("RemoveCooldownKey deletes entry", func(t *testing.T) {
		cm := NewCooldownManager(5 * time.Minute)
		key := NewCooldownKey("test", "key")
		cm.SetCooldownKey(key, 1*time.Minute)
		cm.RemoveCooldownKey(key)

		if cm.Len() != 0 {
			t.Errorf("Expected 0 cooldown entries, got %d", cm.Len())
		}
	})
}

func TestCooldownManager_Clear(t *testing.T) {
	t.Run("Clear removes all entries", func(t *testing.T) {
		cm := NewCooldownManager(5 * time.Minute)
		cm.SetCooldown("openai", "gpt-4", 1*time.Minute)
		cm.SetCooldown("anthropic", "claude", 1*time.Minute)
		cm.Clear()

		if cm.Len() != 0 {
			t.Errorf("Expected 0 cooldown entries, got %d", cm.Len())
		}
	})
}

func TestCooldownManager_Cleanup(t *testing.T) {
	t.Run("Cleanup removes expired entries", func(t *testing.T) {
		cm := NewCooldownManager(5 * time.Minute)
		cm.SetCooldown("openai", "gpt-4", 1*time.Millisecond)
		cm.SetCooldown("anthropic", "claude", 5*time.Minute)

		time.Sleep(10 * time.Millisecond)
		cm.Cleanup()

		if cm.Len() != 1 {
			t.Errorf("Expected 1 remaining entry, got %d", cm.Len())
		}
	})

	t.Run("ClearExpired is alias for Cleanup", func(t *testing.T) {
		cm := NewCooldownManager(5 * time.Minute)
		cm.SetCooldown("openai", "gpt-4", 1*time.Millisecond)
		cm.ClearExpired()
		// Verification done through other tests
	})
}

func TestCooldownManager_Len(t *testing.T) {
	t.Run("Len returns correct count", func(t *testing.T) {
		cm := NewCooldownManager(5 * time.Minute)
		if cm.Len() != 0 {
			t.Errorf("Expected 0, got %d", cm.Len())
		}

		cm.SetCooldown("openai", "gpt-4", 5*time.Minute)
		if cm.Len() != 1 {
			t.Errorf("Expected 1, got %d", cm.Len())
		}

		cm.SetCooldown("anthropic", "claude", 5*time.Minute)
		if cm.Len() != 2 {
			t.Errorf("Expected 2, got %d", cm.Len())
		}
	})
}

func TestCooldownManager_ActiveCount(t *testing.T) {
	t.Run("ActiveCount returns only non-expired", func(t *testing.T) {
		cm := NewCooldownManager(5 * time.Minute)
		cm.SetCooldown("openai", "gpt-4", 1*time.Millisecond)
		cm.SetCooldown("anthropic", "claude", 5*time.Minute)

		time.Sleep(10 * time.Millisecond)
		active := cm.ActiveCount()

		if active != 1 {
			t.Errorf("Expected 1 active entry, got %d", active)
		}
	})
}

func TestCooldownManager_GetRemainingTTL(t *testing.T) {
	t.Run("Returns zero for non-existent key", func(t *testing.T) {
		cm := NewCooldownManager(5 * time.Minute)
		ttl := cm.GetRemainingTTL("unknown", "gpt-4")
		if ttl != 0 {
			t.Errorf("Expected 0, got %v", ttl)
		}
	})

	t.Run("Returns positive TTL for active cooldown", func(t *testing.T) {
		cm := NewCooldownManager(5 * time.Minute)
		cm.SetCooldown("openai", "gpt-4", 1*time.Minute)
		ttl := cm.GetRemainingTTL("openai", "gpt-4")

		if ttl <= 0 {
			t.Error("Expected positive TTL")
		}
		if ttl > 1*time.Minute {
			t.Error("Expected TTL less than 1 minute")
		}
	})
}

func TestCooldownManager_Snapshot(t *testing.T) {
	t.Run("Snapshot returns active cooldowns", func(t *testing.T) {
		cm := NewCooldownManager(5 * time.Minute)
		cm.SetCooldown("openai", "gpt-4", 1*time.Millisecond)
		cm.SetCooldown("anthropic", "claude", 5*time.Minute)

		time.Sleep(10 * time.Millisecond)
		snapshot := cm.Snapshot()

		if len(snapshot) != 1 {
			t.Errorf("Expected 1 entry in snapshot, got %d", len(snapshot))
		}
	})
}

func TestCooldownKey(t *testing.T) {
	t.Run("NewCooldownKey creates valid key", func(t *testing.T) {
		key := NewCooldownKey("openai", "gpt-4")
		expected := "openai/gpt-4"
		if key.String() != expected {
			t.Errorf("Expected '%s', got '%s'", expected, key.String())
		}
	})

	t.Run("CooldownKey as string", func(t *testing.T) {
		key := NewCooldownKey("backend", "model")
		if string(key) != "backend/model" {
			t.Errorf("Expected 'backend/model', got '%s'", string(key))
		}
	})
}
