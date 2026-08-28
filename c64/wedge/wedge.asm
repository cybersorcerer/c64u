// Ultimate wedge - autostart cartridge that gets out of the way.
//
// A plain 8K cartridge sits at $8000 forever and costs 8 KB of BASIC memory:
// the machine reports 30719 bytes free instead of 38911. For a tool whose whole
// point is comfort at the BASIC prompt that is the wrong trade.
//
// This is a Magic Desk cartridge (CRT hardware type 19) instead. Its bank
// register at $DE00 has a disable bit, so the ROM copies what it needs into RAM,
// switches itself off, and BASIC comes up with all of its memory.
//
// The code that performs the switch-off cannot itself live in the ROM being
// switched off, so it is assembled with .pseudopc for $C000, copied there, and
// entered before the ROM disappears.
//
// Built by the Makefile in this directory.

.const MAGICDESK_CTRL = $de00           // bit 7 set disables the cartridge
.const RAM_CODE       = $c000           // 4 KB of RAM the KERNAL never uses

.const IOINIT  = $fda3
.const RAMTAS  = $fd50
.const RESTOR  = $fd15
.const CINT    = $ff5b
.const BASIC_COLDSTART = $a000          // vector, not entry point

* = $8000 "Cartridge header"

        .word coldStart                 // $8000: cold start vector
        .word coldStart                 // $8002: warm start vector
        .byte $c3, $c2, $cd, $38, $30   // $8004: "CBM80" autostart signature

coldStart:
        sei
        cld
        ldx #$ff
        txs

        // Copy the resident part into RAM while the ROM is still mapped in.
        ldx #$00
!loop:
        lda residentSource,x
        sta RAM_CODE,x
        lda residentSource + $100,x
        sta RAM_CODE + $100,x
        inx
        bne !loop-

        jmp RAM_CODE                    // continue from RAM

// Everything below is assembled to run at $C000 but stored in the ROM image.
residentSource:
.pseudopc RAM_CODE {

resident:
        // Unmap the cartridge. From here on $8000-$9FFF is plain RAM again, so
        // this must not be executed from the cartridge itself.
        lda #$80
        sta MAGICDESK_CTRL

        // Standard KERNAL start-up. RAMTAS now finds RAM all the way to $A000,
        // so BASIC reports its full 38911 bytes.
        jsr IOINIT
        jsr RAMTAS
        jsr RESTOR
        jsr CINT
        cli

        lda #$06
        sta $d020                       // proof that the cartridge ran

        jmp (BASIC_COLDSTART)
}

// Pad the image to a full 8 KB; the CRT chip packet declares $2000 bytes.
* = $9fff
        .byte $00
