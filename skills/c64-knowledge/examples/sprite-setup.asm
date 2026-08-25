// Put sprite 0 on screen: pointer, position, colour, X MSB.
//
// The sprite pointer is video-matrix-base + $03F8, which is $07F8 for the
// default screen. It holds data-address / 64 within the current VIC bank,
// so sprite data must start on a 64-byte boundary.
//
// Build: java -jar KickAss.jar sprite-setup.asm -o sprite-setup.prg
// Run:   c64u runners run-prg-upload sprite-setup.prg

.const SCREEN       = $0400
.const SPRITE_PTR   = SCREEN + $03f8
.const SPRITE_DATA  = $0c00             // 64-byte aligned, free RAM, VIC bank 0

BasicUpstart2(start)

start:
        lda #SPRITE_DATA / 64
        sta SPRITE_PTR                  // sprite 0 reads its 63 bytes from here

        lda #160
        sta $d000                       // sprite 0 X, low 8 bits
        lda #100
        sta $d001                       // sprite 0 Y
        lda #%00000000
        sta $d010                       // X bit 8 clear -> X stays below 256

        lda #LIGHT_BLUE
        sta $d027                       // sprite 0 colour

        lda #%00000000
        sta $d01c                       // single colour
        sta $d017                       // no Y expand
        sta $d01d                       // no X expand

        lda #%00000001
        sta $d015                       // enable sprite 0 - do this last

        rts                             // back to BASIC, sprite stays up

// 24 x 21 pixels = 3 bytes per row, 21 rows, 63 bytes + 1 padding byte.
*= SPRITE_DATA "Sprite 0 data"
        .byte %00000000, %11111111, %00000000
        .byte %00000011, %11111111, %11000000
        .byte %00001111, %11111111, %11110000
        .byte %00011111, %11111111, %11111000
        .byte %00111111, %11111111, %11111100
        .byte %01111111, %11111111, %11111110
        .byte %01111110, %00000000, %01111110
        .byte %11111100, %00000000, %00111111
        .byte %11111000, %00000000, %00011111
        .byte %11110000, %00000000, %00001111
        .byte %11110000, %00000000, %00001111
        .byte %11110000, %00000000, %00001111
        .byte %11111000, %00000000, %00011111
        .byte %11111100, %00000000, %00111111
        .byte %01111110, %00000000, %01111110
        .byte %01111111, %11111111, %11111110
        .byte %00111111, %11111111, %11111100
        .byte %00011111, %11111111, %11111000
        .byte %00001111, %11111111, %11110000
        .byte %00000011, %11111111, %11000000
        .byte %00000000, %11111111, %00000000
        .byte $00                       // pad to 64 bytes
