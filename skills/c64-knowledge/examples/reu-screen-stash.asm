// Save, restore, exchange and compare screen RAM through an REU.
//
//   F1  STASH  - copy screen RAM into the REU
//   F3  FETCH  - copy it back
//   F5  SWAP   - exchange screen RAM and REU contents
//   F7  VERIFY - compare; border turns green if identical, red if not
//   RUN/STOP   - exit to BASIC
//
// The command bytes are built from the $DF01 bit fields rather than copied:
//   bit 7 execute, bit 5 AUTOLOAD, bit 4 start now, bits 1-0 the command.
// AUTOLOAD restores $DF02-$DF08 after each transfer, so the parameters are
// written once at startup and every later operation is a single store.
//
// Build: java -jar KickAss.jar reu-screen-stash.asm -o reu-screen-stash.prg
// Run:   c64u runners run-prg-upload reu-screen-stash.prg
//
// Needs an REU enabled on the machine. On a C64 Ultimate, turn on the RAM
// Expansion Unit in the machine configuration first.

.const REU_STATUS   = $df00
.const REU_COMMAND  = $df01
.const REU_C64      = $df02             // and $df03
.const REU_ADDR     = $df04             // and $df05
.const REU_BANK     = $df06
.const REU_LEN      = $df07             // and $df08
.const REU_IRQMASK  = $df09
.const REU_ADDRCTL  = $df0a

// execute + AUTOLOAD + start now + command
.const CMD_STASH    = %10110000
.const CMD_FETCH    = %10110001
.const CMD_SWAP     = %10110010
.const CMD_VERIFY   = %10110011

.const SCREEN       = $0400
.const SCREEN_SIZE  = 40 * 25           // 1000 bytes

.const GETIN        = $ffe4

BasicUpstart2(start)

start:
        jsr setupREU
loop:
        jsr GETIN
        cmp #133                        // F1
        beq doStash
        cmp #134                        // F3
        beq doFetch
        cmp #135                        // F5
        beq doSwap
        cmp #136                        // F7
        beq doVerify
        cmp #$03                        // RUN/STOP
        beq done
        jmp loop
done:
        rts

doStash:
        lda #CMD_STASH
        sta REU_COMMAND                 // the CPU halts here until the DMA ends
        jmp loop

doFetch:
        lda #CMD_FETCH
        sta REU_COMMAND
        jmp loop

doSwap:
        lda #CMD_SWAP
        sta REU_COMMAND
        jmp loop

doVerify:
        lda REU_STATUS                  // clear bits 7-5 before the operation
        lda #CMD_VERIFY
        sta REU_COMMAND
        lda REU_STATUS
        and #%00100000                  // bit 5: a difference was found
        beq !same+
        lda #RED
        sta $d020
        jmp loop
!same:
        lda #GREEN
        sta $d020
        jmp loop

// Written once. AUTOLOAD keeps these values valid across every transfer.
setupREU:
        lda #<SCREEN
        sta REU_C64
        lda #>SCREEN
        sta REU_C64 + 1

        lda #$00
        sta REU_ADDR                    // REU offset $0000 ...
        sta REU_ADDR + 1
        sta REU_BANK                    // ... in bank 0
        sta REU_IRQMASK                 // no REU interrupts
        sta REU_ADDRCTL                 // increment both addresses

        lda #<SCREEN_SIZE
        sta REU_LEN
        lda #>SCREEN_SIZE
        sta REU_LEN + 1
        rts
