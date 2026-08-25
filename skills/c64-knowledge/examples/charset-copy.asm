// Copy the character ROM into RAM and point the VIC at the copy, so
// individual characters can be redefined.
//
// Two separate banking rules collide here:
//   - the CPU sees character ROM at $D000 only while $01 bit 2 (CHAREN) is 0,
//     and that also hides all I/O, so interrupts must be off,
//   - the VIC sees character data at an offset inside its own 16 KB bank,
//     selected by $D018 bits 3-1.
//
// Build: java -jar KickAss.jar charset-copy.asm -o charset-copy.prg
// Run:   c64u runners run-prg-upload charset-copy.prg

.const CHARSET_ROM = $d000
.const CHARSET_RAM = $3000              // 2 KB aligned, VIC bank 0, not $1000-$1FFF
.const SCREEN      = $0400

.const SRC = $fb                        // zero page pointers
.const DST = $fd

BasicUpstart2(start)

start:
        jsr copyCharset

        // $D018: video matrix = SCREEN/1024 in bits 7-4,
        //        character base = CHARSET_RAM/2048 in bits 3-1.
        lda #((SCREEN / 1024) << 4) | ((CHARSET_RAM / 2048) << 1)
        sta $d018

        // Redefine screen code 0 ('@') as a solid block, to prove the copy is live.
        ldx #$00
!fill:  lda #$ff
        sta CHARSET_RAM,x
        inx
        cpx #$08
        bne !fill-

        lda #$00
        sta SCREEN                      // draw screen code 0 in the top left corner
        lda #WHITE
        sta $d800

        rts

// Copies 4096 bytes (256 characters, both cases) from ROM to RAM.
copyCharset:
        sei                             // no interrupts while I/O is banked out
        lda $01
        pha
        and #%11111011                  // CHAREN = 0: character ROM at $D000
        sta $01

        lda #<CHARSET_ROM
        sta SRC
        lda #>CHARSET_ROM
        sta SRC + 1
        lda #<CHARSET_RAM
        sta DST
        lda #>CHARSET_RAM
        sta DST + 1

        ldx #16                         // 16 pages of 256 bytes
        ldy #$00
!page:
!byte:  lda (SRC),y
        sta (DST),y
        iny
        bne !byte-
        inc SRC + 1
        inc DST + 1
        dex
        bne !page-

        pla
        sta $01                         // restore the previous banking
        cli
        rts
