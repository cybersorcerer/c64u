# C64U Ultimate Wedge

A cartridge that adds `@` commands to stock Commodore BASIC V2, so the Ultimate
filesystem is usable from the READY prompt without a disk image and without
JiffyDOS.

```
@$          list the current directory
@CD:NAME    change directory
/NAME       load
↑NAME       load and run
```

Everything goes through the Ultimate Command Interface, so all four commands act
on the same directory - the one `@$` just showed. No SoftIEC, no disk image, and
no second notion of a "current directory" to get confused by.

`@$` prints with CHROUT and, unlike `LOAD"$",8`, leaves the BASIC program in
memory untouched - which on a stock machine is the whole point. RUN/STOP aborts
a long listing.

`↑` is the up arrow key, PETSCII `$5E`. From the host, `c64u machine sendkey`
accepts `^` for it.

## Build

```sh
make                    # build/wedge.crt
make run                # upload and start it, until the next reboot
make install            # copy it into /flash/carts on the device
```

`make run` needs `tools/c64u/build/c64u` built, and both targets need Kick
Assembler - pass `KICKASS=/path/to/KickAss.jar` if it is not in
`~/.local/bin/KickAssembler`.

## Installing permanently

```sh
make install
c64u config set "C64 and Cartridge Settings" Cartridge wedge.crt
c64u config save-to-flash
```

`/flash/carts` is hardcoded in the firmware; the cartridge cannot live anywhere
else. `save-to-flash` is what makes the selection survive a power cycle.

The Command Interface has to be enabled, or the wedge reports
`?COMMAND INTERFACE DISABLED`:

```sh
c64u config set "C64 and Cartridge Settings" "Command Interface" Enabled
```

## How it works, and why

**It is a cartridge** because a wedge loaded as a PRG is gone after every reset
and the Ultimate has no boot-PRG setting. A cartridge in `/flash/carts` is there
from power-on.

**It is a Magic Desk cartridge** (CRT hardware type 19) rather than a plain 8K
one. A plain cartridge occupies `$8000` forever and costs 8 KB of BASIC memory -
the machine reports 30719 bytes free instead of 38911. Magic Desk has a disable
bit in its bank register at `$DE00`, so the ROM copies itself to `$C000`,
switches itself off, and BASIC comes up with all of its memory.

**The hook is installed from an interrupt.** BASIC's cold start rewrites
`$0300-$030B`, so a hook installed before it is silently wiped. Reproducing the
cold start inline is not portable either: a JiffyDOS machine calls `$E4B7` where
a stock one calls `$E453`. Instead the ROM starts BASIC untouched, and the IRQ
handler waits until `$0308` holds the default dispatcher - proof that BASIC has
set its vectors - before hooking it.

## Three traps this ran into

Worth knowing before changing the code, because none of them announce
themselves:

**BASIC tokenises operators.** By the time a line reaches the statement
dispatcher, `/` is `$AD` and `↑` is `$AE`, not `$2F` and `$5E`. Comparing
against the character codes never matches. `@` is not an operator and survives
unchanged, which is why the `@` commands worked while `/` silently did not.

**CHRGET's flags matter.** BASIC's statement executor reads the zero and carry
flags that CHRGET left behind. A `cmp` between the two destroys them, and the
symptom is not a crash but nonsense: `PRINT` executes as `PRINT#` and reports
`?FILE NOT OPEN`. The comparisons are wrapped in `php`/`plp` for that reason.

**BASIC's CLR is not a subroutine.** `$A659` ends in `PLA/TAY/PLA` and juggles
the stack, so `jsr $A659` never comes back properly. The pointers it would set
are written directly instead, and a load-and-run hands a tokenised `RUN` line to
BASIC rather than entering the RUN routine.

## Verified

All four commands were run on a C64 Ultimate with the **stock** KERNAL:

```
@$            -> SPRITE.PRG / VOID.RAIDER.PRG
@CD:USB0      -> 00,OK
/SPRITE.PRG   -> LOADED, and LIST shows "10 SYS2062"
↑SPRITE.PRG   -> program runs: $07F8=$30, $D015=$01, $D027 low nibble $E
```

The loaded bytes were compared against the source file and matched exactly.
Normal BASIC was checked alongside: `PRINT 6*7` gives 42, and a typed program
lists and runs. BASIC reported 38911 bytes free, and `$8000` accepted writes,
confirming the cartridge had unmapped itself.

## Note for JiffyDOS machines

JiffyDOS intercepts `@` before BASIC's statement dispatcher, so its own wedge
answers instead of this one. That is harmless - JiffyDOS already provides `@$`,
`@CD:` and `/NAME` - but it means the commands here only take effect on a
machine running the stock KERNAL.
