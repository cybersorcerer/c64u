// Detect a RAM Expansion Unit without triggering a DMA transfer.
//
// Border turns green if an REU answers, red if it does not.
//
// Two things make this safe where naive probes are not:
//   - $DF01 is never written. A stray write there starts a transfer with
//     whatever addresses happen to be in the registers, which can overwrite
//     arbitrary memory on a machine that does have an REU.
//   - the status register is not assumed to be non-zero. A 1700 (128 KB)
//     legitimately reads $00 there, which is why the check on the original
//     1764 demo disk misreports those units.
//
// Build: java -jar KickAss.jar reu-detect.asm -o reu-detect.prg
// Run:   c64u runners run-prg-upload reu-detect.prg

.const REU_STATUS  = $df00
.const REU_COMMAND = $df01
.const REU_C64     = $df02

BasicUpstart2(start)

start:
        jsr detectREU
        cpy #$00
        beq !found+
        lda #RED
        sta $d020
        rts
!found:
        lda #GREEN
        sta $d020
        rts

// Returns Y = 0 when an REU is present, Y != 0 otherwise.
// Clobbers A, X, Y and the flags.
detectREU:
        ldy #$ff                        // assume no REU

        lda REU_STATUS                  // reading clears status bits 7-5
        sty REU_STATUS                  // read-only, so $FF must not stick
        cpy REU_STATUS
        beq !done+                      // it stuck -> nothing is decoding $DF00

        // Probe $DF05 down to $DF02. Those four hold whatever is written to
        // them. X never reaches 0, so $DF01 is never touched.
        ldx #$04
!loop:
        txa
        sta REU_COMMAND,x               // $DF01 + x, i.e. $DF05 .. $DF02
        cmp REU_COMMAND,x
        bne !done+                      // did not read back -> no REU
        dex
        bne !loop-

        ldy #$00                        // all four matched
!done:
        rts
