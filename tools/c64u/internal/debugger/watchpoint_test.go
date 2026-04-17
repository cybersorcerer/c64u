package debugger

import "testing"

func TestWatchpointParse(t *testing.T) {
	wp, err := ParseWatchpoint("D020")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wp.Address != 0xD020 {
		t.Errorf("address: got %04X, want D020", wp.Address)
	}
	if wp.HasCondition {
		t.Error("expected no condition")
	}
}

func TestWatchpointParseWithCondition(t *testing.T) {
	wp, err := ParseWatchpoint("D020=0F")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wp.Address != 0xD020 {
		t.Errorf("address: got %04X, want D020", wp.Address)
	}
	if !wp.HasCondition {
		t.Error("expected condition")
	}
	if wp.ConditionValue != 0x0F {
		t.Errorf("condition value: got %02X, want 0F", wp.ConditionValue)
	}
}

func TestWatchpointParseInvalid(t *testing.T) {
	_, err := ParseWatchpoint("ZZZZ")
	if err == nil {
		t.Error("expected error for invalid address")
	}
}

func TestWatchListAddRemove(t *testing.T) {
	wl := NewWatchList()
	wp, _ := ParseWatchpoint("D020")
	wl.Add(wp)
	if len(wl.All()) != 1 {
		t.Errorf("expected 1 watchpoint, got %d", len(wl.All()))
	}
	wl.Remove(0)
	if len(wl.All()) != 0 {
		t.Errorf("expected 0 watchpoints after remove")
	}
}

func TestWatchListCheckHit(t *testing.T) {
	wl := NewWatchList()
	wp, _ := ParseWatchpoint("D020")
	wl.Add(wp)

	hit := wl.Check(0xD020, 0x05, false)
	if !hit {
		t.Error("expected hit on address D020")
	}
	if wl.All()[0].HitCount != 1 {
		t.Errorf("expected HitCount=1, got %d", wl.All()[0].HitCount)
	}
}

func TestWatchListCheckCondition(t *testing.T) {
	wl := NewWatchList()
	wp, _ := ParseWatchpoint("D020=05")
	wl.Add(wp)

	// Falscher Wert — kein Condition-Hit, aber normaler Hit
	hit := wl.Check(0xD020, 0x03, true)
	if !hit {
		t.Error("expected address hit")
	}
	if wl.All()[0].ConditionMet {
		t.Error("condition should not be met for value 03")
	}

	// Richtiger Wert — Condition-Hit
	hit = wl.Check(0xD020, 0x05, true)
	if !hit {
		t.Error("expected address hit")
	}
	if !wl.All()[0].ConditionMet {
		t.Error("condition should be met for value 05")
	}
}
