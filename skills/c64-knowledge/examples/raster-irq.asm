// Raster interrupt with the KERNAL banked out.
//
// Demonstrates the three things that go wrong most often:
//   - silencing the CIA timer interrupts before taking over,
//   - using the hardware vector at $FFFE because $01 = $35 removes the KERNAL,
//   - acknowledging $D019 so the interrupt does not re-fire forever.
//
// Build: java -jar KickAss.jar raster-irq.asm -o raster-irq.prg
// Run:   c64u runners run-prg-upload raster-irq.prg

.const RASTER_LINE = $80

BasicUpstart2(start)

start:
        sei

        lda #$7f
        sta $dc0d               // disable all CIA 1 interrupt sources
        sta $dd0d               // disable all CIA 2 interrupt sources
        lda $dc0d               // reading acknowledges anything pending
        lda $dd0d

        lda #$35                // RAM at $A000 and $E000, I/O still visible.
        sta $01                 // The KERNAL IRQ handler is gone from here on.

        lda #<irq
        sta $fffe               // hardware IRQ vector, now in RAM
        lda #>irq
        sta $ffff

        lda #RASTER_LINE
        sta $d012
        lda $d011
        and #$7f                // RST8 = 0: the compare line is below 256
        sta $d011

        lda #$01
        sta $d01a               // enable the raster interrupt source
        asl $d019               // acknowledge any pending VIC interrupt

        cli
        jmp *                   // everything happens in the interrupt now

irq:
        // No KERNAL handler ran, so nothing was saved for us.
        pha
        txa
        pha
        tya
        pha

        asl $d019               // acknowledge, or this fires again immediately

        lda #WHITE
        sta $d020
        ldx #$10
!wait:  dex                     // burn a few hundred cycles to make the band visible
        bne !wait-
        lda #BLACK
        sta $d020

        pla
        tay
        pla
        tax
        pla
        rti
