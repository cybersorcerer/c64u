package debugger

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// Watchpoint describes a watched memory address.
type Watchpoint struct {
	Address        uint16
	HasCondition   bool
	ConditionValue uint8
	HitCount       uint64
	LastValue      uint8
	ConditionMet   bool // true if the condition held on the last hit
}

// ParseWatchpoint parses an expression of the form "ADDR" or "ADDR=VAL" (hex).
func ParseWatchpoint(expr string) (Watchpoint, error) {
	parts := strings.SplitN(expr, "=", 2)
	addrStr := strings.TrimSpace(parts[0])
	addrVal, err := strconv.ParseUint(addrStr, 16, 16)
	if err != nil {
		return Watchpoint{}, fmt.Errorf("invalid address %q: %w", addrStr, err)
	}
	wp := Watchpoint{Address: uint16(addrVal)}
	if len(parts) == 2 {
		valStr := strings.TrimSpace(parts[1])
		val, err := strconv.ParseUint(valStr, 16, 8)
		if err != nil {
			return Watchpoint{}, fmt.Errorf("invalid value %q: %w", valStr, err)
		}
		wp.HasCondition = true
		wp.ConditionValue = uint8(val)
	}
	return wp, nil
}

// WatchList is a thread-safe list of watchpoints.
type WatchList struct {
	mu  sync.RWMutex
	wps []Watchpoint
}

// NewWatchList creates an empty WatchList.
func NewWatchList() *WatchList {
	return &WatchList{}
}

// Add appends a watchpoint.
func (wl *WatchList) Add(wp Watchpoint) {
	wl.mu.Lock()
	defer wl.mu.Unlock()
	wl.wps = append(wl.wps, wp)
}

// Remove deletes the watchpoint at index i.
func (wl *WatchList) Remove(i int) {
	wl.mu.Lock()
	defer wl.mu.Unlock()
	if i < 0 || i >= len(wl.wps) {
		return
	}
	wl.wps = append(wl.wps[:i], wl.wps[i+1:]...)
}

// All returns a copy of every watchpoint.
func (wl *WatchList) All() []Watchpoint {
	wl.mu.RLock()
	defer wl.mu.RUnlock()
	out := make([]Watchpoint, len(wl.wps))
	copy(out, wl.wps)
	return out
}

// Check tests whether an address/value pair hits a watchpoint. It reports
// hit=true when at least one watchpoint matches, and conditionMet=true when
// a value condition holds as well.
// TODO: filter by write direction once TUI exposes read/write mode selection
func (wl *WatchList) Check(addr uint16, data uint8, write bool) (hit bool, conditionMet bool) {
	wl.mu.Lock()
	defer wl.mu.Unlock()
	for i := range wl.wps {
		wp := &wl.wps[i]
		if wp.Address != addr {
			continue
		}
		hit = true
		wp.HitCount++
		wp.LastValue = data
		wp.ConditionMet = wp.HasCondition && wp.ConditionValue == data
		if wp.ConditionMet {
			conditionMet = true
		}
	}
	return hit, conditionMet
}

// ConditionTriggered reports whether at least one watchpoint has a
// satisfied condition, which is what drives the automatic pause.
func (wl *WatchList) ConditionTriggered() bool {
	wl.mu.RLock()
	defer wl.mu.RUnlock()
	for _, wp := range wl.wps {
		if wp.ConditionMet {
			return true
		}
	}
	return false
}

// ClearConditions resets every ConditionMet flag.
func (wl *WatchList) ClearConditions() {
	wl.mu.Lock()
	defer wl.mu.Unlock()
	for i := range wl.wps {
		wl.wps[i].ConditionMet = false
	}
}
