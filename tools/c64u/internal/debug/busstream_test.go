package debug

import (
	"strings"
	"testing"
)

// Signale sind active-low: ein gelöschtes Bit heisst, die Leitung ist aktiv.
func TestFormat6510_SignalsWhenAsserted(t *testing.T) {
	const base = uint32(1)<<31 | 1<<26 | 1<<24 | 0x31<<16 | 0x0314

	asserted := base | 1<<29 // GAME, IRQ, NMI aktiv; EXROM ruht
	got := format6510(decode6510(asserted))
	for _, want := range []string{" GAME", " IRQ", " NMI"} {
		if !strings.Contains(got, want) {
			t.Errorf("asserted: %q lacks %q", got, want)
		}
	}
	if strings.Contains(got, "EXROM") {
		t.Errorf("asserted: %q shows idle EXROM", got)
	}

	idle := base | 1<<30 | 1<<29 | 1<<27 | 1<<25
	got = format6510(decode6510(idle))
	for _, bad := range []string{"GAME", "EXROM", "IRQ", "NMI"} {
		if strings.Contains(got, bad) {
			t.Errorf("idle: %q shows %q", got, bad)
		}
	}
}
