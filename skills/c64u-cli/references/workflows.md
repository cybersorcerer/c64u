# Workflows

Recipes that have been run against real hardware. Each ends with a check, because a command
that returns successfully has only told you it was accepted.

## Assemble, run, verify

The core loop. Never stop after step 2.

```sh
java -jar KickAss.jar program.asm -o program.prg
c64u runners run-prg-upload program.prg
sleep 2
c64u machine read-mem 07f8 --length 8        # whatever the program should have changed
```

Pick a check that only the program could satisfy. A screen full of spaces proves nothing - the
screen was already blank. Write a distinctive pattern first, or read a location the program
computes.

Worked example, verifying a sprite setup wrote its pointer:

```sh
c64u runners run-prg-upload sprite-setup.prg
sleep 2
c64u machine read-mem 07f8 --length 1        # expect the data address / 64
```

## Prove a round trip instead of a single value

When testing something that saves and restores, use a pattern that cannot occur by accident, and
add a negative control - a case that must fail. Without it, a passing check may just mean the
code never ran.

```sh
c64u machine write-mem 0400 08050c0c0f       # distinctive pattern
# ... trigger the save ...
c64u machine write-mem 0400 ffffffffff       # destroy it
# ... trigger the restore ...
c64u machine read-mem 0400 --length 5        # expect the pattern back

# negative control: change one byte, the comparison must now fail
c64u machine write-mem 0400 41
# ... trigger the compare ... expect "different"
```

## Drive a running program by key

Useful when the program has a key-driven menu. Reset first so the starting state is known.

```sh
c64u machine reset
sleep 5
c64u runners run-prg-upload menu.prg
sleep 2
c64u machine sendkey '\f1'
sleep 1
c64u machine read-mem 0400 --length 16       # did F1 do what it should?
```

For a PETSCII code the escapes do not cover, write the buffer directly - see `limits.md`.

## Type into BASIC

```sh
c64u machine reset
sleep 5
c64u machine sendkey 'PRINT "HELLO"\n'
sleep 2
c64u machine read-mem 0400 --to 07e7 -o screen.bin
```

The editor is in uppercase/graphics mode after a reset, so send uppercase. Screen RAM holds
screen codes, not PETSCII - see the `c64-knowledge` skill.

## Capture the screen for inspection

```sh
c64u machine read-mem 0400 --to 07e7 --raw > screen.bin     # 1000 bytes of screen codes
c64u machine read-mem d800 --to dbe7 --raw > colour.bin     # low nibble is the colour
```

## Work from the BASIC prompt without disk images

Reach for this before building a `.d64`. SoftIEC serves a directory of the Ultimate filesystem
as an IEC device, so the C64 loads from it directly.

```sh
c64u drives softiec status                       # is it on, and what does it serve?
c64u drives softiec enable --bus-id 11
c64u drives softiec root /Usb0/development       # needs the C64 at the READY prompt
```

Then, on the C64:

```basic
LOAD"$",11 : LIST      REM directory
LOAD"NAME",11,1        REM load, honouring the load address
```

With a JiffyDOS or CMD wedge the short forms work, and listing does **not** destroy the BASIC
program the way `LOAD"$"` does:

```
@#11          direct the wedge at SoftIEC
@$            list
@CD:SUBDIR    change directory
/NAME         load
```

Copy files in with `c64u fs upload`, and they appear in the next directory listing. No image to
rebuild, no remount.

**Check whether it is switched on before concluding it does not work** — `IEC Drive` defaults to
disabled, and a disabled SoftIEC simply does not answer on the bus.

## Build and mount a disk image

```sh
c64u files create-d64 mydisk.d64 --name "MY DISK"
c64u files pack-d64 ./programs mydisk.d64 --name "MY DISK" --id 01
c64u files info mydisk.d64

c64u drives mount-upload a mydisk.d64
c64u drives list --json
```

Then load from the C64 side with `LOAD"*",8,1` - or skip the disk entirely and use
`runners run-prg-upload`, which is faster for a single program.

## Watch while automating

```sh
c64u streams listen video &                  # native window, U64 only
# ... run the automation ...
kill %1
```

Handy when a check fails and you cannot tell whether the program crashed, hung, or simply did
something different from what you expected.

## Check the REU before relying on it

```sh
c64u config show "C64 and Cartridge Settings" | grep -i "expansion\|REU"
```

If it reports `RAM Expansion Unit: Disabled`, REU code will find no hardware:

```sh
c64u config set "C64 and Cartridge Settings" "RAM Expansion Unit" Enabled
c64u config save-to-flash
```

## Script-friendly output

```sh
host=$(c64u info --json | jq -r .hostname)
bytes=$(c64u machine read-mem 07f8 --length 1 --json | jq -r .data)
c64u machine read-mem 0400 --length 1000 --raw | xxd | head
```

`--raw` prints nothing but the bytes, so it is safe in a pipe even when a file was written at
the same time.
