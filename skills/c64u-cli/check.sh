#!/bin/sh
# Checks that every command the reference documents still exists in the CLI.
#
# A reference listing a removed or renamed command is worse than no reference:
# an agent follows it, the call fails, and the failure looks like a device
# problem.
#
# The check walks the CLI's own command tree and compares sets. Asking
# "<command> --help" is not usable as a probe: cobra answers successfully for an
# unknown subcommand by printing the parent's help, so a made-up command would
# pass. Nothing is executed here, only help output is parsed.

set -eu

C64U=${C64U:-../../tools/c64u/build/c64u}
DOC=references/commands.md

if [ ! -x "$C64U" ]; then
    echo "SKIP: $C64U not built - run 'make -C tools/c64u build'"
    exit 0
fi

subcommands() {
    $C64U "$@" --help 2>/dev/null |
        sed -n '/^Available Commands:/,/^$/p' |
        sed -n 's/^  \([a-z][a-z0-9-]*\) .*/\1/p'
}

# The tree is at most three levels deep (c64u streams listen video).
actual=$(
    for a in $(subcommands); do
        echo "$a"
        for b in $(subcommands "$a"); do
            echo "$a $b"
            for c in $(subcommands "$a" "$b"); do
                echo "$a $b $c"
            done
        done
    done | sort -u
)

# Command lines in the doc start with "c64u "; keep the leading lowercase words,
# which form the command path, and drop arguments and flags.
documented=$(
    sed -n 's/^c64u \([a-z][a-z0-9-]*\( [a-z][a-z0-9-]*\)*\).*/\1/p' "$DOC" |
    sed 's/ *$//' | sort -u
)

has_children() {
    echo "$actual" | grep -q "^$1 "
}

# Walk the words of a documented line. A word is part of the command path while
# it names a real subcommand. The first word that does not is either an argument
# - fine, if the path so far is a leaf command - or a command that no longer
# exists, which is the whole point of this check.
fail=0
count=0
for entry in $(echo "$documented" | tr ' ' '\037'); do
    line=$(echo "$entry" | tr '\037' ' ')
    count=$((count + 1))
    path=""
    for word in $line; do
        candidate=${path:+$path }$word
        if echo "$actual" | grep -qx "$candidate"; then
            path=$candidate
            continue
        fi
        if [ -z "$path" ] || has_children "$path"; then
            printf '  FAIL  documented but not in the CLI: c64u %s\n' "$candidate"
            fail=$((fail + 1))
        fi
        break                           # the rest of the line is arguments
    done
done

printf '%d documented commands checked, %d missing\n' "$count" "$fail"
[ "$fail" -eq 0 ]
