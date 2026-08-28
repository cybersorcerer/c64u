// Ultimate wedge - autostart cartridge adding "@" commands to stock BASIC V2.
//
// Why a cartridge: a wedge loaded as a PRG is gone after every reset, and the
// Ultimate has no boot-PRG setting. A cartridge in /flash/carts is there from
// power-on.
//
// Why Magic Desk (CRT hardware type 19): a plain 8K cartridge occupies $8000
// permanently and costs 8 KB of BASIC memory - 30719 bytes free instead of
// 38911. Magic Desk has a disable bit in its bank register at $DE00, so this
// ROM copies itself into RAM, switches itself off, and BASIC comes up whole.
//
// Why $C000: the KERNAL and BASIC never touch that 4 KB, so the resident part
// survives RUN/STOP+RESTORE and NEW.
//
// Commands:
//   @$        directory of the Ultimate filesystem, straight to the screen
//   @CD:NAME  change directory
//   /NAME     load
//   ^NAME     load and run  (^ is the up arrow, PETSCII $5E)
//
// Everything goes through the Ultimate Command Interface, so it all works on
// the directory shown by @$ - no SoftIEC, no disk image, no second notion of a
// current directory. @$ prints with CHROUT and, unlike LOAD"$", leaves the
// BASIC program in memory untouched.

// ---------------------------------------------------------------- constants

.const MAGICDESK_CTRL = $de00           // bit 7 disables the cartridge
.const RAM_CODE       = $c000
.const STRPTR         = $fb             // zero page scratch, free for user code
.const DESTPTR        = $fd             // load destination, also zero page
.const BASIC_START    = $0801
.const STATUS_MAX     = 38              // one screen line, minus room for CR

// Pages copied from ROM to $C000 at boot. The assert below fails if the
// resident part outgrows this.
.const RESIDENT_PAGES = 6

// Ultimate Command Interface
.const UCI_CONTROL    = $df1c           // write
.const UCI_STATUS     = $df1c           // read
.const UCI_CMD_DATA   = $df1d           // write
.const UCI_IDENT      = $df1d           // read, $C9 when present
.const UCI_RESP_DATA  = $df1e
.const UCI_STAT_DATA  = $df1f

.const PUSH_CMD       = %00000001
.const DATA_ACC       = %00000010
.const ABORT          = %00000100
.const ST_DATA_AV     = %10000000
.const ST_STAT_AV     = %01000000
.const ST_STATE_MASK  = %00110000
.const ST_STATE_BUSY  = %00010000
.const ST_STATE_MORE  = %00110000

.const TARGET_DOS     = $01
.const DOS_OPEN_FILE  = $02
.const DOS_CLOSE_FILE = $03
.const DOS_READ_DATA  = $04
.const DOS_CHANGE_DIR = $11
.const DOS_OPEN_DIR   = $13
.const DOS_READ_DIR   = $14

.const FA_READ        = $01             // open mode flag

.const ATTR_DIR       = $10             // FAT attribute bit

// Character codes as literal values. A 'x' literal is translated with whatever
// .encoding is in force, and the default screencode_mixed turns '@' into $00 -
// which compares against the wrong byte and truncates strings.
.const CH_AT      = $40
.const CH_DOLLAR  = $24
.const CH_C       = $43
// BASIC tokenises operators before a line is executed, so by the time the
// dispatcher sees the line, '/' has become $AD and the up arrow $AE. Comparing
// against $2F and $5E never matches. '@' is not an operator and survives as is.
.const TOK_RUN     = $8a          // the RUN keyword, as BASIC stores it
.const TOK_SLASH   = $ad
.const TOK_ARROWUP = $ae
.const CH_ZERO    = $30
.const CH_LC_A    = $61
.const CH_LC_Z    = $7a

// KERNAL and BASIC
.const IOINIT   = $fda3
.const RAMTAS   = $fd50
.const RESTOR   = $fd15
.const CINT     = $ff5b
.const CHROUT   = $ffd2
.const STOPKEY  = $ffe1                 // Z set when RUN/STOP is held
.const CHRGET   = $0073
.const TXTPTR   = $7a                   // BASIC text pointer
.const TXTTAB   = $2b                   // start of BASIC program
.const VARTAB   = $2d                   // end of program / start of variables
.const BASIC_RELINK = $a533             // rebuild BASIC line links
.const IGONE    = $0308                 // BASIC statement dispatch vector
.const IMAIN    = $0302                 // BASIC main loop vector
.const IRQVEC   = $0314                 // KERNAL IRQ vector
.const KERNAL_IRQ = $ea31               // default IRQ handler
.const BASIC_COLDSTART = $a000          // vector to the ROM's BASIC cold start
.const BASIC_IGONE     = $a7e4          // default contents of the IGONE vector
.const BASIC_DISPATCH  = $a7e7          // token dispatch, character already read
.const BASIC_LOOP      = $a7ae          // interpreter loop

// ---------------------------------------------------------------- cartridge

* = $8000 "Cartridge header"

        .word coldStart
        .word coldStart
        .byte $c3, $c2, $cd, $38, $30   // "CBM80"

coldStart:
        sei
        cld
        ldx #$ff
        txs

        // Copy the resident part to RAM while the ROM is still visible.
        // Unrolled per page so the loop stays a simple 8-bit counter.
        ldx #$00
!loop:
        .for (var page = 0; page < RESIDENT_PAGES; page++) {
                lda residentSource + page * $100, x
                sta RAM_CODE + page * $100, x
        }
        inx
        bne !loop-

        jmp RAM_CODE

// ---------------------------------------------------------------- resident

residentSource:
.pseudopc RAM_CODE {

resident:
        // Unmap the cartridge. Must not run from the cartridge itself.
        lda #$80
        sta MAGICDESK_CTRL

        jsr IOINIT
        jsr RAMTAS
        jsr RESTOR
        jsr CINT

        // BASIC's cold start rewrites $0300-$030B, so installing the hook here
        // would achieve nothing. Reproducing the cold start inline is not an
        // option either: it differs between KERNAL revisions - a JiffyDOS
        // machine calls $E4B7 where a stock one calls $E453.
        //
        // So let the ROM start BASIC untouched and take the hook in through the
        // IRQ vector, which BASIC start-up does not reset. One frame later the
        // installer runs, hooks IGONE and puts the IRQ back.
        lda #<irqInstaller
        sta IRQVEC
        lda #>irqInstaller
        sta IRQVEC + 1

        cli
        jmp (BASIC_COLDSTART)

// Runs on every interrupt until BASIC has set up its own vectors, then installs
// the hook and steps aside.
//
// Waiting matters: the first interrupt arrives before BASIC's cold start has
// reached $E453, and that routine rewrites $0300-$030B. A hook installed any
// earlier is silently wiped. $0308 holding the default dispatcher is the signal
// that $E453 has run - RAMTAS left it at zero before that.
irqInstaller:
        lda IGONE
        cmp #<BASIC_IGONE
        bne notReady
        lda IGONE + 1
        cmp #>BASIC_IGONE
        beq install
notReady:
        jmp KERNAL_IRQ

install:
        lda #<KERNAL_IRQ
        sta IRQVEC
        lda #>KERNAL_IRQ
        sta IRQVEC + 1

        lda #<wedgeHandler
        sta IGONE
        lda #>wedgeHandler
        sta IGONE + 1

        // The banner must not be printed from here: BASIC is still writing its
        // own start-up message and CHROUT would interleave with it. Hook the
        // main loop instead, which BASIC enters once it is done and idle.
        lda IMAIN
        sta origMain
        lda IMAIN + 1
        sta origMain + 1
        lda #<bannerOnce
        sta IMAIN
        lda #>bannerOnce
        sta IMAIN + 1

        jmp KERNAL_IRQ

// First pass through the BASIC main loop: print the banner, then get out of the
// way for good.
bannerOnce:
        lda origMain
        sta IMAIN
        lda origMain + 1
        sta IMAIN + 1

        ldx #<bannerText
        ldy #>bannerText
        jsr printString

        jmp (IMAIN)

// Called by BASIC instead of the statement dispatcher. The default does CHRGET
// and falls into the dispatcher, so anything not ours must end up there too.
// CHRGET leaves flags that BASIC's statement executor depends on: zero for the
// end of a statement, carry clear for a digit. A cmp overwrites them, so they
// are saved across the comparisons - without this, PRINT is executed as PRINT#
// and RUN reports ?UNDEF'D STATEMENT.
wedgeHandler:
        jsr CHRGET
        php
        cmp #CH_AT
        beq !at+
        cmp #TOK_SLASH
        beq !load+
        cmp #TOK_ARROWUP
        beq !loadRun+
        plp
        jmp BASIC_DISPATCH
!at:
        plp
        jmp atCommand
!load:
        plp
        jmp doLoad
!loadRun:
        plp
        jmp doLoadRun

atCommand:
        jsr CHRGET                      // the character after '@'
        cmp #CH_DOLLAR
        beq doDirectory
        cmp #CH_C
        beq doChangeDir

        ldx #<errText
        ldy #>errText
        jsr printString
        jmp endOfCommand

doDirectory:
        jsr directory
        jmp endOfCommand

doChangeDir:
        jsr changeDir
        jmp endOfCommand

// "/NAME" loads, "^NAME" loads and starts.
doLoad:
        lda #$00
        sta runAfterLoad
        jmp loadCommand

doLoadRun:
        lda #$ff
        sta runAfterLoad
        jmp loadCommand

// BASIC carries on interpreting whatever is left of the line, so the remainder
// has to be consumed - otherwise "@$" is followed by a ?SYNTAX ERROR for the
// characters the wedge already dealt with.
endOfCommand:
        jsr CHRGET
        bne endOfCommand
        jmp BASIC_LOOP

// ------------------------------------------------------------ UCI plumbing

// Returns with carry set when no Ultimate Command Interface is mapped in.
uciPresent:
        lda UCI_IDENT
        cmp #$c9
        beq !ok+
        ldx #<noUciText
        ldy #>noUciText
        jsr printString
        sec
        rts
!ok:
        clc
        rts

// Waits for the protocol to leave "command busy".
uciWait:
        lda UCI_STATUS
        and #ST_STATE_MASK
        cmp #ST_STATE_BUSY
        beq uciWait
        rts

// Reads and discards the status channel.
uciDrainStatus:
        lda UCI_STATUS
        and #ST_STAT_AV
        beq !done+
        lda UCI_STAT_DATA
        jmp uciDrainStatus
!done:
        rts

// Copies the status channel into statusBuf, null terminated. The Ultimate
// answers in Commodore form, "00,OK" or "01,FILE NOT FOUND", so the first two
// characters decide success.
uciReadStatus:
        ldx #$00
!loop:
        lda UCI_STATUS
        and #ST_STAT_AV
        beq !done+
        lda UCI_STAT_DATA
        cpx #STATUS_MAX
        bcs !loop-                      // keep draining, stop storing
        sta statusBuf,x
        inx
        jmp !loop-
!done:
        lda #$00
        sta statusBuf,x
        rts

// Carry clear when the status reads "00".
uciStatusOK:
        lda statusBuf
        cmp #CH_ZERO
        bne !bad+
        lda statusBuf + 1
        cmp #CH_ZERO
        bne !bad+
        clc
        rts
!bad:
        sec
        rts

// Prints the status line only when it reports a failure.
uciReportError:
        jsr uciStatusOK
        bcc !ok+
        ldx #<statusBuf
        ldy #>statusBuf
        jsr printString
        lda #13
        jsr CHROUT
        sec
        rts
!ok:
        clc
        rts

uciAccept:
        lda #DATA_ACC
        sta UCI_CONTROL
        rts

// Asks the Ultimate to drop the current transfer and return to idle.
uciAbort:
        lda #ABORT
        sta UCI_CONTROL
        rts

// ------------------------------------------------------------- @$ directory

directory:
        jsr uciPresent
        bcc !go+
        rts
!go:
        // Open the current directory.
        lda #TARGET_DOS
        sta UCI_CMD_DATA
        lda #DOS_OPEN_DIR
        sta UCI_CMD_DATA
        lda #PUSH_CMD
        sta UCI_CONTROL
        jsr uciWait
        jsr uciReadStatus
        jsr uciAccept
        jsr uciReportError              // e.g. 86,CAN'T READ DIRECTORY
        bcc !ok+
        rts
!ok:

        // Read it. Every entry arrives as its own packet.
        lda #TARGET_DOS
        sta UCI_CMD_DATA
        lda #DOS_READ_DIR
        sta UCI_CMD_DATA
        lda #PUSH_CMD
        sta UCI_CONTROL

entryLoop:
        jsr uciWait
        jsr printEntry

        // Remember whether more packets follow before accepting this one,
        // because accepting clears the state.
        lda UCI_STATUS
        and #ST_STATE_MASK
        cmp #ST_STATE_MORE
        php
        jsr uciAccept
        plp
        bne !done+

        // A directory of a few hundred files would otherwise scroll past with
        // no way to stop it. Leaving the transfer half read would strand the
        // interface, so tell the Ultimate to drop it.
        jsr STOPKEY
        bne entryLoop
        jsr uciAbort
        rts
!done:
        jsr uciDrainStatus
        rts

// First byte of a packet is the FAT attribute, the rest is the name.
printEntry:
        lda UCI_STATUS
        and #ST_DATA_AV
        bne !have+
        rts
!have:
        lda UCI_RESP_DATA
        sta entryAttr

!name:
        lda UCI_STATUS
        and #ST_DATA_AV
        beq !endName+
        lda UCI_RESP_DATA
        jsr toPetscii
        jsr CHROUT
        jmp !name-
!endName:
        lda entryAttr
        and #ATTR_DIR
        beq !newline+
        ldx #<dirText
        ldy #>dirText
        jsr printString
!newline:
        lda #13
        jsr CHROUT
        rts

// ---------------------------------------------------------- /NAME and ^NAME

// Loads the file named on the rest of the BASIC line. The first two bytes of a
// PRG are its load address, exactly as with LOAD",8,1".
loadCommand:
        jsr uciPresent
        bcc !go+
        jmp endOfCommand
!go:
        lda #TARGET_DOS
        sta UCI_CMD_DATA
        lda #DOS_OPEN_FILE
        sta UCI_CMD_DATA
        lda #FA_READ
        sta UCI_CMD_DATA

        // The command length defines the filename length, so no terminator.
        ldx #$00
!copy:
        jsr CHRGET
        beq !send+
        sta UCI_CMD_DATA
        inx
        jmp !copy-
!send:
        cpx #$00
        bne !named+
        ldx #<noNameText
        ldy #>noNameText
        jsr printString
        jmp endOfCommand
!named:
        lda #PUSH_CMD
        sta UCI_CONTROL
        jsr uciWait
        jsr uciReadStatus
        jsr uciAccept
        jsr uciReportError
        bcc !opened+
        jmp endOfCommand
!opened:
        jsr readFileIntoMemory
        jsr closeFile
        jsr finaliseBasicLoad

        lda runAfterLoad
        beq !justLoaded+
        jmp startProgram
!justLoaded:
        ldx #<readyText
        ldy #>readyText
        jsr printString
        jmp endOfCommand

// Streams the file into memory. Asking for $FFFF bytes transfers whatever the
// file holds; the packets simply stop.
readFileIntoMemory:
        lda #$00
        sta headerCount

        lda #TARGET_DOS
        sta UCI_CMD_DATA
        lda #DOS_READ_DATA
        sta UCI_CMD_DATA
        lda #$ff
        sta UCI_CMD_DATA                // length low
        lda #$ff
        sta UCI_CMD_DATA                // length high
        lda #PUSH_CMD
        sta UCI_CONTROL

packetLoop:
        jsr uciWait
!bytes:
        lda UCI_STATUS
        and #ST_DATA_AV
        beq !packetDone+
        lda UCI_RESP_DATA
        jsr storeByte
        jmp !bytes-
!packetDone:
        lda UCI_STATUS
        and #ST_STATE_MASK
        cmp #ST_STATE_MORE
        php
        jsr uciAccept
        plp
        beq packetLoop

        jsr uciDrainStatus
        rts

// The first two bytes are the load address; everything after goes to memory.
storeByte:
        ldx headerCount
        cpx #$02
        bcs !data+

        sta loadAddr,x
        inc headerCount
        cpx #$01
        bne !out+

        // Both address bytes are in, so the destination pointer can be set up.
        lda loadAddr
        sta DESTPTR
        lda loadAddr + 1
        sta DESTPTR + 1
!out:
        rts
!data:
        ldy #$00
        sta (DESTPTR),y
        inc DESTPTR
        bne !out-
        inc DESTPTR + 1
        rts

closeFile:
        lda #TARGET_DOS
        sta UCI_CMD_DATA
        lda #DOS_CLOSE_FILE
        sta UCI_CMD_DATA
        lda #PUSH_CMD
        sta UCI_CONTROL
        jsr uciWait
        jsr uciDrainStatus
        jsr uciAccept
        rts

// Carry set when the file just loaded is a BASIC program.
loadedBasic:
        lda loadAddr
        cmp #<BASIC_START
        bne !no+
        lda loadAddr + 1
        cmp #>BASIC_START
        bne !no+
        sec
        rts
!no:
        clc
        rts

// A BASIC program is unusable until the pointers behind it are moved and the
// line links are rebuilt - LIST and RUN both go wrong otherwise. This has to
// happen for a plain load as well, not only when the program is started.
//
// The pointers are written directly rather than by calling BASIC's CLR: that
// routine ends in PLA/TAY/PLA and juggles the stack, so it cannot be used as an
// ordinary subroutine. Calling it crashes the machine on the way back.
finaliseBasicLoad:
        lda headerCount
        cmp #$02
        bne !out+                       // nothing arrived, leave BASIC alone
        jsr loadedBasic
        bcc !out+

        ldx #$00
!loop:
        lda DESTPTR
        sta VARTAB,x
        lda DESTPTR + 1
        sta VARTAB + 1,x
        inx
        inx
        cpx #$06                        // VARTAB, ARYTAB and STREND
        bne !loop-

        jsr BASIC_RELINK
!out:
        rts

// BASIC programs are handed to BASIC's own RUN, which performs a proper CLR.
// Machine code is entered at its load address.
startProgram:
        jsr loadedBasic
        bcc !machineCode+

        // Rather than entering the RUN routine directly - which reads the flags
        // it was called with to decide whether a line number follows, and is
        // easy to get subtly wrong - point the text pointer at a one-token
        // "RUN" line and let BASIC execute it exactly as if it had been typed.
        lda #<(runLine - 1)
        sta TXTPTR
        lda #>(runLine - 1)
        sta TXTPTR + 1
        jsr CHRGET                      // fetches the RUN token
        jmp BASIC_DISPATCH

!machineCode:
        jmp (loadAddr)

// ------------------------------------------------------------- @CD:NAME

changeDir:
        jsr uciPresent
        bcc !go+
        rts
!go:
        jsr CHRGET                      // 'D'
        jsr CHRGET                      // ':'

        lda #TARGET_DOS
        sta UCI_CMD_DATA
        lda #DOS_CHANGE_DIR
        sta UCI_CMD_DATA

!copy:
        jsr CHRGET
        beq !send+                      // end of the BASIC line
        sta UCI_CMD_DATA
        jmp !copy-
!send:
        lda #PUSH_CMD
        sta UCI_CONTROL
        jsr uciWait

        // Echo whatever the status channel says, e.g. "00,OK".
        jsr printStatus
        jsr uciAccept
        rts

printStatus:
        lda UCI_STATUS
        and #ST_STAT_AV
        beq !done+
        lda UCI_STAT_DATA
        jsr toPetscii
        jsr CHROUT
        jmp printStatus
!done:
        lda #13
        jmp CHROUT

// ------------------------------------------------------------------ helpers

// Filenames arrive as ASCII. Uppercase passes through; lowercase would show as
// graphics characters in the default character set, so fold it.
toPetscii:
        cmp #CH_LC_A
        bcc !plain+
        cmp #CH_LC_Z + 1
        bcs !plain+
        sec
        sbc #$20
!plain:
        rts

// X/Y point at a null terminated string.
//
// The pointer has to live in zero page: (indirect),y has no absolute form, and
// pointing it at a $C0xx location assembles to something that reads elsewhere
// entirely.
printString:
        stx STRPTR
        sty STRPTR + 1
        ldy #$00
!loop:
        lda (STRPTR),y
        beq !done+
        jsr CHROUT
        iny
        bne !loop-
!done:
        rts

origMain:     .word $0000
entryAttr:    .byte $00
runAfterLoad: .byte $00
headerCount:  .byte $00
loadAddr:     .word $0000
runLine:      .byte TOK_RUN, $00      // a tokenised "RUN", executed after load
statusBuf:    .fill STATUS_MAX + 1, 0

// These strings go to CHROUT, which takes PETSCII. Kick Assembler's default
// encoding is screencode_mixed, where uppercase letters happen to survive the
// round trip but '@' becomes $00 - which silently truncates a string.
.encoding "petscii_upper"

bannerText: .byte 13
            .text "C64U ULTIMATE WEDGE BY CYBERSORCERER"
            .byte 13
            .text "@$ DIR  @CD:NAME  /LOAD  "
            .byte $5e                   // up arrow, no ASCII equivalent
            .text "LOAD+RUN"
            .byte 13, 0
dirText:    .text "  <DIR>"
            .byte 0
errText:    .text "?UNKNOWN WEDGE COMMAND"
            .byte 13, 0
noNameText: .text "?MISSING FILENAME"
            .byte 13, 0
readyText:  .text "LOADED"
            .byte 13, 0
noUciText:  .text "?COMMAND INTERFACE DISABLED"
            .byte 13, 0
residentEnd:
}

.print "resident size: " + (residentEnd - resident) + " bytes"
.errorif (residentEnd - resident) > RESIDENT_PAGES * $100, "resident part outgrew RESIDENT_PAGES - raise it"

// Pad to a full 8 KB image.
* = $9fff
        .byte $00
