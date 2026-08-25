# CIA 6526 - Keyboard, Joysticks, Timers, Serial Bus

Two identical chips with different wiring:

| | Base | Interrupt line | Wired to |
|---|---|---|---|
| CIA 1 | `$DC00` | IRQ | Keyboard matrix, both joystick ports, paddles |
| CIA 2 | `$DD00` | NMI | Serial (IEC) bus, user port, RS-232, VIC bank select |

Both mirror every 16 bytes through their page, so `$DC00` and `$DC10` are the same register.
Use the base addresses.

## Register layout (offset from the base)

| Offset | Name | Function |
|---|---|---|
| `+0` | PRA | Port A data |
| `+1` | PRB | Port B data |
| `+2` | DDRA | Port A direction, `1` = output |
| `+3` | DDRB | Port B direction |
| `+4` | TA LO | Timer A low byte |
| `+5` | TA HI | Timer A high byte |
| `+6` | TB LO | Timer B low byte |
| `+7` | TB HI | Timer B high byte |
| `+8` | TOD 10THS | Time of day, tenths of a second |
| `+9` | TOD SEC | Seconds (BCD) |
| `+A` | TOD MIN | Minutes (BCD) |
| `+B` | TOD HR | Hours (BCD), bit 7 = PM |
| `+C` | SDR | Serial shift register |
| `+D` | ICR | Interrupt control, read status / write mask |
| `+E` | CRA | Control register A |
| `+F` | CRB | Control register B |

## CIA 1 - keyboard and joysticks

`$DC00` (port A) selects a keyboard column, `$DC01` (port B) reads the rows. Both are **active
low**: write a `0` to the column you want, read a `0` for every key held down in it.

```asm
        lda #%11111110          // select column 0 (PA0)
        sta $dc00
        lda $dc01               // bit n low = the key in row n is pressed
```

### Keyboard matrix

Column across, row down. A key is at the intersection of a `$DC00` bit and a `$DC01` bit.

| | PA0 | PA1 | PA2 | PA3 | PA4 | PA5 | PA6 | PA7 |
|---|---|---|---|---|---|---|---|---|
| **PB0** | INS/DEL | 3 | 5 | 7 | 9 | + | £ | 1 |
| **PB1** | RETURN | W | R | Y | I | P | * | ← |
| **PB2** | CRSR ←→ | A | D | G | J | L | ; | CTRL |
| **PB3** | F7 | 4 | 6 | 8 | 0 | - | HOME | 2 |
| **PB4** | F1 | Z | C | B | M | . | RSHIFT | SPACE |
| **PB5** | F3 | S | F | H | K | : | = | C= |
| **PB6** | F5 | E | T | U | O | @ | ↑ | Q |
| **PB7** | CRSR ↑↓ | LSHIFT | X | V | N | , | / | RUN/STOP |

RESTORE is not in the matrix - it is wired directly to the NMI line.

### Joysticks

| Port | Register | Bits 0-4 |
|---|---|---|
| Port 2 | `$DC00` | up, down, left, right, fire - active low |
| Port 1 | `$DC01` | same |

```asm
        lda $dc00
        and #%00011111
        cmp #%00011111          // nothing pressed
```

**Trap:** reading joystick 1 on `$DC01` collides with the keyboard, because the same lines carry
both. A joystick held left looks like a key press and vice versa. Games work around it by
disabling the keyboard scan interrupt; simple programs usually prefer port 2.

## CIA 2 - serial bus, user port, VIC bank

`$DD00` (port A):

| Bits | Function |
|---|---|
| 0-1 | VIC bank select, **inverted**: `%11` = bank 0 `$0000`, `%10` = bank 1 `$4000`, `%01` = bank 2 `$8000`, `%00` = bank 3 `$C000` |
| 2 | RS-232 TXD |
| 3 | Serial ATN out |
| 4 | Serial CLK out |
| 5 | Serial DATA out |
| 6 | Serial CLK in |
| 7 | Serial DATA in |

Set bits 0-1 to outputs in `$DD02` before changing the VIC bank, and preserve the other bits:

```asm
        lda $dd02
        ora #%00000011
        sta $dd02
        lda $dd00
        and #%11111100
        ora #%00000010          // bank 1: $4000-$7FFF
        sta $dd00
```

`$DD01` (port B) is the user port, also used for RS-232.

**Trap:** CIA 2 raises **NMI**, not IRQ. An NMI cannot be masked with `sei`. Code that must not
be interrupted has to point the NMI vector at an `rti`, and even that leaves a window.

## Timers

Two 16-bit down counters per chip. Writing `TA LO`/`TA HI` sets the latch; the counter is
loaded from it on a forced load or, in continuous mode, on underflow. Underflow sets the
matching ICR bit.

**Trap:** reading `TA LO`/`TA HI` returns the **current counter**, not the latch. There is no way
to read back the interval you programmed, and two reads a moment apart give different values.
Keep a copy in RAM if you need to know it.

At the PAL CPU clock of 985248 Hz, a latch value of `N` produces an interrupt every `N+1`
cycles. The KERNAL programs CIA 1 timer A to `$4025` (16421), giving the familiar ~60 Hz system
interrupt.

```asm
        lda #<16421
        sta $dc04
        lda #>16421
        sta $dc05
        lda #%10000001
        sta $dc0d               // enable timer A interrupt
        lda #%00010001
        sta $dc0e               // force load + start, continuous
```

### CRA (`+E`)

| Bit | Function |
|---|---|
| 0 | Start timer A |
| 1 | Timer A output appears on PB6 |
| 2 | Output mode: `0` toggle, `1` pulse |
| 3 | Run mode: `0` continuous, `1` one-shot |
| 4 | Force load - a strobe, always reads `0` |
| 5 | Input mode: `0` count phi2 cycles, `1` count CNT pin pulses |
| 6 | Serial port: `0` input, `1` output |
| 7 | TOD clock: `0` = 60 Hz mains, `1` = 50 Hz |

### CRB (`+F`)

Bits 0-4 as CRA, for timer B. Then:

| Bit | Function |
|---|---|
| 5-6 | Input: `00` phi2, `01` CNT, `10` timer A underflow, `11` timer A underflow while CNT is high |
| 7 | `0` writes to TOD set the clock, `1` they set the alarm |

Mode `%10` chains the timers into one 32-bit counter - the standard way to get intervals longer
than 65536 cycles.

## Interrupt control (`+D`)

**Reading** returns the status and **clears it**:

| Bit | Source |
|---|---|
| 0 | Timer A underflow |
| 1 | Timer B underflow |
| 2 | TOD alarm |
| 3 | Serial shift register full or empty |
| 4 | FLAG pin (cassette read / user port) |
| 7 | An interrupt from this chip is pending |

**Writing** sets or clears the mask: bit 7 decides the direction, bits 0-4 select the sources.

```asm
        lda #%01111111
        sta $dc0d               // bit 7 = 0: clear every source
        lda $dc0d               // read to acknowledge anything pending
```

Those two lines are the standard opening of any program that takes over the interrupt system,
together with the same pair on `$DD0D`. Skipping the read leaves a pending interrupt that fires
the moment `cli` runs.

**Trap:** reading ICR clears it. Read it once into a register and test that copy; a second read
returns zeros and the source is lost.

## Time of day

A real-time clock in BCD, ticking from the mains frequency selected by CRA bit 7. Registers must
be read from hours down to tenths - reading the hours latches the whole value, reading the
tenths releases it. Reading out of order gives torn values. Writing follows the same order in
reverse.

Rarely worth using for timing; the jiffy clock at `$00A0` or a CIA timer is simpler.

## Disabling everything

```asm
        sei
        lda #$7f
        sta $dc0d               // mask off all CIA 1 sources
        sta $dd0d               // and CIA 2
        lda $dc0d               // acknowledge
        lda $dd0d
```

After this only the VIC can interrupt, which is what raster effects want. See `vic-ii.md`.

## Why injected keystrokes cannot drive games

The matrix is scanned: `$DC01` reflects the physical key lines at the moment of reading. Writing
a value into `$DC00` or `$DC01` from outside - over DMA, for instance - is overwritten by the
next scan before a polling game reads it. Injecting into the keyboard buffer at `$0277` works
only for code that reads through the KERNAL, which means BASIC and `GET`/`INPUT`, not games.
