# Verify and package the skills in this directory.
#
#   make check   assemble and verify every skill that has its own Makefile
#   make dist    build c64u-skills-<version>.tar.gz and .zip into dist/
#   make clean   remove dist/ and each skill's build output
#
# The archives contain the skill directories at their root, so extracting one
# into any harness skill directory produces the expected layout:
#
#   tar xzf c64u-skills-v1.0.0.tar.gz -C .agents/skills/
#   -> .agents/skills/c64-knowledge/SKILL.md

# The archive name carries no version, matching the binary assets and letting
# .../releases/latest/download/c64u-skills.tar.gz resolve. Nothing but the skill
# directories goes into the archive, so extracting it into a skills directory
# cannot collide with an unrelated project's files.
DIST := dist
NAME := c64u-skills

# Every directory holding a SKILL.md is a skill.
SKILLS := $(patsubst %/SKILL.md,%,$(wildcard */SKILL.md))

# Skills that ship verifiable content carry their own Makefile.
BUILDABLE := $(foreach s,$(SKILLS),$(if $(wildcard $(s)/Makefile),$(s)))

.PHONY: check dist clean

check:
	@for s in $(BUILDABLE); do \
		echo "== $$s"; \
		$(MAKE) --no-print-directory -C $$s check || exit 1; \
	done
	@echo "checked $(words $(BUILDABLE)) of $(words $(SKILLS)) skills"

# COPYFILE_DISABLE keeps macOS from adding ._ resource-fork entries.
dist: check
	@mkdir -p $(DIST)
	@COPYFILE_DISABLE=1 tar czf $(DIST)/$(NAME).tar.gz \
		--exclude build --exclude .gitignore --exclude .DS_Store $(SKILLS)
	@rm -f $(DIST)/$(NAME).zip
	@zip -q -r $(DIST)/$(NAME).zip $(SKILLS) \
		-x '*/build/*' '*/.gitignore' '*.DS_Store'
	@echo "$(DIST)/$(NAME).tar.gz"
	@echo "$(DIST)/$(NAME).zip"
	@# One zip per skill as well. ChatGPT takes a single skill as a zip upload,
	@# and a bundle holding several skills is ambiguous there.
	@for s in $(SKILLS); do \
		rm -f $(DIST)/$$s.zip; \
		zip -q -r $(DIST)/$$s.zip $$s \
			-x '*/build/*' '*/.gitignore' '*.DS_Store'; \
		echo "$(DIST)/$$s.zip"; \
	done

clean:
	@rm -rf $(DIST)
	@for s in $(BUILDABLE); do $(MAKE) --no-print-directory -C $$s clean; done
