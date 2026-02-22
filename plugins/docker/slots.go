package docker

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"
)

// SlotInfo represents an allocated slot
type SlotInfo struct {
	Slot     int    `json:"slot"`
	Worktree string `json:"worktree"`
}

// SlotManager manages agent slot allocation using a file-based JSON store
type SlotManager struct {
	slotsFile string
	maxSlots  int
}

// NewSlotManager creates a new slot manager.
// slotsFile is the path to the .slots.json file.
// maxSlots is the maximum number of concurrent agent slots.
func NewSlotManager(slotsFile string, maxSlots int) *SlotManager {
	return &SlotManager{
		slotsFile: slotsFile,
		maxSlots:  maxSlots,
	}
}

// Allocate assigns an available slot to a worktree name.
// Returns the slot number (1-based) or error if no slots available.
// Idempotent: if the worktree already has a slot, returns that slot.
func (sm *SlotManager) Allocate(worktreeName string) (int, error) {
	f, err := sm.openLocked()
	if err != nil {
		return 0, err
	}
	defer sm.closeUnlocked(f)

	slots, err := sm.readSlots(f)
	if err != nil {
		return 0, err
	}

	// Idempotency: return existing slot if already allocated
	for _, s := range slots {
		if s.Worktree == worktreeName {
			return s.Slot, nil
		}
	}

	// Find lowest available slot number
	used := make(map[int]bool, len(slots))
	for _, s := range slots {
		used[s.Slot] = true
	}

	slot := 0
	for i := 1; i <= sm.maxSlots; i++ {
		if !used[i] {
			slot = i
			break
		}
	}
	if slot == 0 {
		return 0, fmt.Errorf("no slots available: all %d slots are in use", sm.maxSlots)
	}

	slots = append(slots, SlotInfo{Slot: slot, Worktree: worktreeName})
	if err := sm.writeSlots(f, slots); err != nil {
		return 0, err
	}

	return slot, nil
}

// Release frees the slot held by a worktree name.
func (sm *SlotManager) Release(worktreeName string) error {
	f, err := sm.openLocked()
	if err != nil {
		return err
	}
	defer sm.closeUnlocked(f)

	slots, err := sm.readSlots(f)
	if err != nil {
		return err
	}

	updated := slots[:0]
	for _, s := range slots {
		if s.Worktree != worktreeName {
			updated = append(updated, s)
		}
	}

	return sm.writeSlots(f, updated)
}

// FindSlot returns the slot number for a worktree, or 0 if not found.
func (sm *SlotManager) FindSlot(worktreeName string) (int, error) {
	slots, err := sm.readSlotsNoLock()
	if err != nil {
		return 0, err
	}
	for _, s := range slots {
		if s.Worktree == worktreeName {
			return s.Slot, nil
		}
	}
	return 0, nil
}

// ListActive returns all currently allocated slots.
func (sm *SlotManager) ListActive() ([]SlotInfo, error) {
	return sm.readSlotsNoLock()
}

// openLocked opens (or creates) the slots file with an exclusive lock.
func (sm *SlotManager) openLocked() (*os.File, error) {
	f, err := os.OpenFile(sm.slotsFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("open slots file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("lock slots file: %w", err)
	}
	return f, nil
}

// closeUnlocked releases the lock and closes the file.
func (sm *SlotManager) closeUnlocked(f *os.File) {
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	_ = f.Close()
}

// readSlots reads the current slot list from an already-open file.
func (sm *SlotManager) readSlots(f *os.File) ([]SlotInfo, error) {
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat slots file: %w", err)
	}
	if info.Size() == 0 {
		return []SlotInfo{}, nil
	}

	var slots []SlotInfo
	dec := json.NewDecoder(f)
	if err := dec.Decode(&slots); err != nil {
		return nil, fmt.Errorf("decode slots file: %w", err)
	}
	return slots, nil
}

// writeSlots truncates the file and writes the updated slot list.
func (sm *SlotManager) writeSlots(f *os.File, slots []SlotInfo) error {
	if err := f.Truncate(0); err != nil {
		return fmt.Errorf("truncate slots file: %w", err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		return fmt.Errorf("seek slots file: %w", err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(slots); err != nil {
		return fmt.Errorf("encode slots file: %w", err)
	}
	return nil
}

// readSlotsNoLock reads slots without acquiring a file lock (for read-only queries).
func (sm *SlotManager) readSlotsNoLock() ([]SlotInfo, error) {
	data, err := os.ReadFile(sm.slotsFile)
	if os.IsNotExist(err) {
		return []SlotInfo{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read slots file: %w", err)
	}
	if len(data) == 0 {
		return []SlotInfo{}, nil
	}
	var slots []SlotInfo
	if err := json.Unmarshal(data, &slots); err != nil {
		return nil, fmt.Errorf("decode slots file: %w", err)
	}
	return slots, nil
}
