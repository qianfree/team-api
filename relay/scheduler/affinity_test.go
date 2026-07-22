package scheduler

import (
	"testing"
	"time"
)

func TestAffinityStore_SetGet(t *testing.T) {
	store := NewAffinityStore()

	store.Set(1, 1, "gpt-4o", 100)
	ch, ok := store.Get(1, 1, "gpt-4o")
	if !ok {
		t.Fatal("expected affinity to be found")
	}
	if ch != 100 {
		t.Errorf("expected channel 100, got %d", ch)
	}
}

func TestAffinityStore_NotFound(t *testing.T) {
	store := NewAffinityStore()

	_, ok := store.Get(1, 999, "nonexistent")
	if ok {
		t.Error("expected affinity not to be found")
	}
}

func TestAffinityStore_Expiry(t *testing.T) {
	store := NewAffinityStore()

	// Manually set an expired entry
	store.mu.Lock()
	store.entries[affinityKey{TenantID: 1, UserID: 1, ModelName: "test"}] = &affinityEntry{
		ChannelID: 100,
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	store.mu.Unlock()

	_, ok := store.Get(1, 1, "test")
	if ok {
		t.Error("expected expired affinity not to be found")
	}
}

func TestAffinityStore_Delete(t *testing.T) {
	store := NewAffinityStore()
	store.Set(1, 1, "gpt-4o", 100)
	store.Delete(1, 1, "gpt-4o")

	_, ok := store.Get(1, 1, "gpt-4o")
	if ok {
		t.Error("expected affinity to be deleted")
	}
}

func TestAffinityStore_DeleteByChannel(t *testing.T) {
	store := NewAffinityStore()
	store.Set(1, 1, "gpt-4o", 100)
	store.Set(1, 2, "gpt-4o", 100) // same model, different user
	store.Set(1, 1, "claude-3", 200)

	store.DeleteByChannel(100)

	_, ok1 := store.Get(1, 1, "gpt-4o")
	_, ok2 := store.Get(1, 2, "gpt-4o")
	_, ok3 := store.Get(1, 1, "claude-3")

	if ok1 || ok2 {
		t.Error("expected channel 100 affinities to be deleted")
	}
	if !ok3 {
		t.Error("expected channel 200 affinity to remain")
	}
}

func TestAffinityStore_HitCount(t *testing.T) {
	store := NewAffinityStore()

	store.Set(1, 1, "gpt-4o", 100)
	store.Set(1, 1, "gpt-4o", 100) // same key, should increment

	store.mu.RLock()
	entry := store.entries[affinityKey{TenantID: 1, UserID: 1, ModelName: "gpt-4o"}]
	store.mu.RUnlock()

	if entry.HitCount != 2 {
		t.Errorf("expected hit count 2, got %d", entry.HitCount)
	}
}

func TestAffinityStore_TenantIsolation(t *testing.T) {
	store := NewAffinityStore()

	// Tenant 1, User 1 → channel 100
	store.Set(1, 1, "gpt-4o", 100)
	// Tenant 2, User 1 → channel 200 (same userID, different tenant)
	store.Set(2, 1, "gpt-4o", 200)

	ch1, ok1 := store.Get(1, 1, "gpt-4o")
	ch2, ok2 := store.Get(2, 1, "gpt-4o")

	if !ok1 || ch1 != 100 {
		t.Errorf("tenant 1 expected channel 100, got %d, ok=%v", ch1, ok1)
	}
	if !ok2 || ch2 != 200 {
		t.Errorf("tenant 2 expected channel 200, got %d, ok=%v", ch2, ok2)
	}

	// Deleting tenant 1's affinity should not affect tenant 2
	store.Delete(1, 1, "gpt-4o")
	_, ok1 = store.Get(1, 1, "gpt-4o")
	_, ok2 = store.Get(2, 1, "gpt-4o")
	if ok1 {
		t.Error("expected tenant 1 affinity to be deleted")
	}
	if !ok2 {
		t.Error("expected tenant 2 affinity to remain")
	}
}

func TestAffinityStore_CleanExpired(t *testing.T) {
	store := NewAffinityStore()

	store.Set(1, 1, "active", 100)
	store.Set(1, 2, "active", 200)

	// Manually add an expired entry
	store.mu.Lock()
	store.entries[affinityKey{TenantID: 1, UserID: 3, ModelName: "expired"}] = &affinityEntry{
		ChannelID: 300,
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	store.mu.Unlock()

	cleaned := store.CleanExpired()
	if cleaned != 1 {
		t.Errorf("expected 1 expired entry cleaned, got %d", cleaned)
	}
	if store.Size() != 2 {
		t.Errorf("expected 2 remaining entries, got %d", store.Size())
	}
}
