# Limits and Surprising Behaviour

Everything here was checked against a physical C64 Ultimate, not inferred.

## A DMA write persists only if nothing else drives that address

`read-mem` and `write-mem` go through DMA and **do** reach the I/O area at `$D000-$DFFF`, both
reading and writing. What differs is whether the value survives.

| Target | Written value | Why |
|---|---|---|
| `$D020` border colour | sticks | Nothing rewrites it; the VIC just displays what is latched |
| `$0400` screen RAM | sticks | Plain RAM |
| `$DC00` CIA 1 port A | gone within a frame | The KERNAL keyboard scan rewrites it 60 times a second |

So "the write did nothing" almost always means something else owns that register. Before
concluding the CLI failed, ask what code is running on the machine and what it writes.

The corollary for reads: a value read back from a register reflects the hardware, not
necessarily what you wrote.

The other reason a write vanishes is that no RAM is mapped there at the time. `$8000-$9FFF`
is the cartridge window: after a normal reset it reads and writes as RAM, but in the state
`runners run-prg` leaves behind it reads back as `$FF` and swallows writes. `$A000-$BFFF` and
`$E000-$FFFF` read the ROMs, so a write lands in the RAM underneath and the read shows the ROM.
`write-mem` reports success either way - it confirms that the request was accepted, not that
the byte is in memory. Read the address back when it matters.

## RAM under ROM and I/O is out of reach

DMA follows the processor port: where the BASIC ROM, the KERNAL or I/O is banked in, `read-mem`
shows that, and the REST API has no way to read the RAM underneath. Switching the bank from
outside does not help either. `$00`/`$01` read as `00 00` through DMA, and writing `$36` to `$01`
with the machine paused changed nothing at `$A000`: the write reaches RAM, not the CPU port.

Checked: `write-mem a000 DEADBEEF` after a reset, then `read-mem a000` still shows the BASIC ROM
(`94 E3 7B E3`). The firmware's own machine code monitor (Developer > Machine Code Monitor) does
show `DE AD BE EF` after `O` switches its view, so the bytes are there - only the monitor can
reach them. A program that banks the ROM out itself (`$01 = $35`, common in games) exposes the
RAM to its own code, not to DMA reads.

## Colour registers read back with the upper bits set

VIC colour registers are 4 bits wide and the unused bits read as `1`:

```sh
c64u machine write-mem d020 02
c64u machine read-mem  d020 --length 1     # D020: F2, not 02
```

Mask with `& 0x0F` before comparing. The same applies to colour RAM at `$D800-$DBE7`.

## Keyboard injection drives BASIC, not games

`sendkey` writes PETSCII into the keyboard buffer at `$0277` and updates the count at `$00C6`.
That is the path the KERNAL and BASIC read from, so it works for the BASIC prompt, `INPUT`,
`GET`, and typing commands.

Games and demos usually poll the keyboard matrix through CIA 1 (`$DC00`/`$DC01`) instead. Those
registers are driven by the hardware scan of the physical key lines: an injected value is
overwritten before any game reads it, so a "press SPACE to start" title screen cannot be driven
with `sendkey`.

Measured against a game that scans the matrix itself: three `sendkey " "` left the screen
byte-identical, while `$0277` held `$20` and `$00C6` held `1`. The byte arrives, and nothing
reads it.

`machine input` is the path that does reach such a program - it injects at the key matrix and
the joystick ports instead of the buffer. It needs the Ultimate 64 input hardware **and** a
firmware that has the endpoint - 1.2.0 on the C64 Ultimate, 3.15 on the other products - and its
two refusals mean different things: `404` is a firmware without the endpoint, `501` is a
product without Ultimate 64 input hardware, which a cartridge in a real C64 never has.

`c64u machine reset` releases everything held through `machine input`: a `press:joy2:fire`
before the reset reads as released afterwards (`$DC00` back to `$7F`). A press sent right after
the reset call returns does stick. The video window sends what is still held again after Mod+R.

The `sendkey` buffer is **10 bytes**. Longer strings are chunked with `--delay` between chunks, and the
machine has to consume each chunk. If the running program does not drain the buffer, the rest
is lost.

## Function key codes are interleaved

`\f1` through `\f8` map to `$85 $89 $86 $8A $87 $8B $88 $8C` - not sequentially - because F2,
F4, F6 and F8 are the shifted variants of F1, F3, F5 and F7. To inject a code the escapes do not
cover, write the buffer directly:

```sh
c64u machine write-mem 0277 86      # F3
c64u machine write-mem 00c6 01      # one character waiting
```

## Running a program proves the upload, not the result

`run-prg-upload` reports success once the machine has been told to start. Whether the program
did the right thing is a separate question, and `read-mem` is how you answer it. See
`workflows.md`.

A program that ends in an endless loop keeps the machine busy; `c64u machine reset` is the way
out.

## U64-only commands

These need an Ultimate 64, not a 1541 Ultimate II:

- `c64u streams` - video, audio, debug
- `c64u machine poweroff`, `poweron`
- `c64u machine debug-reg`, `debug-reg-set` (register `$D7FF`)
- `c64u machine input` - and firmware 1.2.0 (C64 Ultimate) or 3.15 (Ultimate 64) on top of that

An Ultimate II+L answers `404` with an empty body for every `streams` path, valid stream names
included, while an Ultimate 64 answers `404` with `Unrecognized stream name` for a made-up one
and `200` for a real one. An empty body means the route does not exist at all, which is the
quickest way to tell a missing feature from a rejected argument.

`c64u machine menu-screen` is not U64-only: it needs firmware 1.2.0 on the C64 Ultimate or 3.15
elsewhere, and it was verified on an Ultimate II+L and on a C64 Ultimate on 1.2.1.

## Video and audio are missing from some builds

`streams listen video` and `listen audio` open a window and use the host's graphics and sound
libraries, so they are compiled in only where that is possible: the macOS, Windows and Linux
binaries have them, builds for other systems answer that the stream is not available.
`listen debug` is plain output and works in every build.

On Linux the binary loads the graphics and sound libraries only when a window or audio stream
opens (X11 or XWayland, OpenGL, PulseAudio or ALSA); every other command runs without them. In
a VM without a GPU, Mesa warns `DRI3 error: Could not get DRI3 device` and renders in software;
the warning is harmless.

## A stream start fails with `Network Host Resolve Error`

Streams leave through the wired Ethernet port only. Before a unicast stream starts, the firmware
sends a small UDP packet to the destination so that its hardware address gets looked up, then
reads that address from the wired port's ARP table. On firmware 1.2.1, when Ethernet and Wi-Fi
are both connected to the same subnet, that packet leaves through Wi-Fi instead, so the wired
table never gets the entry and the start fails with `Network Host Resolve Error` (HTTP 404) - for
video, audio and debug alike. It only works for a destination that has itself talked to the
device's wired address recently, which makes it look sporadic.

Checked on 1.2.1 (build `31687dbd`): with both interfaces up, unicast starts to four LAN hosts
that had not contacted the device failed 3 of 3; with Wi-Fi off, starts to the router went from
5 of 5 failing to 5 of 5 working. A start to the multicast group `239.0.1.65` worked while
unicast failed: multicast needs no lookup. The upstream 1541ultimate firmware fixed it in commit `5d42e1db` ("Bind stream
primer sockets to the wired interface", 2026-10-17); a reboot does not help.

Ways out, all checked on 1.2.1:

- Turn Wi-Fi off when the cable is plugged in.
- Keep Wi-Fi (Wake-on-LAN needs it) but take it out of the subnet: `WiFi settings` with
  `Use DHCP` `Disabled`, a free static address and `Static Netmask` `255.255.255.255`, then
  `c64u config save-to-flash "WiFi settings"` and `c64u machine reboot` - the Wi-Fi DHCP setting
  only takes effect at boot. Afterwards fresh hosts streamed 3 of 3, REST still answered on the
  Wi-Fi address and the clock kept its time. A static address with the normal `/24` mask does
  not help: what matters is the shared subnet, not DHCP.
- `c64u streams start` to a multicast group, for receivers that join it.

`streams listen video`, `audio` and `debug` retry for 6 seconds, which helps only if the
destination happens to talk to the wired address meanwhile, then name the cause.

## Configuration changes are volatile by default

`c64u config set` takes effect at once but is lost on power-off. Follow it with
`c64u config save-to-flash` to persist.

`reset-to-default` changes only the settings in memory; flash keeps the saved values. Until
something saves, `load-from-flash` brings them back - checked on the device: `Drive Bus ID`
went 9 → 8 with `reset-to-default "Drive A Settings"` and back to 9 with
`load-from-flash "Drive A Settings"`. Unsaved changes made before the reset are gone either way,
so export first when they matter, and put the export back with `restore` (see
`workflows.md`, "Back up and restore the configuration"):

```sh
c64u config export config-backup.json
c64u config restore config-backup.json
```

Do not run `save-to-flash` on a device that booted into safe mode: it holds factory defaults in
memory then, and saving replaces the stored settings with them (firmware 1.2.1 API notes; not
reproduced here).

All three take a category name or pattern on firmware 1.2.1 and then touch only the matching
categories - `save-to-flash "C64 and Cartridge Settings"` persists one change without writing
other experiments to flash.

## Saved SoftIEC partitions load at power-on or from the menu

No REST endpoint sets SoftIEC partitions. `c64u drives softiec partitions set` writes
`/Flash/config/iec_partitions.ipr`, and firmware 1.2.1 loads that file by itself only when the
device is switched on: `c64u machine reboot` restarts the C64 and its cartridge, and the active
list stays as it was. Opening the file in the Ultimate menu loads it at once - Disk File Browser
> Flash > config > `iec_partitions.ipr` > Load; any `.ipr` file offers Load there (checked
2026-11-27, together with `drives softiec partitions`, which shows both lists). The file replaces the built-in list - partition 1 on `/Temp` is gone unless it is named -
and the lowest number becomes current, the one `LOAD"$",11` lists. The UCI command `$05 $20`
changes partitions at once, but only from code running on the C64 and only until power-off.

## Wake-on-LAN needs Wi-Fi and a saved setting

`c64u machine poweron` broadcasts a magic packet for the device's `wifi_mac`. Firmware 1.2.1
wakes on it only with `U64 Specific Settings` > `Wake On Wi-Fi` `Enabled` - the default is
`Disabled` - and only once that is saved to flash: set in memory alone it woke the device once
and read `Disabled` again after that power-on. Saved, it woke 4 of 4 within 2-4 s, also with
Wi-Fi on a static `/32` address (checked 2026-11-27). The wired interface does not wake the
device. A switched-off device cannot report its MAC, which is why `machine poweroff` caches it.

## Remote FTP servers load at power-on

`c64u fs servers add` writes `/Flash/config/ftp_servers` (`{"version":1,"servers":[{alias, host,
port, user, password, folder}]}`, the same file the menu writes). Firmware 1.2.1 reads it only when
switched on, and unlike a SoftIEC `.ipr` it has no Load action in the menu, so a change needs
`machine poweroff` and `machine poweron` - or adding the server in the menu, which works at once.
The password is URL-encoded the way the firmware's `url_encode` does it: letters, digits and
`_-.*` stay, the rest becomes lowercase `%xx` (`pa ss&w=rd` logged in as `pa%20ss%26w%3drd`;
checked 2026-11-27 against an FTP server on another computer).

## JSON key casing is not uniform

Most commands emit snake_case (`core_version`, `bus_id`). `c64u fs ls --json` emits PascalCase
(`Name`, `Size`, `IsDir`, `Type`). Scripts touching both need to handle both.

## The video window is a keyboard, joystick and mouse

While `c64u streams listen video` has focus, keys, gamepads and the captured mouse reach the C64
through `machine:input`, so games and the Ultimate menu see them, not only BASIC. That needs
firmware 1.2.0 (C64 Ultimate) or 3.15; on older firmware the window falls back to the keyboard
buffer, with the same BASIC-only limitation as `sendkey`.

- In character mode (the default) every typed character reaches the C64 as a short tap, so a
  held key follows the host's key repeat and a game reading the keyboard matrix sees it stutter.
  Position mode (Mod+K) holds C64 keys as long as the host keys are held; use it for games.
- The mouse is on control port 1 only; while it is captured a joystick on port 1 is ignored.
  Port 1 shares `$DC01` with the keyboard rows, so while the KERNAL scans the keyboard a button
  reads as a key: the left button (fire, bit 4) types a space at the BASIC prompt, as on a real
  C64 with a 1351.
- `C=+O`, the monitor's hotkey, does not arrive through `machine:input` - not from a running
  program and not inside the menu. Open the monitor through the menu instead: Mod+U in the window
  (or `c64u machine menu-button`), then F1 › Developer › Machine Code Monitor.
- The device takes about 25 ms per input call on a LAN. The window sends what changed since the
  last call, so input is delayed by about one call but never queues up. The price: pressing the
  same key twice within one call arrives as a single press.
- On firmware without `machine:input` the window notices before the first keystroke and types
  through the keyboard buffer from the start.
