#!/bin/sh
# Runs the examples on a real C64 Ultimate and checks what they changed.
#
# Assembling proves an example is still valid 6502; it does not prove the
# addresses and bit values in the reference files are right. This does - each
# check reads back a byte that only the program under test could have produced.
#
# Skipped, not failed, when no device answers: the examples must stay buildable
# on a machine that has none.

set -eu

C64U=${C64U:-../../tools/c64u/build/c64u}
BUILD=${BUILD:-build}

pass=0
fail=0

if [ ! -x "$C64U" ]; then
    echo "SKIP: $C64U not built - run 'make -C tools/c64u build'"
    exit 0
fi

if ! "$C64U" info >/dev/null 2>&1; then
    echo "SKIP: no C64 Ultimate reachable"
    exit 0
fi

# read <address> <length> -> hex bytes, uppercase, no separators
read_mem() {
    "$C64U" machine read-mem "$1" --length "$2" --raw 2>/dev/null |
        od -An -tx1 -v | tr -d ' \n' | tr 'a-f' 'A-F'
}

check() {
    label=$1 addr=$2 len=$3 want=$4
    got=$(read_mem "$addr" "$len")
    if [ "$got" = "$want" ]; then
        printf '  ok    %s ($%s = %s)\n' "$label" "$addr" "$got"
        pass=$((pass + 1))
    else
        printf '  FAIL  %s ($%s: got %s, want %s)\n' "$label" "$addr" "$got" "$want"
        fail=$((fail + 1))
    fi
}

run() {
    "$C64U" runners run-prg-upload "$BUILD/$1" >/dev/null 2>&1
    sleep 3
}

echo "Running examples on $("$C64U" info --json | sed -n 's/.*"hostname" *: *"\([^"]*\)".*/\1/p')"

# The sprite pointer is data-address/64 and nothing else writes $07F8.
run sprite-setup.prg
check "sprite-setup pointer" 07f8 1 "30"
check "sprite-setup enable"  d015 1 "01"

# Bitmap mode: BMM set in $D011, and a pixel at a position computed in advance.
run hires-bitmap.prg
check "hires-bitmap D011" d011 1 "3B"
check "hires-bitmap pixel" 2f04 1 "80"

# The DOS target answers its own identity; the wedge relies on the same path.
run uci-identify.prg
check "uci-identify reply" 0400 4 "150C1409"

# SID registers are write-only, so the note cannot be read back off the chip.
# What can be checked is that the example assembled the frequency the official
# table gives for the standard this machine actually reports.
run sid-note.prg
standard=$(read_mem 02a6 1)
image=$(od -An -tx1 -v "$BUILD/sid-note.prg" | tr -d ' \n')
case "$standard" in
    01) want=a244a01d; label="sid-note A4 constant (PAL)" ;;
    00) want=a231a01c; label="sid-note A4 constant (NTSC)" ;;
    *)  printf '  FAIL  video standard byte is %s\n' "$standard"
        fail=$((fail + 1)); want=""; label="" ;;
esac
if [ -n "$want" ]; then
    case "$image" in
        *"$want"*)
            printf '  ok    %s\n' "$label"
            pass=$((pass + 1)) ;;
        *)
            printf '  FAIL  %s: %s not in the assembled image\n' "$label" "$want"
            fail=$((fail + 1)) ;;
    esac
fi

"$C64U" machine reset >/dev/null 2>&1

echo "$pass passed, $fail failed"
[ "$fail" -eq 0 ]
