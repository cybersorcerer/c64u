// Package debugger provides live CPU state reconstruction from 6510 bus traces.
package debugger

import (
	"sync"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/debug"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/disasm"
)

// CPUState holds the last known CPU register state.
type CPUState struct {
	PC               uint16
	SP               uint8
	Status           uint8  // Processor status register (aus Stack-Push rekonstruiert)
	EntriesProcessed uint64
	InstrCount       uint64
}

// Flag-Bits im 6502 Status Register
const (
	FlagN uint8 = 1 << 7
	FlagV uint8 = 1 << 6
	FlagB uint8 = 1 << 4
	FlagD uint8 = 1 << 3
	FlagI uint8 = 1 << 2
	FlagZ uint8 = 1 << 1
	FlagC uint8 = 1 << 0
)

// DisasmLine represents one decoded instruction in the history buffer.
type DisasmLine struct {
	Result    disasm.Result
	IsCurrent bool
}

// IOAccess records a read or write to a known C64 I/O address.
type IOAccess struct {
	Address uint16
	Data    uint8
	Write   bool
	Comment string
}

// Tracker reconstructs CPU state from 6510 bus traces.
//
// Sync-point strategy:
//
//	On a real 6502/6510, the opcode byte is always fetched in the FIRST bus cycle
//	of an instruction. We can reliably identify opcode fetches by looking for
//	"sync points": a CPU write (RW=0) or a BA=0 cycle always completes the current
//	instruction. The NEXT PHI2+RW=1+BA=1 read after a sync point is guaranteed to
//	be an opcode fetch.
//
//	Without sync points we would misidentify operand bytes as opcodes (e.g. the
//	address bytes of JMP $08C9 would be decoded as separate instructions).
type Tracker struct {
	mu sync.RWMutex

	state CPUState

	instrBuf []DisasmLine
	instrMax int

	ioBuf []IOAccess
	ioMax int

	Watches        *WatchList
	watchTriggered bool

	// synced=true: the next PHI2+RW+BA=1 read is an opcode fetch
	synced bool

	// fetch state: 0=waiting for opcode, 1=collecting operands
	fetchPhase  int
	pendingAddr uint16
	pendingData [3]byte
	pendingLen  int
	pendingSize int // expected total instruction size

	// Stack-Write-Tracking für Status-Register-Rekonstruktion
	stackWriteBuf   [3]byte
	stackWriteCount int
}

// NewTracker creates a new CPU state tracker.
func NewTracker(instrHistory, ioHistory int) *Tracker {
	if instrHistory <= 0 {
		instrHistory = 200
	}
	if ioHistory <= 0 {
		ioHistory = 60
	}
	return &Tracker{
		instrMax: instrHistory,
		ioMax:    ioHistory,
		state:    CPUState{SP: 0xFF},
		// Start unsynced: we don't know where we are in the instruction stream yet.
		// We'll sync on the first write or BA=0 cycle.
		synced:  false,
		Watches: NewWatchList(),
	}
}

// Feed processes one decoded bus entry.
func (t *Tracker) Feed(e debug.Entry) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.state.EntriesProcessed++

	// Watchpoint-Prüfung auf jedem Bus-Zugriff (6510 und 1541)
	if t.Watches.Check(e.Address, e.Data, !e.RW) {
		if t.Watches.ConditionTriggered() {
			t.watchTriggered = true
		}
	}

	if e.Is1541 {
		return
	}

	// Stack pointer + Flag-Rekonstruktion aus Stack-Writes ($01xx, RW=0)
	if e.Address >= 0x0100 && e.Address <= 0x01FF {
		t.state.SP = uint8(e.Address & 0xFF)
		// Bei IRQ/BRK/NMI schreibt der 6502 PCH, PCL, P (Status) auf den Stack.
		// Wir tracken die letzen 3 Stack-Writes — der dritte ist P.
		if !e.RW {
			t.stackWriteBuf[0] = t.stackWriteBuf[1]
			t.stackWriteBuf[1] = t.stackWriteBuf[2]
			t.stackWriteBuf[2] = e.Data
			t.stackWriteCount++
			if t.stackWriteCount >= 3 {
				// stackWriteBuf[2] ist das Status-Register
				t.state.Status = t.stackWriteBuf[2]
			}
		}
	}

	// ── Sync point detection ─────────────────────────────────────────────────
	if e.PHI2 && !e.RW {
		t.recordIO(e.Address, e.Data, true)
		t.synced = true
		// Laufenden Fetch NICHT abbrechen: JSR schreibt PCH/PCL auf den Stack
		// während es noch AddrHi liest — fetchPhase=1 darf nicht zurückgesetzt
		// werden, sonst wird AddrHi als neuer Opcode interpretiert.
		if t.fetchPhase == 0 {
			t.pendingLen = 0
		}
		return
	}

	// BA=0: VIC is stealing cycles, CPU is not running — abandon current fetch
	// and re-sync on next valid read.
	if !e.BA {
		t.synced = true  // after BA recovers, next CPU read = opcode
		t.fetchPhase = 0
		t.pendingLen = 0
		return
	}

	// Only process PHI2=1 CPU reads from here on
	if !e.PHI2 || !e.RW {
		return
	}

	// ── Fetch state machine ──────────────────────────────────────────────────
	if !t.synced {
		// Not yet synced: skip this byte, wait for a write or BA=0 first.
		return
	}

	// Bestimmte Reads sind niemals Opcode-Fetches:
	//   $0100-$01FF  Stack-Reads (RTI/RTS zieht PCH/PCL/P zurück)
	//   $FFFA-$FFFF  IRQ/NMI/RST Vektorreads (nach Stack-Push des IRQ)
	// Diese überspringen, synced bleibt true.
	if (e.Address >= 0x0100 && e.Address <= 0x01FF) ||
		e.Address >= 0xFFFA {
		return
	}

	switch t.fetchPhase {
	case 0:
		// This read is a guaranteed opcode fetch.
		t.pendingAddr = e.Address
		t.pendingData[0] = e.Data
		t.pendingLen = 1
		ins := disasm.Lookup(e.Data)
		t.pendingSize = ins.InstructionLength()
		if t.pendingSize == 1 {
			t.commitInstruction()
			// synced stays true: single-byte instruction, next read is next opcode
		} else {
			t.fetchPhase = 1
			// synced stays true while we collect operands
		}

	case 1:
		// Operand byte 1: must be at pendingAddr+1, otherwise we lost sync
		if e.Address != t.pendingAddr+1 {
			t.synced = false
			t.fetchPhase = 0
			t.pendingLen = 0
			return
		}
		t.pendingData[1] = e.Data
		t.pendingLen = 2
		if t.pendingSize == 2 {
			t.commitInstruction()
		} else {
			t.fetchPhase = 2
		}

	case 2:
		// Operand byte 2: must be at pendingAddr+2
		if e.Address != t.pendingAddr+2 {
			t.synced = false
			t.fetchPhase = 0
			t.pendingLen = 0
			return
		}
		t.pendingData[2] = e.Data
		t.pendingLen = 3
		t.commitInstruction()
	}
}

func (t *Tracker) commitInstruction() {
	raw := t.pendingData[:t.pendingLen]
	result, _ := disasm.Disassemble(t.pendingAddr, raw)
	t.state.InstrCount++

	// Kernal/BIOS ROM ($E000-$FFFF) und BASIC ROM ($A000-$BFFF) nicht in den
	// Disassembly-Puffer aufnehmen — sie überfluten sonst den Trace mit
	// IRQ-Handler-Code und verbergen das eigentliche Programm.
	isROM := t.pendingAddr >= 0xA000 && t.pendingAddr <= 0xBFFF ||
		t.pendingAddr >= 0xE000
	if isROM {
		return
	}

	// PC nur auf RAM-Adressen setzen, damit Kernal-IRQs den sichtbaren PC nicht überschreiben
	t.state.PC = t.pendingAddr

	t.instrBuf = append(t.instrBuf, DisasmLine{Result: result, IsCurrent: true})
	if len(t.instrBuf) > t.instrMax {
		t.instrBuf = t.instrBuf[len(t.instrBuf)-t.instrMax:]
	}

	// I/O read annotation for absolute addressing modes
	if result.Comment != "" && t.pendingLen == 3 {
		ioAddr := uint16(t.pendingData[1]) | uint16(t.pendingData[2])<<8
		t.recordIO(ioAddr, 0, false)
	}

	t.fetchPhase = 0
	t.pendingLen = 0
	// synced remains true: after committing, next read = next opcode
}

func (t *Tracker) recordIO(addr uint16, data uint8, write bool) {
	name := ioRegisterName(addr)
	if name == "" {
		return
	}
	t.ioBuf = append(t.ioBuf, IOAccess{
		Address: addr,
		Data:    data,
		Write:   write,
		Comment: name,
	})
	if len(t.ioBuf) > t.ioMax {
		t.ioBuf = t.ioBuf[len(t.ioBuf)-t.ioMax:]
	}
}

// State returns a snapshot of the current CPU state.
func (t *Tracker) State() CPUState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

// Instructions returns a copy of the instruction history.
func (t *Tracker) Instructions() []DisasmLine {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]DisasmLine, len(t.instrBuf))
	copy(out, t.instrBuf)
	return out
}

// IOAccesses returns a copy of the I/O access history.
func (t *Tracker) IOAccesses() []IOAccess {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]IOAccess, len(t.ioBuf))
	copy(out, t.ioBuf)
	return out
}

// WatchTriggered gibt true zurück wenn eine Watchpoint-Bedingung ausgelöst hat.
// Setzt das Flag danach zurück.
func (t *Tracker) WatchTriggered() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	triggered := t.watchTriggered
	t.watchTriggered = false
	t.Watches.ClearConditions()
	return triggered
}

func ioRegisterName(addr uint16) string {
	names := map[uint16]string{
		0xD011: "VIC SCROLY", 0xD012: "VIC RASTER", 0xD015: "VIC SPENA",
		0xD016: "VIC SCROLX", 0xD018: "VIC VMCSB", 0xD019: "VIC VICIRQ",
		0xD01A: "VIC IRQMASK", 0xD020: "VIC EXTCOL", 0xD021: "VIC BGCOL0",
		0xD022: "VIC BGCOL1", 0xD023: "VIC BGCOL2",
		0xD400: "SID V1 FREQ LO", 0xD401: "SID V1 FREQ HI",
		0xD404: "SID V1 CTRL", 0xD405: "SID V1 AD", 0xD406: "SID V1 SR",
		0xD40B: "SID V2 CTRL", 0xD412: "SID V3 CTRL",
		0xD417: "SID RESFILT", 0xD418: "SID SIGVOL",
		0xDC00: "CIA1 PRA", 0xDC01: "CIA1 PRB",
		0xDC04: "CIA1 TA LO", 0xDC05: "CIA1 TA HI",
		0xDC0D: "CIA1 ICR", 0xDC0E: "CIA1 CRA", 0xDC0F: "CIA1 CRB",
		0xDD00: "CIA2 PRA", 0xDD01: "CIA2 PRB",
		0xDD0D: "CIA2 ICR", 0xDD0E: "CIA2 CRA", 0xDD0F: "CIA2 CRB",
		0x0000: "CPU DDR", 0x0001: "CPU PORT",
	}
	return names[addr]
}
