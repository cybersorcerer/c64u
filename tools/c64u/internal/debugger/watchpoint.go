package debugger

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// Watchpoint beschreibt eine überwachte Speicheradresse.
type Watchpoint struct {
	Address        uint16
	HasCondition   bool
	ConditionValue uint8
	HitCount       uint64
	LastValue      uint8
	ConditionMet   bool // true wenn beim letzten Hit die Bedingung erfüllt war
}

// ParseWatchpoint parst einen Ausdruck der Form "ADDR" oder "ADDR=VAL" (hex).
func ParseWatchpoint(expr string) (Watchpoint, error) {
	parts := strings.SplitN(expr, "=", 2)
	addrStr := strings.TrimSpace(parts[0])
	addrVal, err := strconv.ParseUint(addrStr, 16, 16)
	if err != nil {
		return Watchpoint{}, fmt.Errorf("ungültige Adresse %q: %w", addrStr, err)
	}
	wp := Watchpoint{Address: uint16(addrVal)}
	if len(parts) == 2 {
		valStr := strings.TrimSpace(parts[1])
		val, err := strconv.ParseUint(valStr, 16, 8)
		if err != nil {
			return Watchpoint{}, fmt.Errorf("ungültiger Wert %q: %w", valStr, err)
		}
		wp.HasCondition = true
		wp.ConditionValue = uint8(val)
	}
	return wp, nil
}

// WatchList verwaltet eine thread-sichere Liste von Watchpoints.
type WatchList struct {
	mu  sync.RWMutex
	wps []Watchpoint
}

// NewWatchList erstellt eine leere WatchList.
func NewWatchList() *WatchList {
	return &WatchList{}
}

// Add fügt einen Watchpoint hinzu.
func (wl *WatchList) Add(wp Watchpoint) {
	wl.mu.Lock()
	defer wl.mu.Unlock()
	wl.wps = append(wl.wps, wp)
}

// Remove entfernt den Watchpoint am Index i.
func (wl *WatchList) Remove(i int) {
	wl.mu.Lock()
	defer wl.mu.Unlock()
	if i < 0 || i >= len(wl.wps) {
		return
	}
	wl.wps = append(wl.wps[:i], wl.wps[i+1:]...)
}

// All gibt eine Kopie aller Watchpoints zurück.
func (wl *WatchList) All() []Watchpoint {
	wl.mu.RLock()
	defer wl.mu.RUnlock()
	out := make([]Watchpoint, len(wl.wps))
	copy(out, wl.wps)
	return out
}

// Check prüft ob die Adresse/Wert-Kombination einen Watchpoint trifft.
// Gibt true zurück wenn mindestens ein Watchpoint zutrifft.
func (wl *WatchList) Check(addr uint16, data uint8, write bool) (hit bool) {
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
	}
	return hit
}

// ConditionTriggered gibt true zurück wenn mindestens ein Watchpoint
// eine erfüllte Bedingung hat (für Auto-Pause-Logik).
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

// ClearConditions setzt alle ConditionMet-Flags zurück.
func (wl *WatchList) ClearConditions() {
	wl.mu.Lock()
	defer wl.mu.Unlock()
	for i := range wl.wps {
		wl.wps[i].ConditionMet = false
	}
}
