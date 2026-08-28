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
.const RESIDENT_PAGES = 8

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
.const BASIC_KEYWORDS = $a09e           // token $80 is the first entry
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

        jsr detectJiffy

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

        // Show the prefix that is actually usable on this machine.
        ldx #<helpStock
        ldy #>helpStock
        lda jiffyPresent
        beq !show+
        ldx #<helpJiffy
        ldy #>helpJiffy
!show:
        jsr printString

        jmp (IMAIN)

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

        // Everything else is a two-letter command.
        jsr CHRGET
        sta cmdChar2
        lda cmdChar

        cmp #CH_C
        beq !c+
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

origMain:     .word $0000
entryAttr:    .byte $00
runAfterLoad: .byte $00
cmdChar:      .byte $00
cmdChar2:     .byte $00
dosCommand:   .byte $00
chunkCount:   .byte $00
driveId:      .byte $00
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

bannerText: .byte 13
            .text "C64U ULTIMATE WEDGE BY CYBERSORCERER"
            .byte 13, 0

helpStock:  .text "@$ DIR  @CD:NAME  /LOAD  "
            .byte $5e                   // up arrow, no ASCII equivalent
            .text "LOADRUN"
            .byte 13
            .text "@SV: @MD: @RM: @MT: @SW   @=PATH"
            .byte 13, 0

// JiffyDOS owns @, / and the up arrow, so everything moves behind '&'.
helpJiffy:  .text "&$ DIR  &CD:NAME  &/LOAD  &"
            .byte $5e
            .text "LOADRUN"
            .byte 13
            .text "&SV: &MD: &RM: &MT: &SW   &=PATH"
            .byte 13, 0
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
residentEnd:
}

.print "resident size: " + (residentEnd - resident) + " bytes"
.errorif (residentEnd - resident) > RESIDENT_PAGES * $100, "resident part outgrew RESIDENT_PAGES - raise it"

// Pad to a full 8 KB image.
* = $9fff
        .byte $00
