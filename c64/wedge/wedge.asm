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
// Commands (^ is the up arrow, PETSCII $5E; & replaces @ under JiffyDOS):
//   @          current path
//   @$         directory, straight to the screen
//   @CD:NAME   change directory        @MD:NAME  create directory
//   @RM:NAME   delete file             @SV:NAME  save the BASIC program
//   @T:NAME    show a text file        @DR       list the drives
//   @MT9:NAME  mount a disk image      @SW9      swap to the next disk
//   /NAME      load                    ^NAME     load and run
//
// The digit in @MT and @SW is the drive bus id and may be left out; drive A is
// not always 8.
//
// The prefix depends on the machine. JiffyDOS claims '@', '/' and the up arrow
// and intercepts them before BASIC's dispatcher, so on such a machine those
// forms never reach this code. The cartridge detects JiffyDOS at boot and, when
// it is present, answers only to '&' - which JiffyDOS leaves alone. '&' always
// works; the classic prefixes work on a stock KERNAL.
//
// Everything goes through the Ultimate Command Interface, so it all works on
// the directory the listing just showed - no SoftIEC, no disk image, no second
// notion of a current directory. Listing prints with CHROUT and, unlike
// LOAD"$", leaves the BASIC program in memory untouched.

// ---------------------------------------------------------------- constants

.const MAGICDESK_CTRL = $de00           // bit 7 disables the cartridge
.const RAM_CODE       = $c000
.const STRPTR         = $fb             // zero page scratch, free for user code
.const DESTPTR        = $fd             // load destination, also zero page
.const SRCPTR         = $a3             // save source, free during our commands
.const KWPTR          = $a5             // keyword table walk
.const BASIC_START    = $0801
.const STATUS_MAX     = 38              // one screen line, minus room for CR

// Pages copied from ROM to $C000 at boot. The assert below fails if the
// resident part outgrows this.
.const RESIDENT_PAGES = 13

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
.const TARGET_CONTROL = $04
.const CTRL_GET_DRVINFO = $29
.const DRVINFO_MAX    = 8               // sanity limit on the reported count
.const DRVBUF_SIZE    = 1 + 3 * DRVINFO_MAX
.const DRIVE_TYPE_ENTRY = 8             // type byte plus seven name characters
.const DOS_OPEN_FILE  = $02
.const DOS_CLOSE_FILE = $03
.const DOS_READ_DATA  = $04
.const DOS_DELETE_FILE = $09
.const DOS_CHANGE_DIR = $11
.const DOS_GET_PATH   = $12
.const DOS_OPEN_DIR   = $13
.const DOS_READ_DIR   = $14
.const DOS_CREATE_DIR = $16
.const DOS_MOUNT_DISK = $23
.const DOS_SWAP_DISK  = $25
.const DOS_WRITE_DATA = $05

.const FA_READ        = $01             // open mode flags
.const FA_WRITE       = $02
.const FA_CREATE_NEW  = $04
.const FA_CREATE_ALWAYS = $08

.const DEFAULT_DRIVE_ID = 8             // the Ultimate falls back to the last
                                        // drive mounted on when 8 is absent
.const SAVE_CHUNK     = 128             // bytes per WRITE_DATA packet

.const ATTR_DIR       = $10             // FAT attribute bit

// Character codes as literal values. A 'x' literal is translated with whatever
// .encoding is in force, and the default screencode_mixed turns '@' into $00 -
// which compares against the wrong byte and truncates strings.
.const CH_AT      = $40
.const CH_AMP     = $26
.const CH_DOLLAR  = $24
.const CH_C       = $43
.const CH_D       = $44
.const CH_M       = $4d
.const CH_R       = $52
.const CH_S       = $53
.const CH_T       = $54
.const CH_V       = $56
.const CH_W       = $57
.const CH_COLON   = $3a
.const CH_QUESTION = $3f
.const CH_SPACE    = $20
.const CH_CLEAR    = $93          // clear screen, what BASIC prints first
.const CH_TAB      = $09
.const CH_LF       = $0a
.const CH_CR       = $0d
.const CH_DOT      = $2e
.const CH_SLASH    = $2f

// PETSCII box drawing, as CHROUT expects it.
.const CH_HBAR      = $c0
.const CH_VBAR      = $dd
.const CH_CORNER_TL = $b0
.const CH_CORNER_TR = $ae
.const CH_CORNER_BL = $ad
.const CH_CORNER_BR = $bd
.const HELP_WIDTH   = 40
// BASIC tokenises operators before a line is executed, so by the time the
// dispatcher sees the line, '/' has become $AD and the up arrow $AE. Comparing
// against $2F and $5E never matches. '@' is not an operator and survives as is.
.const TOK_RUN     = $8a          // the RUN keyword, as BASIC stores it
.const TOK_PRINT   = $99          // what '?' becomes when BASIC tokenises
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
.const BASIC_KEYWORDS = $a09e           // token $80 is the first entry
.const IGONE    = $0308                 // BASIC statement dispatch vector
.const BSOUT    = $0326                 // KERNAL character output vector
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

        jsr detectJiffy

        // Getting the greeting above BASIC's banner takes a detour. Printing
        // here is useless: BASIC's start-up clears the screen and wipes it.
        // JiffyDOS gets away with it because it replaces the KERNAL's start-up
        // message outright, which a cartridge cannot.
        //
        // So hook the character-output vector instead. The clear is the first
        // thing BASIC prints; the hook lets it through, prints the greeting on
        // the fresh screen, and steps aside so BASIC's own banner follows below.
        lda BSOUT
        sta origBsout
        lda BSOUT + 1
        sta origBsout + 1
        lda #<bannerHook
        sta BSOUT
        lda #>bannerHook
        sta BSOUT + 1

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

        jmp KERNAL_IRQ

// Sits in the character-output vector for exactly one call: the screen clear
// that opens BASIC's start-up. The vector is restored first, because printing
// the greeting goes through CHROUT and would otherwise re-enter this routine.
// CHROUT is contracted to leave A, X, Y and the carry as it found them, and
// BASIC's start-up depends on it: the message is printed by $AB24, which keeps
// the string index in Y and the remaining length in X across every call. The
// greeting destroys both, so the whole hook saves and restores them - not only
// the register used to put the vector back.
bannerHook:
        sta hookChar
        txa
        pha
        tya
        pha

        lda origBsout
        sta BSOUT
        lda origBsout + 1
        sta BSOUT + 1

        lda hookChar
        cmp #CH_CLEAR
        bne !passThrough+

        jsr CHROUT                      // let the clear happen first
        php
        jsr printBanner
        jmp !done+
!passThrough:
        jsr CHROUT
        php
!done:
        plp
        pla
        tay
        pla
        tax
        lda hookChar
        rts

// Prints the greeting.
printBanner:
        ldx #<bannerText
        ldy #>bannerText
        jsr printString

        // Name the prefix that is actually usable on this machine.
        ldx #<hintStock
        ldy #>hintStock
        lda jiffyPresent
        beq !show+
        lda #CH_AMP
        sta prefixChar
        ldx #<hintJiffy
        ldy #>hintJiffy
!show:
        jmp printString

// JiffyDOS claims '@', '/' and the up arrow for its own wedge and intercepts
// them before BASIC's dispatcher ever runs, so on such a machine those prefixes
// are unreachable here. Scanning the KERNAL for its banner string is a more
// durable test than checking a fixed address, which moves between versions.
detectJiffy:
        lda #$00
        sta jiffyPresent
        lda #<$e000
        sta STRPTR
        lda #>$e000
        sta STRPTR + 1
scanLoop:
        ldy #$00
!compare:
        lda (STRPTR),y
        cmp jiffySig,y
        bne !advance+
        iny
        cpy #jiffySigEnd - jiffySig
        bne !compare-
        lda #$ff
        sta jiffyPresent
        rts
!advance:
        inc STRPTR
        bne scanLoop
        inc STRPTR + 1
        bne scanLoop                    // stop after wrapping past $FFFF
        rts

// Called by BASIC instead of the statement dispatcher. The default does CHRGET
// and falls into the dispatcher, so anything not ours must end up there too.
// CHRGET leaves flags that BASIC's statement executor depends on: zero for the
// end of a statement, carry clear for a digit. A cmp overwrites them, so they
// are saved across the comparisons - without this, PRINT is executed as PRINT#
// and RUN reports ?UNDEF'D STATEMENT.
wedgeHandler:
        jsr CHRGET
        php
        sta cmdChar

        // '&' always belongs to the wedge, on either kind of machine.
        cmp #CH_AMP
        beq !ours+

        // The classic prefixes are only ours when JiffyDOS is not present.
        ldx jiffyPresent
        bne !notOurs+

        cmp #CH_AT
        beq !ours+
        cmp #TOK_SLASH
        beq !ours+
        cmp #TOK_ARROWUP
        beq !ours+
!notOurs:
        plp
        jmp BASIC_DISPATCH
!ours:
        plp
        jmp wedgeCommand

// '&' and '@' are prefixes and carry the command in the next character; '/' and
// the up arrow are commands in their own right.
wedgeCommand:
        lda cmdChar
        cmp #CH_AMP
        beq !prefix+
        cmp #CH_AT
        bne !dispatch+
!prefix:
        jsr CHRGET
        sta cmdChar
!dispatch:
        lda cmdChar
        bne !notPath+
        jmp doPath                      // prefix alone: where am I?
!notPath:
        cmp #CH_DOLLAR
        bne !notDir+
        jmp doDirectory
!notDir:
        cmp #TOK_SLASH
        bne !notLoad+
        jmp doLoad
!notLoad:
        cmp #TOK_ARROWUP
        bne !notRun+
        jmp doLoadRun
!notRun:
        // "?" reaches us as $3F after '@', which suppresses tokenisation, and as
        // the PRINT token after '&', which does not. Accept both.
        cmp #CH_QUESTION
        bne !notHelp+
        jmp doHelp
!notHelp:
        cmp #TOK_PRINT
        bne !notHelp2+
        jmp doHelp
!notHelp2:
        cmp #CH_T
        bne !notType+
        jmp doType
!notType:

        // Everything else is a two-letter command.
        jsr CHRGET
        sta cmdChar2
        lda cmdChar

        cmp #CH_C
        beq !c+
        cmp #CH_D
        beq !d+
        cmp #CH_M
        beq !m+
        cmp #CH_R
        beq !r+
        cmp #CH_S
        beq !s+
        jmp unknownCommand
!c:
        lda cmdChar2
        cmp #CH_D
        bne unknownCommand
        jmp doChangeDir
!d:
        lda cmdChar2
        cmp #CH_R
        bne unknownCommand
        jmp doDriveInfo
!m:
        lda cmdChar2
        cmp #CH_D
        bne !notMd+
        jmp doMakeDir
!notMd:
        cmp #CH_T
        bne unknownCommand
        jmp doMount
!r:
        lda cmdChar2
        cmp #CH_M
        bne unknownCommand
        jmp doRemove
!s:
        lda cmdChar2
        cmp #CH_V
        bne !notSv+
        jmp doSave
!notSv:
        cmp #CH_W
        bne unknownCommand
        jmp doSwap

unknownCommand:
        ldx #<errText
        ldy #>errText
        jsr printString
        jmp endOfCommand

doDirectory:
        jsr directory
        jmp endOfCommand

doChangeDir:
        lda #DOS_CHANGE_DIR
        jsr simpleNameCommand
        jmp endOfCommand

doMakeDir:
        lda #DOS_CREATE_DIR
        jsr simpleNameCommand
        jmp endOfCommand

doRemove:
        lda #DOS_DELETE_FILE
        jsr simpleNameCommand
        jmp endOfCommand

doPath:
        jsr currentPath
        jmp endOfCommand

doType:
        jsr typeFile
        jmp endOfCommand

doDriveInfo:
        jsr driveInfo
        jmp endOfCommand

doHelp:
        jsr printHelp
        jmp endOfCommand

// ------------------------------------------------------------------- help

// Draws the command table. The rows carry no prefix character; it is printed in
// front of each one from prefixChar, so a single table serves both the stock
// '@' and the JiffyDOS '&'.
printHelp:
        jsr helpBorderTop

        lda #<helpRows
        sta STRPTR
        lda #>helpRows
        sta STRPTR + 1

!row:
        ldy #$00
        lda (STRPTR),y
        beq !done+                      // empty row ends the table

        lda #CH_VBAR
        jsr CHROUT
        lda #CH_SPACE
        jsr CHROUT
        lda prefixChar
        jsr CHROUT
        lda #$03
        sta helpCol                     // bar, space, prefix

!chars:
        ldy #$00
        lda (STRPTR),y
        jsr advanceHelp
        cmp #$00
        beq !pad+
        jsr CHROUT
        inc helpCol
        jmp !chars-

!pad:
        lda helpCol
        cmp #HELP_WIDTH - 1
        bcs !close+
        lda #CH_SPACE
        jsr CHROUT
        inc helpCol
        jmp !pad-
!close:
        lda #CH_VBAR
        jsr CHROUT
        jmp !row-

!done:
        jsr helpBorderBottom
        rts

advanceHelp:
        inc STRPTR
        bne !out+
        inc STRPTR + 1
!out:
        rts

helpBorderTop:
        lda #CH_CORNER_TL
        ldx #CH_CORNER_TR
        jmp helpBorder
helpBorderBottom:
        lda #CH_CORNER_BL
        ldx #CH_CORNER_BR
helpBorder:
        stx helpRightCorner
        jsr CHROUT
        ldx #HELP_WIDTH - 2
!line:
        lda #CH_HBAR
        jsr CHROUT
        dex
        bne !line-
        lda helpRightCorner
        jmp CHROUT

doMount:
        jsr mountImage
        jmp endOfCommand

doSwap:
        jsr swapDisk
        jmp endOfCommand

doSave:
        jsr saveProgram
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
// Hands the line back to BASIC as an empty one.
//
// Consuming the rest with CHRGET is not enough: a command that already read the
// terminator would make CHRGET step past it and into the leftovers of whatever
// longer line was typed before, which BASIC then tries to execute. Pointing the
// text pointer at a zero byte of our own makes the next CHRGET return end of
// line whatever the command did.
endOfCommand:
        lda #<(lineEnd - 1)
        sta TXTPTR
        lda #>(lineEnd - 1)
        sta TXTPTR + 1
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

// Discards a command whose bytes are already in the queue but which will never
// be pushed, as happens when the filename turns out to be missing. Without this
// the leftovers sit in front of the next command, which then fails with an
// error that has nothing to do with what was typed.
abortCommand:
        lda #ABORT
        sta UCI_CONTROL
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
        jsr advanceText
        jsr sendFilename
        cpx #$00
        bne !named+
        jsr abortCommand
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
        lda #<storeByte
        sta byteSink
        lda #>storeByte
        sta byteSink + 1
        jmp readFile

// The same stream, printed instead of stored.
readFileToScreen:
        lda #$00
        sta typeAborted
        sta prevByte
        lda #<typeByte
        sta byteSink
        lda #>typeByte
        sta byteSink + 1

readFile:
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
        jsr callSink
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

// 6502 has no indirect JSR, so the sink is reached through a JMP whose own RTS
// returns to the packet loop.
callSink:
        jmp (byteSink)

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

// "T:NAME" prints a text file.
//
// A .PRG is refused rather than shown: it holds a load address and tokenised
// BASIC or machine code, so every byte of it would be meaningless on screen.
typeFile:
        jsr uciPresent
        bcc !go+
        rts
!go:
        lda #TARGET_DOS
        sta UCI_CMD_DATA
        lda #DOS_OPEN_FILE
        sta UCI_CMD_DATA
        lda #FA_READ
        sta UCI_CMD_DATA

        jsr advanceText
        jsr skipSeparator
        jsr sendFilename
        cpx #$00
        bne !named+
        jsr abortCommand
        ldx #<noNameText
        ldy #>noNameText
        jmp printString
!named:
        lda #PUSH_CMD
        sta UCI_CONTROL
        jsr uciWait
        jsr uciReadStatus
        jsr uciAccept
        jsr uciReportError
        bcc !opened+
        rts
!opened:
        // The name has already gone into the command queue by this point, so
        // the suffix is checked after opening rather than before: discarding a
        // half written command would leave its bytes in the queue.
        jsr nameIsPrg
        bcc !show+
        jsr closeFile
        ldx #<prgText
        ldy #>prgText
        jsr printString
        lda prefixChar                  // name the load command that does work
        jsr CHROUT
        lda #CH_SLASH
        jsr CHROUT
        ldx #<prgText2
        ldy #>prgText2
        jmp printString
!show:
        jsr readFileToScreen
        jmp closeFile

// Carry set when the name just sent ends in ".PRG".
nameIsPrg:
        ldx #$03
!loop:
        lda nameTail,x
        cmp prgSuffix,x
        bne !no+
        dex
        bpl !loop-
        sec
        rts
!no:
        clc
        rts

// Prints one byte of a text file.
//
// What the encoding is depends on who wrote the file: a PC leaves ASCII with LF
// line endings, the C64 itself leaves PETSCII with CR. One translation serves
// both, because the two only part company in the letter range - digits, spaces
// and punctuation are the same byte in either. Lowercase is folded to uppercase
// so the screen stays in its start-up character set, and a lone LF becomes a CR
// while the LF of a CRLF pair is dropped.
//
// Bytes below $20 are not passed through: in PETSCII they would clear the
// screen, switch the character set or turn on reverse video, so a file that is
// not text could leave the machine in a state the user has to guess their way
// out of. They print as '.' instead.
typeByte:
        ldx typeAborted
        bne !out+
        tax                             // the untranslated byte, for prevByte

        cmp #CH_LF
        bne !notLf+
        lda prevByte
        cmp #CH_CR
        beq !store+                     // second half of a CRLF pair
        lda #CH_CR
        jmp !emit+
!notLf:
        txa
        cmp #CH_CR
        beq !emit+
        cmp #CH_TAB
        bne !notTab+
        lda #CH_SPACE
        jmp !emit+
!notTab:
        cmp #CH_SPACE
        bcc !dot+                       // any other control byte
        cmp #$60
        bcc !emit+                      // $20-$5F is common to both encodings
        cmp #$61
        bcc !dot+
        cmp #$7b
        bcs !high+
        and #$df                        // ASCII lowercase
        jmp !emit+
!high:
        cmp #$c1
        bcc !dot+
        cmp #$db
        bcs !dot+
        and #$7f                        // PETSCII uppercase from the mixed set
        jmp !emit+
!dot:
        lda #CH_DOT
!emit:
        jsr CHROUT
!store:
        stx prevByte
        jsr STOPKEY
        bne !out+
        lda #$ff
        sta typeAborted
!out:
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

// --------------------------------------------------------- name handling

// Sends the rest of the BASIC line to the command queue as a filename.
//
// Two details matter. The bytes are read straight through the text pointer
// rather than with CHRGET, because CHRGET skips spaces and filenames may
// contain them. And BASIC may have tokenised parts of the name: after '@' it
// leaves the line alone, but after '&' it does not, so "PRINTER" arrives as the
// PRINT token followed by "ER". Tokens are expanded back into their keywords.
//
// Returns with X non-zero when at least one character was sent.
sendFilename:
        ldx #$03
!clearTail:
        lda #$00
        sta nameTail,x
        dex
        bpl !clearTail-

        ldx #$00
!loop:
        ldy #$00
        lda (TXTPTR),y
        beq !done+

        inc TXTPTR
        bne !advanced+
        inc TXTPTR + 1
!advanced:
        cmp #$80
        bcc !plain+
        jsr sendToken
        inx
        jmp !loop-
!plain:
        sta UCI_CMD_DATA

        // Keep the last four characters, so a caller can look at the suffix
        // without buffering the whole name.
        pha
        lda nameTail + 1
        sta nameTail
        lda nameTail + 2
        sta nameTail + 1
        lda nameTail + 3
        sta nameTail + 2
        pla
        sta nameTail + 3

        inx
        jmp !loop-
!done:
        rts

// Expands one BASIC token in A into its keyword and sends the letters.
// The keyword table starts at $A09E; each entry ends with a byte whose high
// bit is set, and token $80 is the first entry.
sendToken:
        sec
        sbc #$80
        tax
        lda #<BASIC_KEYWORDS
        sta KWPTR
        lda #>BASIC_KEYWORDS
        sta KWPTR + 1

!skipEntry:
        cpx #$00
        beq !emit+
!skipChar:
        ldy #$00
        lda (KWPTR),y
        jsr advanceKeyword
        and #$80
        beq !skipChar-
        dex
        jmp !skipEntry-

!emit:
        ldy #$00
        lda (KWPTR),y
        pha
        and #$7f
        sta UCI_CMD_DATA
        jsr advanceKeyword
        pla
        and #$80
        beq !emit-
        rts

advanceKeyword:
        inc KWPTR
        bne !out+
        inc KWPTR + 1
!out:
        rts

// CHRGET leaves the text pointer on the character it just returned, so the
// direct reads below would see it again. Step past it once.
advanceText:
        inc TXTPTR
        bne !out+
        inc TXTPTR + 1
!out:
        rts

// An optional drive number right after the command, as in "&MT9:NAME". Drive A
// is not always bus 8 - on this machine it answers on 9 - so the id has to be
// selectable. Without one, DEFAULT_DRIVE_ID is used and the Ultimate falls back
// to the drive last mounted on.
readDriveId:
        lda #DEFAULT_DRIVE_ID
        sta driveId
        ldy #$00
        lda (TXTPTR),y
        cmp #CH_ZERO
        bcc !out+
        cmp #CH_ZERO + 10
        bcs !out+
        sec
        sbc #CH_ZERO
        sta driveId
        jsr advanceText
!out:
        rts

// Skips a ':' separator if the command has one.
skipSeparator:
        ldy #$00
        lda (TXTPTR),y
        cmp #CH_COLON
        bne !out+
        inc TXTPTR
        bne !out+
        inc TXTPTR + 1
!out:
        rts

// Commands shaped "<code> <name>": change directory, create directory, delete.
// A is the DOS command byte.
simpleNameCommand:
        sta dosCommand
        jsr uciPresent
        bcc !go+
        rts
!go:
        jsr advanceText
        jsr skipSeparator

        lda #TARGET_DOS
        sta UCI_CMD_DATA
        lda dosCommand
        sta UCI_CMD_DATA
        jsr sendFilename
        cpx #$00
        beq !noName+

        lda #PUSH_CMD
        sta UCI_CONTROL
        jsr uciWait
        jsr printStatus
        jsr uciAccept
        rts
!noName:
        jsr abortCommand
        ldx #<noNameText
        ldy #>noNameText
        jsr printString
        rts

// ------------------------------------------------- @ current path, @MT, @SW

// "Get Path" returns the current directory on the data channel.
currentPath:
        jsr uciPresent
        bcc !go+
        rts
!go:
        lda #TARGET_DOS
        sta UCI_CMD_DATA
        lda #DOS_GET_PATH
        sta UCI_CMD_DATA
        lda #PUSH_CMD
        sta UCI_CONTROL
        jsr uciWait

!chars:
        lda UCI_STATUS
        and #ST_DATA_AV
        beq !done+
        lda UCI_RESP_DATA
        jsr toPetscii
        jsr CHROUT
        jmp !chars-
!done:
        lda #13
        jsr CHROUT
        jsr uciDrainStatus
        jsr uciAccept
        rts

// Mounts a disk image on the drive with the given IEC id. Passing the id of a
// drive that does not exist makes the Ultimate use the last one mounted on.
mountImage:
        jsr uciPresent
        bcc !go+
        rts
!go:
        jsr advanceText
        jsr readDriveId
        jsr skipSeparator

        lda #TARGET_DOS
        sta UCI_CMD_DATA
        lda #DOS_MOUNT_DISK
        sta UCI_CMD_DATA
        lda driveId
        sta UCI_CMD_DATA
        jsr sendFilename
        cpx #$00
        beq !noName+

        lda #PUSH_CMD
        sta UCI_CONTROL
        jsr uciWait
        jsr printStatus
        jsr uciAccept
        rts
!noName:
        jsr abortCommand
        ldx #<noNameText
        ldy #>noNameText
        jsr printString
        rts

// The same action as holding the menu button to swap to the next disk.
swapDisk:
        jsr uciPresent
        bcc !go+
        rts
!go:
        jsr advanceText
        jsr readDriveId

        lda #TARGET_DOS
        sta UCI_CMD_DATA
        lda #DOS_SWAP_DISK
        sta UCI_CMD_DATA
        lda driveId
        sta UCI_CMD_DATA
        lda #PUSH_CMD
        sta UCI_CONTROL
        jsr uciWait
        jsr printStatus
        jsr uciAccept
        rts

// ----------------------------------------------------------------- @DR

// Lists the drives the Ultimate presents on the IEC bus, with the address each
// one answers on. Without this the id that @MT and @SW take has to be guessed,
// and the guess is often wrong: drive A is not always 8 - on the machine this
// was developed against it is 9, and mounting on the wrong id reports
// "90,DRIVE NOT PRESENT" with nothing to suggest what the right one would be.
//
// The reply is a count byte followed by three bytes per drive: type, IEC
// address, power state. The argument asks for the address the drive actually
// answers on rather than the configured one.
driveInfo:
        jsr uciPresent
        bcc !go+
        rts
!go:
        lda #TARGET_CONTROL
        sta UCI_CMD_DATA
        lda #CTRL_GET_DRVINFO
        sta UCI_CMD_DATA
        lda #$01
        sta UCI_CMD_DATA
        lda #PUSH_CMD
        sta UCI_CONTROL

        jsr readDrvInfo
        lda drvLen
        beq !out+

        // Trust the bytes that arrived over the count that was announced. The
        // two need not agree - the reply can be split across packets - and
        // waiting for a group that is not there would hang the machine.
        lda drvBuf
        sta driveCount
!clamp:
        lda driveCount
        beq !out+
        asl                             // three bytes per drive, plus the count
        clc
        adc driveCount
        clc
        adc #$01
        cmp drvLen
        bcc !fits+
        beq !fits+
        dec driveCount
        jmp !clamp-
!fits:
        ldx #<drvHeadText
        ldy #>drvHeadText
        jsr printString

        lda #$01
        sta drvIdx
!each:
        ldx drvIdx
        lda drvBuf,x
        sta driveType
        lda drvBuf + 1,x
        sta driveBus
        lda drvBuf + 2,x
        sta drivePower
        jsr printDrive

        lda drvIdx
        clc
        adc #$03
        sta drvIdx
        dec driveCount
        bne !each-
!out:
        rts

// Collects the whole reply before anything is printed. A packet has to be
// accepted before the next one is sent, so reading and formatting cannot be
// interleaved: a reader that only polls for data spins forever the moment the
// reply does not fit in one packet.
readDrvInfo:
        lda #$00
        sta drvLen
!packet:
        jsr uciWait
!bytes:
        lda UCI_STATUS
        and #ST_DATA_AV
        beq !packetDone+
        lda UCI_RESP_DATA
        ldx drvLen
        cpx #DRVBUF_SIZE
        bcs !bytes-                     // buffer full, drop the rest
        sta drvBuf,x
        inc drvLen
        jmp !bytes-
!packetDone:
        lda UCI_STATUS
        and #ST_STATE_MASK
        cmp #ST_STATE_MORE
        php
        jsr uciAccept
        plp
        beq !packet-
        jmp uciDrainStatus

printDrive:
        lda driveBus
        jsr printDec2
        lda #CH_SPACE
        jsr CHROUT
        jsr printDriveType
        lda drivePower
        beq !off+
        ldx #<onText
        ldy #>onText
        jmp printString
!off:
        ldx #<offText
        ldy #>offText
        jmp printString

// Each table entry is the type byte followed by seven characters, so every
// name occupies the same width and the power column stays aligned.
printDriveType:
        ldx #$00
!scan:
        lda driveTypes,x
        cmp #$ff
        beq !unknown+
        cmp driveType
        beq !emit+
        txa
        clc
        adc #DRIVE_TYPE_ENTRY
        tax
        jmp !scan-
!emit:
        inx
        ldy #DRIVE_TYPE_ENTRY - 1
!char:
        lda driveTypes,x
        jsr CHROUT
        inx
        dey
        bne !char-
        rts
!unknown:
        ldx #$00
!other:
        lda unknownTypeText,x
        jsr CHROUT
        inx
        cpx #DRIVE_TYPE_ENTRY - 1
        bne !other-
        rts

// Prints A as two characters, space padded, for values below 100.
printDec2:
        ldx #CH_ZERO
!tens:
        cmp #10
        bcc !ones+
        sbc #10
        inx
        jmp !tens-
!ones:
        pha
        cpx #CH_ZERO
        bne !tensDigit+
        ldx #CH_SPACE                   // no leading zero
!tensDigit:
        txa
        jsr CHROUT
        pla
        clc
        adc #CH_ZERO
        jmp CHROUT

// ------------------------------------------------------------- @SV:NAME

// Writes the BASIC program in memory to a file, load address first, so the
// result is a .prg that loads back with /NAME.
//
// The command queue holds 896 bytes, so the program is sent in chunks well
// inside that. FA_WRITE + FA_CREATE_NEW truncates an existing file rather than
// appending to it.
saveProgram:
        jsr uciPresent
        bcc !go+
        rts
!go:
        jsr advanceText
        jsr skipSeparator

        lda #TARGET_DOS
        sta UCI_CMD_DATA
        lda #DOS_OPEN_FILE
        sta UCI_CMD_DATA
        lda #FA_WRITE | FA_CREATE_NEW | FA_CREATE_ALWAYS
        sta UCI_CMD_DATA
        jsr sendFilename
        cpx #$00
        bne !named+
        jsr abortCommand
        ldx #<noNameText
        ldy #>noNameText
        jsr printString
        rts
!named:
        lda #PUSH_CMD
        sta UCI_CONTROL
        jsr uciWait
        jsr uciReadStatus
        jsr uciAccept
        jsr uciReportError
        bcc !opened+
        rts
!opened:
        // Source pointer starts at the beginning of the BASIC program.
        lda TXTTAB
        sta SRCPTR
        lda TXTTAB + 1
        sta SRCPTR + 1

        // First packet carries the two load address bytes.
        lda #TARGET_DOS
        sta UCI_CMD_DATA
        lda #DOS_WRITE_DATA
        sta UCI_CMD_DATA
        lda #$00
        sta UCI_CMD_DATA                // two alignment bytes
        sta UCI_CMD_DATA
        lda TXTTAB
        sta UCI_CMD_DATA
        lda TXTTAB + 1
        sta UCI_CMD_DATA

        ldy #$00
!bytes:
        jsr sourceAtEnd
        bcs !flush+
        ldy #$00
        lda (SRCPTR),y
        sta UCI_CMD_DATA
        inc SRCPTR
        bne !counted+
        inc SRCPTR + 1
!counted:
        inc chunkCount
        lda chunkCount
        cmp #SAVE_CHUNK
        bne !bytes-

        // Chunk full: push it and start the next one.
        jsr saveFlush
        lda #TARGET_DOS
        sta UCI_CMD_DATA
        lda #DOS_WRITE_DATA
        sta UCI_CMD_DATA
        lda #$00
        sta UCI_CMD_DATA
        sta UCI_CMD_DATA
        jmp !bytes-

!flush:
        jsr saveFlush
        jsr closeFile

        ldx #<savedText
        ldy #>savedText
        jsr printString
        rts

saveFlush:
        lda #$00
        sta chunkCount
        lda #PUSH_CMD
        sta UCI_CONTROL
        jsr uciWait
        jsr uciDrainStatus
        jsr uciAccept
        rts

// Carry set once the source pointer has reached the end of the program.
sourceAtEnd:
        lda SRCPTR + 1
        cmp VARTAB + 1
        bcc !more+
        bne !end+
        lda SRCPTR
        cmp VARTAB
        bcc !more+
!end:
        sec
        rts
!more:
        clc
        rts

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

origBsout:    .word $0000
hookChar:     .byte $00
byteSink:     .word $0000
prevByte:     .byte $00
typeAborted:  .byte $00
driveCount:   .byte $00
drvIdx:       .byte $00
drvLen:       .byte $00
drvBuf:       .fill DRVBUF_SIZE, 0
driveType:    .byte $00
driveBus:     .byte $00
drivePower:   .byte $00
nameTail:     .fill 4, 0
entryAttr:    .byte $00
runAfterLoad: .byte $00
cmdChar:      .byte $00
cmdChar2:     .byte $00
dosCommand:   .byte $00
chunkCount:   .byte $00
driveId:      .byte $00
prefixChar:   .byte CH_AT
helpCol:      .byte $00
helpRightCorner: .byte $00
jiffyPresent: .byte $00
headerCount:  .byte $00
loadAddr:     .word $0000
runLine:      .byte TOK_RUN, $00      // a tokenised "RUN", executed after load
lineEnd:      .byte $00, $00          // an empty line to hand back to BASIC
statusBuf:    .fill STATUS_MAX + 1, 0

// These strings go to CHROUT, which takes PETSCII. Kick Assembler's default
// encoding is screencode_mixed, where uppercase letters happen to survive the
// round trip but '@' becomes $00 - which silently truncates a string.
.encoding "petscii_upper"

// The greeting sits above BASIC's own start-up message, so it stays one line:
// the hint is appended, not put on a line of its own.
bannerText: .byte 13
            .text "UCI WEDGE BY CYBERSORCERER "
            .byte 0

hintStock:  .text "@? FOR HELP"
            .byte 13, 0

// JiffyDOS owns @, / and the up arrow, so everything moves behind '&'.
hintJiffy:  .text "&? FOR HELP"
            .byte 13, 0

// One row per command, without the prefix; printHelp puts it in front.
helpRows:   .text "$        DIRECTORY"
            .byte 0
            .text "         CURRENT PATH"
            .byte 0
            .text "CD:NAME  CHANGE DIRECTORY"
            .byte 0
            .text "MD:NAME  CREATE DIRECTORY"
            .byte 0
            .text "RM:NAME  DELETE FILE"
            .byte 0
            .text "T:NAME   SHOW TEXT FILE"
            .byte 0
            .text "SV:NAME  SAVE BASIC PROGRAM"
            .byte 0
            .text "/NAME    LOAD"
            .byte 0
            .byte $5e
            .text "NAME    LOAD AND RUN"
            .byte 0
            .text "MT9:NAME MOUNT DISK IMAGE"
            .byte 0
            .text "SW9      SWAP TO NEXT DISK"
            .byte 0
            .text "DR       LIST DRIVES"
            .byte 0
            .byte 0                     // empty row: end of table
dirText:    .text "  <DIR>"
            .byte 0
jiffySig:   .text "JIFFYDOS"
jiffySigEnd:

errText:    .text "?UNKNOWN WEDGE COMMAND"
            .byte 13, 0
noNameText: .text "?MISSING FILENAME"
            .byte 13, 0
readyText:  .text "LOADED"
            .byte 13, 0
savedText:  .text "SAVED"
            .byte 13, 0
noUciText:  .text "?COMMAND INTERFACE DISABLED"
            .byte 13, 0
prgText:    .text "?NOT TEXT - USE "
            .byte 0
prgText2:   .text " TO LOAD"
            .byte 13, 0
prgSuffix:  .text ".PRG"

drvHeadText: .text "ID TYPE    POWER"
            .byte 13, 0
onText:     .text " ON"
            .byte 13, 0
offText:    .text " OFF"
            .byte 13, 0
unknownTypeText: .text "?      "

// Type byte, then a seven character name. The values are the ones the Ultimate
// documents for CTRL_CMD_GET_DRVINFO; $FF ends the table.
driveTypes:
            .byte $00
            .text "1541   "
            .byte $01
            .text "1571   "
            .byte $02
            .text "1581   "
            .byte $03
            .text "UNSET  "
            .byte $0f
            .text "SOFTIEC"
            .byte $50
            .text "PRINTER"
            .byte $ff
residentEnd:
}

.print "resident size: " + (residentEnd - resident) + " bytes"
.errorif (residentEnd - resident) > RESIDENT_PAGES * $100, "resident part outgrew RESIDENT_PAGES - raise it"

// Pad to a full 8 KB image.
* = $9fff
        .byte $00
