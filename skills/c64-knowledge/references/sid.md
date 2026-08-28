# SID 6581/8580 Sound Chip

Base `$D400`, mirrored every `$20` to `$D7FF`. Three voices; voice 1 starts at `$D400`,
voice 2 at `$D407`, voice 3 at `$D40E`.

## Per-voice registers (offset from the voice base)

| Offset | Register | Function |
|---|---|---|
| `+0` | FREQ LO | Frequency, low byte |
| `+1` | FREQ HI | Frequency, high byte |
| `+2` | PW LO | Pulse width, low byte |
| `+3` | PW HI | Pulse width, bits 8-11 (bits 4-7 unused) |
| `+4` | CONTROL | Waveform and gate, see below |
| `+5` | ATTACK/DECAY | Attack in bits 7-4, decay in bits 3-0 |
| `+6` | SUSTAIN/RELEASE | Sustain level in bits 7-4, release in bits 3-0 |

### Control register bits

| Bit | Name | Function |
|---|---|---|
| 7 | NOISE | Noise waveform |
| 6 | PULSE | Pulse waveform |
| 5 | SAWTOOTH | Sawtooth waveform |
| 4 | TRIANGLE | Triangle waveform |
| 3 | TEST | Freezes and resets the oscillator while set |
| 2 | RING | Ring modulation with the previous voice (triangle only) |
| 1 | SYNC | Hard sync with the previous voice |
| 0 | GATE | `1` starts attack/decay/sustain, `0` starts release |

"Previous voice" wraps: voice 1 syncs/rings with voice 3.

## Global registers

| Address | Function |
|---|---|
| `$D415` | Filter cutoff, low 3 bits |
| `$D416` | Filter cutoff, high 8 bits |
| `$D417` | Resonance in bits 7-4; filter enable for external in bit 3, voice 3 in bit 2, voice 2 in bit 1, voice 1 in bit 0 |
| `$D418` | Volume in bits 3-0; low-pass bit 4, band-pass bit 5, high-pass bit 6, voice 3 mute bit 7 |
| `$D419` | Paddle X (read only) |
| `$D41A` | Paddle Y (read only) |
| `$D41B` | Voice 3 oscillator output (read only) - a free random/waveform source |
| `$D41C` | Voice 3 envelope output (read only) |

**Write-only:** `$D400-$D418` cannot be read back. Keep a shadow copy in RAM if you need the
current value, especially for `$D418` where volume and filter mode share one byte.

## Frequency

```
Fout = (Fn * Fclk) / 16777216        ->        Fn = Hz * 16777216 / Fclk
```

| System | Fclk | Multiplier (16777216 / Fclk) |
|---|---|---|
| PAL | 985248 Hz | 17.02842 |
| NTSC | 1022727 Hz | 16.40439 |

**Truncate, do not round.** The note table in the official C64 Ultimate user guide (appendix J)
takes the floor of the result. Checked against all 95 entries: flooring reproduces every PAL
value exactly, while rounding disagrees on 43 of them by one. The audible difference is
nothing - one unit is about 0.01% - but code that rounds will not match published tables, and
that costs time when comparing against a listing.

So A4 = 440 Hz is `$1D44` (7492) on PAL and `$1C31` (7217) on NTSC - a difference of about
two thirds of a semitone.

```
Fn = floor(Hz * 16777216 / Fclk)
```
**Always branch on `$02A6` or assemble both tables.** See `examples/sid-note.asm`.

In Kick Assembler, compute the table at assemble time rather than hand-typing it:

```asm
.function sidFreq(hz, clock) {
    .return floor(hz * 16777216 / clock)
}
.const A4_PAL = sidFreq(440, 985248)
```

## Envelope rates

Attack is the time from silence to peak; decay and release are the times from peak to the
sustain level and from sustain to silence.

| Value | Attack | Decay / Release |
|---|---|---|
| 0 | 2 ms | 6 ms |
| 1 | 8 ms | 24 ms |
| 2 | 16 ms | 48 ms |
| 3 | 24 ms | 72 ms |
| 4 | 38 ms | 114 ms |
| 5 | 56 ms | 168 ms |
| 6 | 68 ms | 204 ms |
| 7 | 80 ms | 240 ms |
| 8 | 100 ms | 300 ms |
| 9 | 250 ms | 750 ms |
| 10 | 500 ms | 1.5 s |
| 11 | 800 ms | 2.4 s |
| 12 | 1 s | 3 s |
| 13 | 3 s | 9 s |
| 14 | 5 s | 15 s |
| 15 | 8 s | 24 s |

Sustain is a level, not a time: `0` = silence, `15` = full.

## Playing a note correctly

```asm
        lda #$0f
        sta $d418          // master volume, once at init

        // 1. gate off and set the envelope BEFORE the note starts
        lda #$00
        sta $d404          // control: gate off, no waveform
        lda #$09
        sta $d405          // attack 0, decay 9
        lda #$00
        sta $d406          // sustain 0, release 0

        // 2. frequency
        lda #<A4_PAL
        sta $d400
        lda #>A4_PAL
        sta $d401

        // 3. waveform + gate on, in one write
        lda #%00010001     // triangle + gate
        sta $d404
```

To stop the note, clear only bit 0 and leave the waveform bits:

```asm
        lda #%00010000     // triangle, gate off -> release phase
        sta $d404
```

## Practice rules

1. **Set ADSR before gating on.** Changing attack/decay while the gate is already high may be
   ignored until the next gate cycle. This is the classic "my attack does nothing" bug.
2. **Default to triangle for melodic voices.** It is the most forgiving waveform: soft, stable
   pitch, and audible at any pulse-width setting because it has none. Reach for pulse when you
   want a reedy or hollow timbre, sawtooth for brass and leads.
3. **A pulse waveform with pulse width `$000` or `$FFF` is silent.** Set `$D402`/`$D403`
   whenever bit 6 of the control register is set; `$0800` (50% duty) is a safe start.
4. **Never write `$00` into `$D418` to silence a note.** That kills the master volume for all
   three voices and produces a click. Gate off instead.
5. **Use the TEST bit to reset an oscillator** before a hard-attack note, so the waveform
   always starts at the same phase: set bit 3, write the frequency, clear bit 3 and set gate.
6. **Leave a gap between release and the next gate-on.** Retriggering while the previous note is
   still releasing on the 6581 can drop the attack entirely (the ADSR delay bug). One frame is
   usually enough.
7. **6581 versus 8580.** The 6581 needs the volume register bumped for audible filtered output
   and has a much stronger filter; the 8580 filter is cleaner and its pulse-width modulation is
   less noisy. Combined waveforms (two waveform bits at once) sound different between the two
   chips and are not portable.
8. **Voice 3 is your noise source.** Set it to noise, mute it with `$D418` bit 7, and read
   `$D41B` for a fast random byte.
