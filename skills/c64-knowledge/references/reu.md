# REU - RAM Expansion Unit

The REU adds external RAM reachable only through DMA. The C64 cannot execute code from it and
cannot address it directly; every byte must be transferred by the RAM Expansion Controller
(REC), which appears at `$DF00-$DF0A` in I/O area 2.

Memory is organised in **64 KB banks** selected by a single byte, giving a theoretical maximum
of 16 MB.

| Model | Size |
|---|---|
| 1700 | 128 KB |
| 1764 | 256 KB |
| 1750 | 512 KB |
| C64 Ultimate / TC64 / VICE | configurable, up to 16 MB |

## REC registers

| Address | Register | Access |
|---|---|---|
| `$DF00` | Status | read only |
| `$DF01` | Command | write |
| `$DF02/$DF03` | C64 address, low/high | read/write |
| `$DF04/$DF05` | REU address, low/high | read/write |
| `$DF06` | REU bank | write |
| `$DF07/$DF08` | Transfer length in bytes, low/high | read/write |
| `$DF09` | Interrupt mask | write |
| `$DF0A` | Address control | write |

### `$DF00` - status (read only)

| Bit | Meaning |
|---|---|
| 7 | An interrupt is pending |
| 6 | Transfer complete |
| 5 | VERIFY found a difference |
| 4 | Size: `0` = 128 KB, `1` = 256 KB or more |
| 3-0 | Version - never properly implemented, ignore |

**Bits 7-5 are cleared by reading.** Read the register once before an operation if you intend
to test it afterwards.

Bit 4 distinguishes only 128 KB from "more". The exact size cannot be read; it must be probed -
see below.

### `$DF01` - command

| Bit | Meaning |
|---|---|
| 7 | Execute - always `1` |
| 6 | unused |
| 5 | AUTOLOAD: restore `$DF02-$DF08` after the transfer |
| 4 | `1` = start now, `0` = wait for a write to `$FF00` |
| 3-2 | unused |
| 1-0 | `00` STASH (C64 -> REU), `01` FETCH (REU -> C64), `10` SWAP, `11` VERIFY |

**Write `$DF01` last.** The transfer starts on that write (unless bit 4 is clear).

Derived command bytes:

| | STASH | FETCH | SWAP | VERIFY |
|---|---|---|---|---|
| immediate | `$90` | `$91` | `$92` | `$93` |
| immediate + AUTOLOAD | `$B0` | `$B1` | `$B2` | `$B3` |
| wait for `$FF00` | `$80` | `$81` | `$82` | `$83` |
| wait for `$FF00` + AUTOLOAD | `$A0` | `$A1` | `$A2` | `$A3` |

**Trap:** published listings disagree here. Some set the unused bits 6 and 3-2 as well
(`$FC`/`$FD`/`$FE`/`$FF` for the AUTOLOAD-immediate set). Those work on real hardware because
the bits are ignored, but they are not the documented values and they obscure what the command
actually does. Build the byte from the bit fields.

### `$DF09` - interrupt mask

| Bit | Meaning |
|---|---|
| 7 | Enable REU interrupts |
| 6 | Interrupt when a transfer completes |
| 5 | Interrupt when VERIFY finds a difference |
| 4-0 | unused |

It raises an ordinary IRQ, handled through `$0314`/`$0315`. The handler must check `$DF00` to
confirm the REU was the source.

### `$DF0A` - address control

| Bits 7-6 | Effect |
|---|---|
| `%00` | Increment both addresses (default) |
| `%01` | Fix the REU address, increment the C64 address |
| `%10` | Fix the C64 address, increment the REU address |
| `%11` | Fix both |

`%01` fills a C64 memory range from a single REU byte - a very fast memory clear. `%10` does the
same in the other direction to wipe REU memory. Both need only one source byte.

## The four commands

| Command | Direction |
|---|---|
| STASH | C64 -> REU |
| FETCH | REU -> C64 |
| SWAP | exchange both regions |
| VERIFY | compare both regions |

After VERIFY, `$DF00` bit 5 reports `0` for identical and `1` for different. On a difference the
DMA stops and the address and bank registers point at the **first differing byte** - but only
if AUTOLOAD was not used, because AUTOLOAD restores those registers and destroys the location.

## Traps

1. **The CPU is halted during DMA, but the rest of the machine is not.** CIA timers, the raster
   counter and the SID keep running. A timer can pass the value you were waiting for while the
   CPU is frozen. Budget for it in interrupt-driven code.
2. **You cannot execute code in the REU.** Unlike GeoRAM, which windows 256 bytes into the C64
   address space, the REU only ever copies. Overlays must be fetched into RAM first.
3. **Accessing RAM under `$DF00-$DFFF` conflicts with the REC.** If you bank out I/O to reach
   the RAM underneath, writes to `$DF01` no longer reach the controller. That is what command
   bit 4 is for: set up the transfer with bit 4 clear, bank out I/O, then write any value to
   `$FF00` to trigger it.
4. **Transfer length `$0000` means 64 KB**, not zero bytes.
5. **Detection can crash the machine.** Probing `$DF00-$DF0A` with test values is the only
   available method, and a stray write to `$DF01` on a real REU starts a DMA that can overwrite
   anything. Never include `$DF01` in a probe loop.
6. **`$DF00` reading as `$00` does not mean "no REU".** The original 1764 demo disk assumed the
   status register can never be zero, which is false on a 1700 (128 KB, bit 4 clear). Detection
   code copied from that disk misreports 128 KB units.

## Detection

Reliable probe, safe on non-REU machines:

1. Read `$DF00` once to clear bits 7-5.
2. Write `$FF` to `$DF00`. It is read-only, so on a real REU the value must not stick. If
   reading back gives `$FF`, there is no REU (an empty expansion port or a ROM cartridge does
   not retain writes either, but the status register would).
3. Write distinct test values into `$DF02-$DF05` and read them back. Only these four registers
   reliably return what was written (`$DF07`/`$DF08` do too). Skip `$DF01` entirely.

See `examples/reu-detect.asm`.

## Size probing

Since `$DF00` bit 4 only separates 128 KB from "more", size must be measured:

1. STASH a short marker (8 bytes is enough) to offset `$0000` of the **highest bank** of each
   candidate size: bank `$FF` for 16 MB, `$7F` for 8 MB, `$3F` for 4 MB, and so on down to
   bank `$01` for 128 KB. Use a different marker byte per size.
2. VERIFY each of those banks in the same order and watch `$DF00` bit 5.
3. Banks above the real size alias onto lower ones, so their markers were overwritten by later
   writes. The first bank whose VERIFY still matches gives the size.

Bank counts: 128 KB = 2 banks, 256 KB = 4, 512 KB = 8, 1 MB = 16, 2 MB = 32, 4 MB = 64,
8 MB = 128, 16 MB = 256.

## Typical transfer

```asm
.const REU_STATUS  = $df00
.const REU_COMMAND = $df01
.const REU_C64     = $df02
.const REU_ADDR    = $df04
.const REU_BANK    = $df06
.const REU_LEN     = $df07
.const REU_IRQMASK = $df09
.const REU_ADDRCTL = $df0a

.const STASH_AUTOLOAD = $b0
.const FETCH_AUTOLOAD = $b1

        lda #<$0400             // C64 source: screen RAM
        sta REU_C64
        lda #>$0400
        sta REU_C64+1
        lda #$00                // REU target: bank 0, offset $0000
        sta REU_ADDR
        sta REU_ADDR+1
        sta REU_BANK
        sta REU_IRQMASK         // no interrupts
        sta REU_ADDRCTL         // increment both addresses
        lda #<1000              // 40*25 bytes
        sta REU_LEN
        lda #>1000
        sta REU_LEN+1

        lda #STASH_AUTOLOAD
        sta REU_COMMAND         // transfer runs here; CPU resumes when done
```

With AUTOLOAD set, `$DF02-$DF08` survive the transfer, so a matching FETCH needs only the one
final write. See `examples/reu-screen-stash.asm`.

## Enabling an REU for testing

| Target | How |
|---|---|
| C64 Ultimate | Enable the RAM Expansion Unit in the machine configuration; check with `c64u config` |
| VICE | Settings -> Expansion cartridge I/O -> REU settings -> enable, choose a size |
| Turbo Chameleon 64 | Options -> REU size |

Test on real hardware as well as in an emulator. REU detection and size probing in particular
behave differently between implementations.

## Source

Register and command details verified against the REU programming article at
retro-programming.de (`/programming/nachschlagewerk/nice-to-know/reu-programmierung/`), which
is itself based on the Commodore 1764 manual. Code here is written fresh in Kick Assembler
syntax; the article's listings are ACME.
