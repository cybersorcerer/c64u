# 6502 / 6510 Opcodes and Cycles

**Generated file - do not edit.** Produced from the disassembler's own opcode table by
`go run ./internal/disasm/gendoc` in `tools/c64u`, so it cannot drift away from the
code that decodes these bytes. Regenerate with `make -C skills/c64-knowledge opcodes`.

## Timing basics

One cycle is one CPU clock: 985248 Hz on PAL, 1022727 Hz on NTSC. A PAL raster line is 63
cycles, an NTSC line 65.

Three things add cycles beyond the table:

1. **Page crossing.** An indexed read whose address crosses a 256-byte boundary costs one extra
   cycle, marked `+` below. Stores never pay it. Aligning tables with `.align $100` avoids it.
2. **Taken branches.** A branch costs 2 cycles when not taken, 3 when taken, 4 when the target
   is on another page.
3. **Badlines.** Every eighth raster line the VIC steals 40-43 cycles from the CPU, leaving
   about 20 of the usual 63. See `vic-ii.md`.

Read-modify-write instructions write the unmodified value back before the modified one. That
extra write is visible to hardware: `inc $d019` acknowledges a VIC interrupt as a side effect,
which is why `asl $d019` is the idiom for acknowledging raster interrupts.

## Opcode grid

Cell shows the mnemonic and the cycle count. `+` means one more cycle when an
indexed address crosses a page. Branches show not taken / taken / taken across a page.
Illegal opcodes are marked with `*`.

|  | x0 | x1 | x2 | x3 | x4 | x5 | x6 | x7 | x8 | x9 | xA | xB | xC | xD | xE | xF |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| **0x** | BRK 7 | ORA 6 | JAM* - | SLO* 8 | NOP* 3 | ORA 3 | ASL 5 | SLO* 5 | PHP 3 | ORA 2 | ASL 2 | ANC* 2 | NOP* 4 | ORA 4 | ASL 6 | SLO* 6 |
| **1x** | BPL 2/3/4 | ORA 5+ | JAM* - | SLO* 8 | NOP* 4 | ORA 4 | ASL 6 | SLO* 6 | CLC 2 | ORA 4+ | NOP* 2 | SLO* 7 | NOP* 4+ | ORA 4+ | ASL 7 | SLO* 7 |
| **2x** | JSR 6 | AND 6 | JAM* - | RLA* 8 | BIT 3 | AND 3 | ROL 5 | RLA* 5 | PLP 4 | AND 2 | ROL 2 | ANC* 2 | BIT 4 | AND 4 | ROL 6 | RLA* 6 |
| **3x** | BMI 2/3/4 | AND 5+ | JAM* - | RLA* 8 | NOP* 4 | AND 4 | ROL 6 | RLA* 6 | SEC 2 | AND 4+ | NOP* 2 | RLA* 7 | NOP* 4+ | AND 4+ | ROL 7 | RLA* 7 |
| **4x** | RTI 6 | EOR 6 | JAM* - | SRE* 8 | NOP* 3 | EOR 3 | LSR 5 | SRE* 5 | PHA 3 | EOR 2 | LSR 2 | ALR* 2 | JMP 3 | EOR 4 | LSR 6 | SRE* 6 |
| **5x** | BVC 2/3/4 | EOR 5+ | JAM* - | SRE* 8 | NOP* 4 | EOR 4 | LSR 6 | SRE* 6 | CLI 2 | EOR 4+ | NOP* 2 | SRE* 7 | NOP* 4+ | EOR 4+ | LSR 7 | SRE* 7 |
| **6x** | RTS 6 | ADC 6 | JAM* - | RRA* 8 | NOP* 3 | ADC 3 | ROR 5 | RRA* 5 | PLA 4 | ADC 2 | ROR 2 | ARR* 2 | JMP 5 | ADC 4 | ROR 6 | RRA* 6 |
| **7x** | BVS 2/3/4 | ADC 5+ | JAM* - | RRA* 8 | NOP* 4 | ADC 4 | ROR 6 | RRA* 6 | SEI 2 | ADC 4+ | NOP* 2 | RRA* 7 | NOP* 4+ | ADC 4+ | ROR 7 | RRA* 7 |
| **8x** | NOP* 2 | STA 6 | NOP* 2 | SAX* 6 | STY 3 | STA 3 | STX 3 | SAX* 3 | DEY 2 | NOP* 2 | TXA 2 | ANE* 2 | STY 4 | STA 4 | STX 4 | SAX* 4 |
| **9x** | BCC 2/3/4 | STA 6 | JAM* - | SHA* 6 | STY 4 | STA 4 | STX 4 | SAX* 4 | TYA 2 | STA 5 | TXS 2 | SHS* 5 | SHY* 5 | STA 5 | SHX* 5 | SHA* 5 |
| **Ax** | LDY 2 | LDA 6 | LDX 2 | LAX* 6 | LDY 3 | LDA 3 | LDX 3 | LAX* 3 | TAY 2 | LDA 2 | TAX 2 | LXA* 2 | LDY 4 | LDA 4 | LDX 4 | LAX* 4 |
| **Bx** | BCS 2/3/4 | LDA 5+ | JAM* - | LAX* 5+ | LDY 4 | LDA 4 | LDX 4 | LAX* 4 | CLV 2 | LDA 4+ | TSX 2 | LAE* 4+ | LDY 4+ | LDA 4+ | LDX 4+ | LAX* 4+ |
| **Cx** | CPY 2 | CMP 6 | NOP* 2 | DCP* 8 | CPY 3 | CMP 3 | DEC 5 | DCP* 5 | INY 2 | CMP 2 | DEX 2 | SBX* 2 | CPY 4 | CMP 4 | DEC 6 | DCP* 6 |
| **Dx** | BNE 2/3/4 | CMP 5+ | JAM* - | DCP* 8 | NOP* 4 | CMP 4 | DEC 6 | DCP* 6 | CLD 2 | CMP 4+ | NOP* 2 | DCP* 7 | NOP* 4+ | CMP 4+ | DEC 7 | DCP* 7 |
| **Ex** | CPX 2 | SBC 6 | NOP* 2 | ISC* 8 | CPX 3 | SBC 3 | INC 5 | ISC* 5 | INX 2 | SBC 2 | NOP 2 | SBC* 2 | CPX 4 | SBC 4 | INC 6 | ISC* 6 |
| **Fx** | BEQ 2/3/4 | SBC 5+ | JAM* - | ISC* 8 | NOP* 4 | SBC 4 | INC 6 | ISC* 6 | SED 2 | SBC 4+ | NOP* 2 | ISC* 7 | NOP* 4+ | SBC 4+ | INC 7 | ISC* 7 |

## Documented instructions

| Mnemonic | Mode | Opcode | Bytes | Cycles |
|---|---|---|---|---|
| ADC | ($nn,X) | `$61` | 2 | 6 |
| ADC | $nn | `$65` | 2 | 3 |
| ADC | #$nn | `$69` | 2 | 2 |
| ADC | $nnnn | `$6D` | 3 | 4 |
| ADC | ($nn),Y | `$71` | 2 | 5+ |
| ADC | $nn,X | `$75` | 2 | 4 |
| ADC | $nnnn,Y | `$79` | 3 | 4+ |
| ADC | $nnnn,X | `$7D` | 3 | 4+ |
| AND | ($nn,X) | `$21` | 2 | 6 |
| AND | $nn | `$25` | 2 | 3 |
| AND | #$nn | `$29` | 2 | 2 |
| AND | $nnnn | `$2D` | 3 | 4 |
| AND | ($nn),Y | `$31` | 2 | 5+ |
| AND | $nn,X | `$35` | 2 | 4 |
| AND | $nnnn,Y | `$39` | 3 | 4+ |
| AND | $nnnn,X | `$3D` | 3 | 4+ |
| ASL | $nn | `$06` | 2 | 5 |
| ASL | A | `$0A` | 1 | 2 |
| ASL | $nnnn | `$0E` | 3 | 6 |
| ASL | $nn,X | `$16` | 2 | 6 |
| ASL | $nnnn,X | `$1E` | 3 | 7 |
| BCC | $nnnn | `$90` | 2 | 2/3/4 |
| BCS | $nnnn | `$B0` | 2 | 2/3/4 |
| BEQ | $nnnn | `$F0` | 2 | 2/3/4 |
| BIT | $nn | `$24` | 2 | 3 |
| BIT | $nnnn | `$2C` | 3 | 4 |
| BMI | $nnnn | `$30` | 2 | 2/3/4 |
| BNE | $nnnn | `$D0` | 2 | 2/3/4 |
| BPL | $nnnn | `$10` | 2 | 2/3/4 |
| BRK | implied | `$00` | 2 | 7 |
| BVC | $nnnn | `$50` | 2 | 2/3/4 |
| BVS | $nnnn | `$70` | 2 | 2/3/4 |
| CLC | implied | `$18` | 1 | 2 |
| CLD | implied | `$D8` | 1 | 2 |
| CLI | implied | `$58` | 1 | 2 |
| CLV | implied | `$B8` | 1 | 2 |
| CMP | ($nn,X) | `$C1` | 2 | 6 |
| CMP | $nn | `$C5` | 2 | 3 |
| CMP | #$nn | `$C9` | 2 | 2 |
| CMP | $nnnn | `$CD` | 3 | 4 |
| CMP | ($nn),Y | `$D1` | 2 | 5+ |
| CMP | $nn,X | `$D5` | 2 | 4 |
| CMP | $nnnn,Y | `$D9` | 3 | 4+ |
| CMP | $nnnn,X | `$DD` | 3 | 4+ |
| CPX | #$nn | `$E0` | 2 | 2 |
| CPX | $nn | `$E4` | 2 | 3 |
| CPX | $nnnn | `$EC` | 3 | 4 |
| CPY | #$nn | `$C0` | 2 | 2 |
| CPY | $nn | `$C4` | 2 | 3 |
| CPY | $nnnn | `$CC` | 3 | 4 |
| DEC | $nn | `$C6` | 2 | 5 |
| DEC | $nnnn | `$CE` | 3 | 6 |
| DEC | $nn,X | `$D6` | 2 | 6 |
| DEC | $nnnn,X | `$DE` | 3 | 7 |
| DEX | implied | `$CA` | 1 | 2 |
| DEY | implied | `$88` | 1 | 2 |
| EOR | ($nn,X) | `$41` | 2 | 6 |
| EOR | $nn | `$45` | 2 | 3 |
| EOR | #$nn | `$49` | 2 | 2 |
| EOR | $nnnn | `$4D` | 3 | 4 |
| EOR | ($nn),Y | `$51` | 2 | 5+ |
| EOR | $nn,X | `$55` | 2 | 4 |
| EOR | $nnnn,Y | `$59` | 3 | 4+ |
| EOR | $nnnn,X | `$5D` | 3 | 4+ |
| INC | $nn | `$E6` | 2 | 5 |
| INC | $nnnn | `$EE` | 3 | 6 |
| INC | $nn,X | `$F6` | 2 | 6 |
| INC | $nnnn,X | `$FE` | 3 | 7 |
| INX | implied | `$E8` | 1 | 2 |
| INY | implied | `$C8` | 1 | 2 |
| JMP | $nnnn | `$4C` | 3 | 3 |
| JMP | ($nnnn) | `$6C` | 3 | 5 |
| JSR | $nnnn | `$20` | 3 | 6 |
| LDA | ($nn,X) | `$A1` | 2 | 6 |
| LDA | $nn | `$A5` | 2 | 3 |
| LDA | #$nn | `$A9` | 2 | 2 |
| LDA | $nnnn | `$AD` | 3 | 4 |
| LDA | ($nn),Y | `$B1` | 2 | 5+ |
| LDA | $nn,X | `$B5` | 2 | 4 |
| LDA | $nnnn,Y | `$B9` | 3 | 4+ |
| LDA | $nnnn,X | `$BD` | 3 | 4+ |
| LDX | #$nn | `$A2` | 2 | 2 |
| LDX | $nn | `$A6` | 2 | 3 |
| LDX | $nnnn | `$AE` | 3 | 4 |
| LDX | $nn,Y | `$B6` | 2 | 4 |
| LDX | $nnnn,Y | `$BE` | 3 | 4+ |
| LDY | #$nn | `$A0` | 2 | 2 |
| LDY | $nn | `$A4` | 2 | 3 |
| LDY | $nnnn | `$AC` | 3 | 4 |
| LDY | $nn,X | `$B4` | 2 | 4 |
| LDY | $nnnn,X | `$BC` | 3 | 4+ |
| LSR | $nn | `$46` | 2 | 5 |
| LSR | A | `$4A` | 1 | 2 |
| LSR | $nnnn | `$4E` | 3 | 6 |
| LSR | $nn,X | `$56` | 2 | 6 |
| LSR | $nnnn,X | `$5E` | 3 | 7 |
| ORA | ($nn,X) | `$01` | 2 | 6 |
| ORA | $nn | `$05` | 2 | 3 |
| ORA | #$nn | `$09` | 2 | 2 |
| ORA | $nnnn | `$0D` | 3 | 4 |
| ORA | ($nn),Y | `$11` | 2 | 5+ |
| ORA | $nn,X | `$15` | 2 | 4 |
| ORA | $nnnn,Y | `$19` | 3 | 4+ |
| ORA | $nnnn,X | `$1D` | 3 | 4+ |
| PHA | implied | `$48` | 1 | 3 |
| PHP | implied | `$08` | 1 | 3 |
| PLA | implied | `$68` | 1 | 4 |
| PLP | implied | `$28` | 1 | 4 |
| ROL | $nn | `$26` | 2 | 5 |
| ROL | A | `$2A` | 1 | 2 |
| ROL | $nnnn | `$2E` | 3 | 6 |
| ROL | $nn,X | `$36` | 2 | 6 |
| ROL | $nnnn,X | `$3E` | 3 | 7 |
| ROR | $nn | `$66` | 2 | 5 |
| ROR | A | `$6A` | 1 | 2 |
| ROR | $nnnn | `$6E` | 3 | 6 |
| ROR | $nn,X | `$76` | 2 | 6 |
| ROR | $nnnn,X | `$7E` | 3 | 7 |
| RTI | implied | `$40` | 1 | 6 |
| RTS | implied | `$60` | 1 | 6 |
| SBC | ($nn,X) | `$E1` | 2 | 6 |
| SBC | $nn | `$E5` | 2 | 3 |
| SBC | #$nn | `$E9` | 2 | 2 |
| SBC | #$nn | `$EB` | 2 | 2 |
| SBC | $nnnn | `$ED` | 3 | 4 |
| SBC | ($nn),Y | `$F1` | 2 | 5+ |
| SBC | $nn,X | `$F5` | 2 | 4 |
| SBC | $nnnn,Y | `$F9` | 3 | 4+ |
| SBC | $nnnn,X | `$FD` | 3 | 4+ |
| SEC | implied | `$38` | 1 | 2 |
| SED | implied | `$F8` | 1 | 2 |
| SEI | implied | `$78` | 1 | 2 |
| STA | ($nn,X) | `$81` | 2 | 6 |
| STA | $nn | `$85` | 2 | 3 |
| STA | $nnnn | `$8D` | 3 | 4 |
| STA | ($nn),Y | `$91` | 2 | 6 |
| STA | $nn,X | `$95` | 2 | 4 |
| STA | $nnnn,Y | `$99` | 3 | 5 |
| STA | $nnnn,X | `$9D` | 3 | 5 |
| STX | $nn | `$86` | 2 | 3 |
| STX | $nnnn | `$8E` | 3 | 4 |
| STX | $nn,Y | `$96` | 2 | 4 |
| STY | $nn | `$84` | 2 | 3 |
| STY | $nnnn | `$8C` | 3 | 4 |
| STY | $nn,X | `$94` | 2 | 4 |
| TAX | implied | `$AA` | 1 | 2 |
| TAY | implied | `$A8` | 1 | 2 |
| TSX | implied | `$BA` | 1 | 2 |
| TXA | implied | `$8A` | 1 | 2 |
| TXS | implied | `$9A` | 1 | 2 |
| TYA | implied | `$98` | 1 | 2 |

## Undocumented instructions

| Mnemonic | Mode | Opcode | Bytes | Cycles |
|---|---|---|---|---|
| ALR | #$nn | `$4B` | 2 | 2 |
| ANC | #$nn | `$0B` | 2 | 2 |
| ANC | #$nn | `$2B` | 2 | 2 |
| ANE | #$nn | `$8B` | 2 | 2 |
| ARR | #$nn | `$6B` | 2 | 2 |
| DCP | ($nn,X) | `$C3` | 2 | 8 |
| DCP | $nn | `$C7` | 2 | 5 |
| DCP | $nnnn | `$CF` | 3 | 6 |
| DCP | ($nn),Y | `$D3` | 2 | 8 |
| DCP | $nn,X | `$D7` | 2 | 6 |
| DCP | $nnnn,Y | `$DB` | 3 | 7 |
| DCP | $nnnn,X | `$DF` | 3 | 7 |
| ISC | ($nn,X) | `$E3` | 2 | 8 |
| ISC | $nn | `$E7` | 2 | 5 |
| ISC | $nnnn | `$EF` | 3 | 6 |
| ISC | ($nn),Y | `$F3` | 2 | 8 |
| ISC | $nn,X | `$F7` | 2 | 6 |
| ISC | $nnnn,Y | `$FB` | 3 | 7 |
| ISC | $nnnn,X | `$FF` | 3 | 7 |
| JAM | implied | `$02` | 1 | - |
| JAM | implied | `$12` | 1 | - |
| JAM | implied | `$22` | 1 | - |
| JAM | implied | `$32` | 1 | - |
| JAM | implied | `$42` | 1 | - |
| JAM | implied | `$52` | 1 | - |
| JAM | implied | `$62` | 1 | - |
| JAM | implied | `$72` | 1 | - |
| JAM | implied | `$92` | 1 | - |
| JAM | implied | `$B2` | 1 | - |
| JAM | implied | `$D2` | 1 | - |
| JAM | implied | `$F2` | 1 | - |
| LAE | $nnnn,Y | `$BB` | 3 | 4+ |
| LAX | ($nn,X) | `$A3` | 2 | 6 |
| LAX | $nn | `$A7` | 2 | 3 |
| LAX | $nnnn | `$AF` | 3 | 4 |
| LAX | ($nn),Y | `$B3` | 2 | 5+ |
| LAX | $nn,Y | `$B7` | 2 | 4 |
| LAX | $nnnn,Y | `$BF` | 3 | 4+ |
| LXA | #$nn | `$AB` | 2 | 2 |
| NOP | $nn | `$04` | 2 | 3 |
| NOP | $nnnn | `$0C` | 3 | 4 |
| NOP | $nn,X | `$14` | 2 | 4 |
| NOP | implied | `$1A` | 1 | 2 |
| NOP | $nnnn,X | `$1C` | 3 | 4+ |
| NOP | $nn,X | `$34` | 2 | 4 |
| NOP | implied | `$3A` | 1 | 2 |
| NOP | $nnnn,X | `$3C` | 3 | 4+ |
| NOP | $nn | `$44` | 2 | 3 |
| NOP | $nn,X | `$54` | 2 | 4 |
| NOP | implied | `$5A` | 1 | 2 |
| NOP | $nnnn,X | `$5C` | 3 | 4+ |
| NOP | $nn | `$64` | 2 | 3 |
| NOP | $nn,X | `$74` | 2 | 4 |
| NOP | implied | `$7A` | 1 | 2 |
| NOP | $nnnn,X | `$7C` | 3 | 4+ |
| NOP | #$nn | `$80` | 2 | 2 |
| NOP | #$nn | `$82` | 2 | 2 |
| NOP | #$nn | `$89` | 2 | 2 |
| NOP | #$nn | `$C2` | 2 | 2 |
| NOP | $nn,X | `$D4` | 2 | 4 |
| NOP | implied | `$DA` | 1 | 2 |
| NOP | $nnnn,X | `$DC` | 3 | 4+ |
| NOP | #$nn | `$E2` | 2 | 2 |
| NOP | implied | `$EA` | 1 | 2 |
| NOP | $nn,X | `$F4` | 2 | 4 |
| NOP | implied | `$FA` | 1 | 2 |
| NOP | $nnnn,X | `$FC` | 3 | 4+ |
| RLA | ($nn,X) | `$23` | 2 | 8 |
| RLA | $nn | `$27` | 2 | 5 |
| RLA | $nnnn | `$2F` | 3 | 6 |
| RLA | ($nn),Y | `$33` | 2 | 8 |
| RLA | $nn,X | `$37` | 2 | 6 |
| RLA | $nnnn,Y | `$3B` | 3 | 7 |
| RLA | $nnnn,X | `$3F` | 3 | 7 |
| RRA | ($nn,X) | `$63` | 2 | 8 |
| RRA | $nn | `$67` | 2 | 5 |
| RRA | $nnnn | `$6F` | 3 | 6 |
| RRA | ($nn),Y | `$73` | 2 | 8 |
| RRA | $nn,X | `$77` | 2 | 6 |
| RRA | $nnnn,Y | `$7B` | 3 | 7 |
| RRA | $nnnn,X | `$7F` | 3 | 7 |
| SAX | ($nn,X) | `$83` | 2 | 6 |
| SAX | $nn | `$87` | 2 | 3 |
| SAX | $nnnn | `$8F` | 3 | 4 |
| SAX | $nn,Y | `$97` | 2 | 4 |
| SBX | #$nn | `$CB` | 2 | 2 |
| SHA | ($nn),Y | `$93` | 2 | 6 |
| SHA | $nnnn,Y | `$9F` | 3 | 5 |
| SHS | $nnnn,Y | `$9B` | 3 | 5 |
| SHX | $nnnn,Y | `$9E` | 3 | 5 |
| SHY | $nnnn,X | `$9C` | 3 | 5 |
| SLO | ($nn,X) | `$03` | 2 | 8 |
| SLO | $nn | `$07` | 2 | 5 |
| SLO | $nnnn | `$0F` | 3 | 6 |
| SLO | ($nn),Y | `$13` | 2 | 8 |
| SLO | $nn,X | `$17` | 2 | 6 |
| SLO | $nnnn,Y | `$1B` | 3 | 7 |
| SLO | $nnnn,X | `$1F` | 3 | 7 |
| SRE | ($nn,X) | `$43` | 2 | 8 |
| SRE | $nn | `$47` | 2 | 5 |
| SRE | $nnnn | `$4F` | 3 | 6 |
| SRE | ($nn),Y | `$53` | 2 | 8 |
| SRE | $nn,X | `$57` | 2 | 6 |
| SRE | $nnnn,Y | `$5B` | 3 | 7 |
| SRE | $nnnn,X | `$5F` | 3 | 7 |

## Undocumented opcodes on real hardware

The 6510 in a C64 executes these reliably, and demos use them for the cycles they save - `LAX`
loads A and X in one instruction, `SAX` stores `A AND X`. Two caveats: `ANE` and `LXA` depend on
analogue behaviour and vary between chips and temperatures, and `SHA`, `SHX`, `SHY` and `SHS`
behave unpredictably when the indexed address crosses a page. `JAM` hangs the CPU until reset.

Kick Assembler accepts them by default; `-excludeillegal` turns them off.
