# C64U Ultimate Wedge

A cartridge that adds `@` commands to stock Commodore BASIC V2, so the Ultimate
filesystem is usable from the READY prompt without a disk image and without
JiffyDOS.

```
@             show the current path
@$            list the current directory
@CD:NAME      change directory        @MD:NAME   create directory
@RM:NAME      delete a file           @SV:NAME   save the BASIC program
@MT9:NAME     mount a disk image      @SW9       swap to the next disk
/NAME         load                    ↑NAME      load and run
```

The digit in `@MT` and `@SW` is the drive's bus id and may be left out. It is
worth giving: drive A is not always 8 — on the machine this was developed
against it answers on 9, and mounting without the id reports
`90,DRIVE NOT PRESENT`.

`@MT` and `@SW` are the part no other wedge offers, because they are device
control rather than file access.

## Two prefixes, because of JiffyDOS

JiffyDOS claims `@`, `/` and the up arrow for its own wedge and intercepts them
**before** BASIC's statement dispatcher, so on a JiffyDOS machine those forms
never reach this cartridge at all.

The cartridge detects JiffyDOS at boot, by scanning the KERNAL for its banner
string rather than checking a fixed address, and adapts:

| Machine | Classic `@` `/` `↑` | `&` prefix |
|---|---|---|
| Stock KERNAL | wedge | wedge |
| JiffyDOS | JiffyDOS | wedge |

`&` therefore always reaches the wedge, and the banner shows which prefix is
live on the machine it just booted on. The two can be installed together:
JiffyDOS keeps making disk loading fast, the wedge reaches the Ultimate
filesystem.

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

**`@` suppresses tokenisation, `&` does not.** Measured on hardware:
`@CD:PRINTER` reaches the dispatcher as plain text, while `&CD:PRINTER` arrives
as the PRINT token followed by `ER`. `%` behaves like `@`, but JiffyDOS claims it
too. So filenames are expanded back from the keyword table at `$A09E` before
being sent - otherwise every file whose name contains a BASIC keyword would be
unreachable under the `&` prefix.

**CHRGET leaves the pointer on the character it returned.** Mixing CHRGET with
direct reads through `$7A` re-reads that byte - which turned `@MD:TESTDIR` into
a request to create `D:TESTDIR`. Filenames are read directly rather than with
CHRGET anyway, because CHRGET also skips spaces.

## Verified

On a C64 Ultimate with the **stock** KERNAL:

```
@$            -> SPRITE.PRG / VOID.RAIDER.PRG
@CD:USB0      -> 00,OK
/SPRITE.PRG   -> LOADED, and LIST shows "10 SYS2062"
↑SPRITE.PRG   -> program runs: $07F8=$30, $D015=$01, $D027 low nibble $E
&$            -> same listing, so '&' works there too
```

On the same machine with the **JiffyDOS** KERNAL:

```
&$              -> SPRITE.PRG / VOID.RAIDER.PRG
&/SPRITE.PRG    -> LOADED
&↑SPRITE.PRG    -> program runs, $07F8=$30
/SPRITE.PRG     -> "SEARCHING FOR SPRITE.PRG", ?FILE NOT FOUND
                   (JiffyDOS handled it, as intended)
&MD:TESTDIR     -> created; &CD:TESTDIR then &  ->  /USB0/TEMP/TESTDIR/
&CD:..          -> 00,OK      &RM:TESTDIR  -> 00,OK
&MT9:WEDGE.D64  -> 00,OK, and "drives list" shows it on drive A
&SW9            -> 00,OK
```

The save/load round trip was run with a filename that contains a BASIC keyword,
which is where the token expansion earns its keep:

```
10 PRINT"WEDGE OK"
&SV:PRINTER.PRG   -> SAVED, 20 bytes on the device, named PRINTER.PRG
NEW
&/PRINTER.PRG     -> LOADED; LIST shows the line; RUN prints WEDGE OK
```

The loaded bytes were compared against the source file and matched exactly.
Normal BASIC was checked alongside: `PRINT 6*7` gives 42, and a typed program
lists and runs. BASIC reported 38911 bytes free, and `$8000` accepted writes,
confirming the cartridge had unmapped itself.

## Do you still need JiffyDOS?

They solve different problems and overlap less than it looks.

JiffyDOS speeds up loading over the **serial bus** - it patches both the C64
KERNAL and the drive ROM. That helps every program that loads through the
KERNAL from device 8 or 9, including games pulling their own data off a mounted
D64, without anyone typing a command. This cartridge does nothing there: it
reads the Ultimate filesystem, not the contents of a disk image.

This cartridge reaches that filesystem directly, with subdirectories and one
consistent current directory for listing, changing and loading - which JiffyDOS
cannot do at all.

So: keep JiffyDOS if you run software from disk images. Use the wedge for your
own files on the Ultimate filesystem. With the `&` prefix both are installed at
the same time without getting in each other's way.
