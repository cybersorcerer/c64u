# Build the wedge cartridge.
#
#   make          assemble wedge.asm and wrap it in build/wedge.crt
#   make run      build, then upload and start it on the connected device
#   make install  build, then copy it into /flash/carts on the device
#   make clean
#
# Override the assembler if it lives elsewhere:
#   make KICKASS=/path/to/KickAss.jar

KICKASS ?= $(HOME)/.local/bin/KickAssembler/KickAss.jar
JAVA    ?= java
C64U    ?= $(CURDIR)/../../tools/c64u/build/c64u
GOMOD   := $(CURDIR)/../../tools/c64u

BUILD := build
NAME  := ULTIMATE WEDGE

.PHONY: all run install clean assembler

all: $(BUILD)/wedge.crt

assembler:
	@test -f "$(KICKASS)" || { \
		echo "Kick Assembler not found at $(KICKASS)"; \
		echo "Pass KICKASS=/path/to/KickAss.jar"; \
		exit 1; }

# -binfile drops the two-byte load address, leaving a raw 8192 byte ROM.
$(BUILD)/wedge.bin: wedge.asm | assembler $(BUILD)
	@$(JAVA) -jar $(KICKASS) $< -binfile -o $(CURDIR)/$@ >/dev/null || \
		{ echo "FAIL: $<"; $(JAVA) -jar $(KICKASS) $< -binfile -o $(CURDIR)/$@; exit 1; }
	@test $$(wc -c < $@) -eq 8192 || { echo "$@ is not 8192 bytes"; exit 1; }
	@echo "  ok  $@"

$(BUILD)/wedge.crt: $(BUILD)/wedge.bin
	@cd $(GOMOD) && go run ./internal/crt/mkcrt \
		-in $(CURDIR)/$< -out $(CURDIR)/$@ -name "$(NAME)" -type 19

$(BUILD):
	@mkdir -p $(BUILD)

# Temporary: runs until the next reboot, without touching the flash.
run: all
	$(C64U) runners run-crt-upload $(BUILD)/wedge.crt

# Permanent: /flash/carts is hardcoded on the device. Select it afterwards with
#   c64u config set "C64 and Cartridge Settings" Cartridge wedge.crt
#   c64u config save-to-flash
install: all
	$(C64U) fs upload $(BUILD)/wedge.crt /Flash/carts/wedge.crt

clean:
	@rm -rf $(BUILD)
