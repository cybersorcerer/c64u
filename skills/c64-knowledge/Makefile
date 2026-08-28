# Assemble the example programs to prove they still build.
#
#   make check     assemble every example, fail on the first error
#   make all       assemble into build/ and keep the .prg files
#   make clean     remove build/
#
# Override the assembler location if it lives elsewhere:
#   make check KICKASS=/path/to/KickAss.jar

KICKASS ?= $(HOME)/.local/bin/KickAssembler/KickAss.jar
JAVA    ?= java

EXAMPLES := $(wildcard examples/*.asm)
BUILD    := build
PRGS     := $(patsubst examples/%.asm,$(BUILD)/%.prg,$(EXAMPLES))

.PHONY: check check-hw all clean assembler opcodes

# Assembling proves the examples are valid 6502. This proves the addresses and
# bit values in the reference files are right, by running them on a real device
# and reading back what they changed. Skipped when no device answers.
check-hw: all
	@sh check-hw.sh

# references/opcodes.md is generated from the disassembler's opcode table so the
# two cannot disagree. Run this after changing that table.
opcodes:
	@cd $(CURDIR)/../../tools/c64u && \
		go run ./internal/disasm/gendoc -o $(CURDIR)/references/opcodes.md

check: all
	@echo "OK: $(words $(EXAMPLES)) examples assembled"

# Fail with a usable message instead of a stack trace from java.
assembler:
	@test -f "$(KICKASS)" || { \
		echo "Kick Assembler not found at $(KICKASS)"; \
		echo "Download it from https://theweb.dk/KickAssembler/ and either put"; \
		echo "KickAss.jar there or pass KICKASS=/path/to/KickAss.jar"; \
		exit 1; }
	@command -v $(JAVA) >/dev/null || { echo "$(JAVA) not on PATH"; exit 1; }

all: assembler $(PRGS)

# Kick Assembler resolves -odir relative to the source file, not to the
# working directory, so the output path has to be absolute.
$(BUILD)/%.prg: examples/%.asm | $(BUILD)
	@$(JAVA) -jar $(KICKASS) $< -odir $(CURDIR)/$(BUILD) >/dev/null || \
		{ echo "FAIL: $<"; $(JAVA) -jar $(KICKASS) $< -odir $(CURDIR)/$(BUILD); exit 1; }
	@echo "  ok  $<"

$(BUILD):
	@mkdir -p $(BUILD)

clean:
	@rm -rf $(BUILD)
