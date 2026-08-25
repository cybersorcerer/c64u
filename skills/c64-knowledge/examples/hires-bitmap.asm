// Standard hires bitmap mode, 320x200, with a plotted sine curve.
//
// Shows the three things bitmap mode needs that text mode does not:
//   - BMM set in $D011, and the bitmap selected by bit 3 of $D018,
//   - the video matrix repurposed as a colour map: one byte per 8x8 cell,
//     high nibble foreground, low nibble background,
//   - an address calculation, because the bitmap is stored in character-cell
//     order, not scanline order.
//
// Build: java -jar KickAss.jar hires-bitmap.asm -o hires-bitmap.prg
// Run:   c64u runners run-prg-upload hires-bitmap.prg

.const BITMAP = $2000           // 8000 bytes, 8 KB aligned, VIC bank 0
.const SCREEN = $0400           // colour map in bitmap mode

.const PTR  = $fb               // zero page work pointer
.const XPOS = $fd
.const YPOS = $fe

BasicUpstart2(start)

start:
        jsr clearBitmap
        jsr fillColourMap

        lda #((SCREEN / 1024) << 4) | %00001000
        sta $d018               // video matrix at $0400, bitmap at $2000
        lda #%00111011
        sta $d011               // BMM on, display enabled, 25 rows, yscroll 3
        lda #%11001000
        sta $d016               // 40 columns, no multicolour

        lda #$00
        sta XPOS
!loop:
        ldx XPOS
        lda sine,x
        sta YPOS
        jsr plot
        inc XPOS
        bne !loop-              // 256 columns, then wrap to 0 and stop

        rts                     // back to BASIC; the picture stays up

// Sets one pixel at (XPOS, YPOS).
// Address = BITMAP + (y>>3)*320 + (x & $F8) + (y & 7), bit $80 >> (x & 7).
plot:
        lda YPOS
        lsr
        lsr
        lsr
        tax                     // character row
        lda rows.lo,x
        sta PTR
        lda rows.hi,x
        sta PTR + 1

        lda XPOS
        and #$f8                // (x>>3)*8 collapses to this for x < 256
        clc
        adc PTR
        sta PTR
        lda PTR + 1
        adc #$00
        sta PTR + 1

        lda YPOS
        and #$07
        tay                     // line within the cell
        lda XPOS
        and #$07
        tax
        lda bits,x
        ora (PTR),y
        sta (PTR),y
        rts

clearBitmap:
        lda #$00
        ldx #$00
!loop:
        .for (var page = 0; page < 32; page++) {
                sta BITMAP + page * $100, x
        }
        inx
        bne !loop-
        rts

fillColourMap:
        lda #$16                // white foreground on blue background
        ldx #$00
!loop:
        sta SCREEN, x
        sta SCREEN + $100, x
        sta SCREEN + $200, x
        sta SCREEN + $2e8, x
        inx
        bne !loop-
        rts

// Start address of each character row: BITMAP + row * 320.
rows:   .lohifill 25, BITMAP + 320 * i

bits:   .byte $80, $40, $20, $10, $08, $04, $02, $01

// One y value per x, centred on 100 with an amplitude of 90.
sine:   .fill 256, round(100 + 90 * sin(toRadians(i * 360 / 256)))
