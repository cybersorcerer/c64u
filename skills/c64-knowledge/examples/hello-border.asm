// Smallest useful C64 program: a BASIC upstart line plus a flashing border.
//
// Build: java -jar KickAss.jar hello-border.asm -o hello-border.prg
// Run:   c64u runners run-prg-upload hello-border.prg
//
// BasicUpstart2 emits "10 SYS <address>" at $0801 and continues assembling
// right after it, so the label below lands at the address the SYS calls.

BasicUpstart2(start)

start:
        inc $d020               // border colour, wraps at 16
        jmp start               // RUN/STOP + RESTORE gets you out
