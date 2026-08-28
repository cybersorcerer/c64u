// Play one note on SID voice 1, with the frequency picked for the machine's
// video standard at runtime.
//
// The SID frequency value depends on the CPU clock, which differs between PAL
// and NTSC. A PAL table played on an NTSC machine is roughly a semitone sharp.
// $02A6 holds 1 for PAL and 0 for NTSC.
//
// Build: java -jar KickAss.jar sid-note.asm -o sid-note.prg
// Run:   c64u runners run-prg-upload sid-note.prg

.const SID          = $d400
.const PAL_CLOCK    = 985248
.const NTSC_CLOCK   = 1022727
.const NOTE_HZ      = 440               // A4

// Truncated, not rounded: that is what the official note tables do, so the
// values here match a printed listing exactly.
.function sidFreq(hz, clock) {
        .return floor(hz * 16777216 / clock)
}

.const FREQ_PAL  = sidFreq(NOTE_HZ, PAL_CLOCK)
.const FREQ_NTSC = sidFreq(NOTE_HZ, NTSC_CLOCK)

BasicUpstart2(start)

start:
        lda #$0f
        sta SID + 24            // master volume, no filter mode

        // Set the envelope while the gate is still low. Writing ADSR after
        // gating on may be ignored until the next gate cycle.
        lda #$00
        sta SID + 4             // control: gate off, no waveform selected
        lda #$09                // attack 0 (2 ms), decay 9 (750 ms)
        sta SID + 5
        lda #$00                // sustain 0, release 0
        sta SID + 6

        ldx #<FREQ_PAL
        ldy #>FREQ_PAL
        lda $02a6
        bne !pal+
        ldx #<FREQ_NTSC
        ldy #>FREQ_NTSC
!pal:
        stx SID + 0
        sty SID + 1

        // Waveform and gate in a single write: triangle, gate on.
        lda #%00010001
        sta SID + 4

        rts                     // the note decays to silence on its own
