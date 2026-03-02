package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func newTestSlotManager(t *testing.T, maxSlots int) (*SlotManager, string) {
	t.Helper()
	tmpDir := t.TempDir()
	slotsFile := filepath.Join(tmpDir, ".slots.json")
	return NewSlotManager(slotsFile, maxSlots), slotsFile
}

func TestSlotManager_AllocateFirstSlot(t *testing.T) {
	sm, _ := newTestSlotManager(t, 5)

	slot, err := sm.Allocate("myapp-fix-auth")
	if err != nil {
		t.Fatalf("Allocate() error = %v", err)
	}
	if slot != 1 {
		t.Errorf("Allocate() = %d, want 1", slot)
	}
}

func TestSlotManager_AllocateFillsSequentially(t *testing.T) {
	sm, _ := newTestSlotManager(t, 3)

	tests := []struct {
		worktree string
		wantSlot int
	}{
		{"myapp-feature-a", 1},
		{"myapp-feature-b", 2},
		{"myapp-feature-c", 3},
	}

	for _, tt := range tests {
		slot, err := sm.Allocate(tt.worktree)
		if err != nil {
			t.Fatalf("Allocate(%q) error = %v", tt.worktree, err)
		}
		if slot != tt.wantSlot {
			t.Errorf("Allocate(%q) = %d, want %d", tt.worktree, slot, tt.wantSlot)
		}
	}
}

func TestSlotManager_AllocateIdempotent(t *testing.T) {
	sm, _ := newTestSlotManager(t, 5)

	first, err := sm.Allocate("myapp-fix-auth")
	if err != nil {
		t.Fatalf("first Allocate() error = %v", err)
	}

	second, err := sm.Allocate("myapp-fix-auth")
	if err != nil {
		t.Fatalf("second Allocate() error = %v", err)
	}

	if first != second {
		t.Errorf("idempotent Allocate() returned different slots: %d vs %d", first, second)
	}
}

func TestSlotManager_AllocateErrorWhenFull(t *testing.T) {
	sm, _ := newTestSlotManager(t, 2)

	_, _ = sm.Allocate("myapp-feature-a")
	_, _ = sm.Allocate("myapp-feature-b")

	_, err := sm.Allocate("myapp-feature-c")
	if err == nil {
		t.Error("Allocate() expected error when all slots full, got nil")
	}
}

func TestSlotManager_ReleaseFreesSlot(t *testing.T) {
	sm, _ := newTestSlotManager(t, 2)

	_, _ = sm.Allocate("myapp-feature-a")
	_, _ = sm.Allocate("myapp-feature-b")

	if err := sm.Release("myapp-feature-a"); err != nil {
		t.Fatalf("Release() error = %v", err)
	}

	slot, err := sm.Allocate("myapp-feature-c")
	if err != nil {
		t.Fatalf("Allocate() after release error = %v", err)
	}
	if slot != 1 {
		t.Errorf("Allocate() after release = %d, want 1 (lowest freed slot)", slot)
	}
}

func TestSlotManager_AllocateReusesLowestFreedSlot(t *testing.T) {
	sm, _ := newTestSlotManager(t, 3)

	_, _ = sm.Allocate("myapp-feature-a") // slot 1
	_, _ = sm.Allocate("myapp-feature-b") // slot 2
	_, _ = sm.Allocate("myapp-feature-c") // slot 3

	_ = sm.Release("myapp-feature-a") // free slot 1
	_ = sm.Release("myapp-feature-b") // free slot 2

	slot, err := sm.Allocate("myapp-feature-d")
	if err != nil {
		t.Fatalf("Allocate() error = %v", err)
	}
	if slot != 1 {
		t.Errorf("Allocate() = %d, want 1 (lowest freed slot)", slot)
	}
}

func TestSlotManager_FindSlotReturnsCorrectSlot(t *testing.T) {
	sm, _ := newTestSlotManager(t, 5)

	_, _ = sm.Allocate("myapp-feature-a") // slot 1
	_, _ = sm.Allocate("myapp-feature-b") // slot 2

	slot, err := sm.FindSlot("myapp-feature-b")
	if err != nil {
		t.Fatalf("FindSlot() error = %v", err)
	}
	if slot != 2 {
		t.Errorf("FindSlot() = %d, want 2", slot)
	}
}

func TestSlotManager_FindSlotReturnsZeroForUnknown(t *testing.T) {
	sm, _ := newTestSlotManager(t, 5)

	_, _ = sm.Allocate("myapp-feature-a")

	slot, err := sm.FindSlot("nonexistent-worktree")
	if err != nil {
		t.Fatalf("FindSlot() error = %v", err)
	}
	if slot != 0 {
		t.Errorf("FindSlot() = %d, want 0 for unknown worktree", slot)
	}
}

func TestSlotManager_ListActive(t *testing.T) {
	sm, _ := newTestSlotManager(t, 5)

	_, _ = sm.Allocate("myapp-feature-a")
	_, _ = sm.Allocate("myapp-feature-b")

	active, err := sm.ListActive()
	if err != nil {
		t.Fatalf("ListActive() error = %v", err)
	}
	if len(active) != 2 {
		t.Errorf("ListActive() returned %d slots, want 2", len(active))
	}

	found := make(map[string]int)
	for _, s := range active {
		found[s.Worktree] = s.Slot
	}
	if found["myapp-feature-a"] != 1 {
		t.Errorf("myapp-feature-a slot = %d, want 1", found["myapp-feature-a"])
	}
	if found["myapp-feature-b"] != 2 {
		t.Errorf("myapp-feature-b slot = %d, want 2", found["myapp-feature-b"])
	}
}

func TestSlotManager_MissingFileCreatedOnFirstWrite(t *testing.T) {
	sm, slotsFile := newTestSlotManager(t, 5)

	// File should not exist yet
	if _, err := os.Stat(slotsFile); !os.IsNotExist(err) {
		t.Fatal("slots file should not exist before first allocation")
	}

	_, err := sm.Allocate("myapp-feature-a")
	if err != nil {
		t.Fatalf("Allocate() error = %v", err)
	}

	if _, err := os.Stat(slotsFile); err != nil {
		t.Errorf("slots file should exist after first allocation: %v", err)
	}
}

func TestSlotManager_HandlesEmptySlotsFile(t *testing.T) {
	tmpDir := t.TempDir()
	slotsFile := filepath.Join(tmpDir, ".slots.json")

	// Create empty file
	if err := os.WriteFile(slotsFile, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	sm := NewSlotManager(slotsFile, 5)

	slot, err := sm.Allocate("myapp-feature-a")
	if err != nil {
		t.Fatalf("Allocate() with empty file error = %v", err)
	}
	if slot != 1 {
		t.Errorf("Allocate() = %d, want 1", slot)
	}
}

func TestSlotManager_ConcurrentAccess(t *testing.T) {
	sm, _ := newTestSlotManager(t, 10)

	const goroutines = 5
	slots := make([]int, goroutines)
	errs := make([]error, goroutines)

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			worktree := fmt.Sprintf("myapp-worker-%d", idx)
			slot, err := sm.Allocate(worktree)
			slots[idx] = slot
			errs[idx] = err
		}(i)
	}

	wg.Wait()

	// All allocations should succeed
	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d Allocate() error = %v", i, err)
		}
	}

	// All slots should be unique and in range 1-10
	seen := make(map[int]bool)
	for i, slot := range slots {
		if slot < 1 || slot > 10 {
			t.Errorf("goroutine %d: slot %d out of range [1, 10]", i, slot)
		}
		if seen[slot] {
			t.Errorf("goroutine %d: slot %d already assigned to another goroutine", i, slot)
		}
		seen[slot] = true
	}
}
