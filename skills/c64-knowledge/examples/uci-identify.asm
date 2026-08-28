// Talks to the Ultimate Command Interface from the C64 side and prints the
// reply of the DOS target's IDENTIFY command on the screen.
//
// This is the smallest complete round trip through the UCI protocol:
//   write command bytes -> PUSH_CMD -> wait out "command busy"
//   -> drain the data queue -> drain the status queue -> DATA_ACC
//
// Needs "Command Interface" enabled in the C64 and Cartridge settings:
//   c64u config set "C64 and Cartridge Settings" "Command Interface" Enabled
//
// Build: java -jar KickAss.jar uci-identify.asm -o uci-identify.prg
// Run:   c64u runners run-prg-upload uci-identify.prg

.const UCI_CONTROL   = $df1c            // write
.const UCI_STATUS    = $df1c            // read
.const UCI_CMD_DATA  = $df1d            // write
.const UCI_IDENT     = $df1d            // read, $C9 when the interface is present
.const UCI_RESP_DATA = $df1e            // read
.const UCI_STAT_DATA = $df1f            // read

.const PUSH_CMD      = %00000001        // control: hand the command over
.const DATA_ACC      = %00000010        // control: all data accepted

.const ST_DATA_AV    = %10000000        // status: response byte waiting
.const ST_STAT_AV    = %01000000        // status: status byte waiting
.const ST_STATE_MASK = %00110000        // status: 00 idle, 01 busy, 10 last, 11 more
.const ST_STATE_BUSY = %00010000

.const TARGET_DOS    = $01
.const DOS_IDENTIFY  = $01

.const SCREEN        = $0400

BasicUpstart2(start)

start:
        jsr clearScreen

        // The identification register reads $C9 when the interface is mapped in.
        // Anything else means it is switched off in the configuration.
        lda UCI_IDENT
        cmp #$c9
        beq present
        lda #RED
        sta $d020
        rts

present:
        // Write the command, then push it.
        lda #TARGET_DOS
        sta UCI_CMD_DATA
        lda #DOS_IDENTIFY
        sta UCI_CMD_DATA
        lda #PUSH_CMD
        sta UCI_CONTROL

        // Wait until the protocol leaves "command busy".
!wait:
        lda UCI_STATUS
        and #ST_STATE_MASK
        cmp #ST_STATE_BUSY
        beq !wait-

        // Response data onto the first screen line.
        ldx #$00
!loop:
        lda UCI_STATUS
        and #ST_DATA_AV
        beq statusChannel
        lda UCI_RESP_DATA
        jsr toScreenCode
        sta SCREEN,x
        inx
        bne !loop-

statusChannel:
        // Status channel onto the second screen line, e.g. "00,OK".
        ldx #$00
!loop:
        lda UCI_STATUS
        and #ST_STAT_AV
        beq accept
        lda UCI_STAT_DATA
        jsr toScreenCode
        sta SCREEN + 40,x
        inx
        bne !loop-

accept:
        // Tell the Ultimate the data was taken; this returns the state machine
        // to idle. Skipping this leaves the interface stuck for the next caller.
        lda #DATA_ACC
        sta UCI_CONTROL

        lda #GREEN
        sta $d020
        rts

// The reply is ASCII; screen codes differ for letters only.
toScreenCode:
        cmp #$40
        bcc !plain+
        cmp #$60
        bcs !plain+
        sec
        sbc #$40
!plain:
        rts

clearScreen:
        lda #$20
        ldx #$00
!loop:
        sta SCREEN,x
        sta SCREEN + $100,x
        sta SCREEN + $200,x
        sta SCREEN + $2e8,x
        inx
        bne !loop-
        lda #WHITE
        ldx #$00
!loop:
        sta $d800,x
        sta $d800 + 40,x
        inx
        cpx #80
        bne !loop-
        rts
