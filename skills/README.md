# Skills

Knowledge packs for C64 development, written to the [Agent Skills][spec] open format. They are
plain Markdown and plain assembler source with no harness-specific syntax, so the same files work
in Claude Code, opencode, pi, Hermes, Codex CLI, Cursor and anything else that reads the format.

[spec]: https://agentskills.io/

## Layout

```
skills/
  README.md                    this file
  Makefile                     make check / make dist
  c64-knowledge/
    SKILL.md                   entry point: frontmatter + routing table
    references/*.md            one self-contained quickref per topic
    examples/*.asm             runnable source, built rather than read
    Makefile                   make check - assembles every example
```

`SKILL.md` starts with YAML frontmatter carrying two required fields:

```yaml
---
name: c64-knowledge
description: <what it covers and when to reach for it>
---
```

**The `name` must match the directory name exactly.** Lowercase letters, digits and hyphens
only, 64 characters maximum, no leading or trailing hyphen. A mismatch makes most harnesses skip
the skill silently. Unknown frontmatter fields are ignored, so tool-specific extras are harmless
but also pointless here.

## Why the content is split across files

Each layer loads only when the one above decided it was relevant:

| Layer | Size | When it enters context |
|---|---|---|
| `description` | ~60 tokens | always - it is what the agent matches against |
| `SKILL.md` body | ~1k tokens | once the skill is judged relevant |
| one `references/*.md` | ~1.5-2k tokens | when that specific topic comes up |
| `examples/*.asm` | - | only when code is needed, and preferably assembled, not read |

Reading all of `c64-knowledge` costs roughly 12k tokens. The intended path costs about 2k. A
harness that simply concatenates everything still works, it just gives up the saving.

## Making the skills available to a harness

### This repository, already wired

Two tracked symlinks point at the canonical `skills/` directory:

```
.agents/skills/c64-knowledge -> ../../skills/c64-knowledge
.claude/skills/c64-knowledge -> ../../skills/c64-knowledge
```

`.agents/skills/` is the vendor-neutral path from the specification, read by Codex CLI, opencode
and pi. `.claude/skills/` is what Claude Code scans. Cloning this repository is therefore enough
for all four - no setup.

`.claude/` also holds per-user settings, so the repository ignores it except for the skills
subtree:

```gitignore
.claude/*
!.claude/skills/
```

### Where each tool looks

| Harness | Project-level | User-level |
|---|---|---|
| Claude Code | `.claude/skills/<name>/SKILL.md` | `~/.claude/skills/<name>/SKILL.md` |
| [Codex CLI][codex] | `.agents/skills/` in the cwd, its parent, and the repo root | `$HOME/.agents/skills/` |
| [opencode][oc] | `.opencode/skills/`, `.claude/skills/`, `.agents/skills/` | `~/.config/opencode/skills/`, `~/.claude/skills/`, `~/.agents/skills/` |
| [pi][pi] | `.pi/skills/`, `.agents/skills/` (cwd and ancestors up to the repo root) | `~/.pi/agent/skills/`, `~/.agents/skills/` |
| [Hermes][hermes] | via `skills.external_dirs` in `config.yaml` | `~/.hermes/skills/` |
| [ChatGPT][gpt] | no directory - upload a zip in Settings | same |

[codex]: https://developers.openai.com/codex/skills
[oc]: https://opencode.ai/docs/skills/
[pi]: https://pi.dev/docs/latest/skills
[hermes]: https://hermes-agent.nousresearch.com/docs/developer-guide/creating-skills
[gpt]: https://help.openai.com/en/articles/20001066-skills-in-chatgpt

Claude Code, Codex CLI, opencode and pi therefore pick these skills up from a plain `git clone`
with no further setup - all four read one of the two symlinked paths above.

Hermes keeps a single source of truth under `~/.hermes/skills/` and needs either an entry in
`skills.external_dirs` or an installed copy.

pi additionally accepts a path directly, which is the quickest way to try a skill without
installing it:

```sh
pi --skill /path/to/c64u/skills/c64-knowledge
```

Codex exposes discovered skills through `/skills`, and they can be invoked explicitly with a
`$` prefix - useful for checking that discovery actually worked.

### Installing into a user-level directory

Pick the directory your harness reads from the table above, then use one of these.

**From a release archive** - no clone, no toolchain:

```sh
# tar.gz
curl -L -o skills.tar.gz \
  https://github.com/cybersorcerer/c64u/releases/latest/download/c64u-skills.tar.gz
mkdir -p ~/.agents/skills
tar xzf skills.tar.gz -C ~/.agents/skills

# zip
unzip c64u-skills.zip -d ~/.agents/skills
```

The archives hold the skill directories at their root, so extraction lands exactly one level
deep: `~/.agents/skills/c64-knowledge/SKILL.md`. Every tagged release carries both, listed in
`checksums.txt` next to the binaries.

**Into ChatGPT** - Skills are uploaded as a zip through Settings rather than placed in a
directory, and the upload is scanned before the skill becomes usable. Custom skill uploads are
limited to the paid plans. Use the single-skill archive:

```
c64-knowledge.zip
```

Every release carries one zip per skill next to the bundle, because a bundle holding several
skills is ambiguous for a per-skill upload. Build them locally with `make -C skills dist`.

**From a clone, by symlink** - stays current with `git pull`:

```sh
git clone https://github.com/cybersorcerer/c64u.git
mkdir -p ~/.agents/skills
ln -s "$PWD/c64u/skills/c64-knowledge" ~/.agents/skills/c64-knowledge
```

**For Hermes**, add the clone to `~/.hermes/config.yaml` instead of copying:

```yaml
skills:
  external_dirs:
    - /path/to/c64u/skills
```

### For a harness not listed here

The format is the portable part; discovery is not. Find the directory your tool scans and place
a copy or a symlink of `c64-knowledge/` in it. If it has no skill mechanism at all, point its
project instruction file at `skills/c64-knowledge/SKILL.md` and let the agent open the reference
files itself - that is what `AGENTS.md` in the repository root does.

A note on symlinks: git tracks them and they work on macOS and Linux. On Windows they need
developer mode or `git config core.symlinks true`; otherwise copy the directory.

## Verifying and packaging

```sh
make -C skills check                   # verify every skill
make -C skills dist                    # build the release archives
make -C skills/c64-knowledge check-hw  # run the examples on a real device
```

`check` runs each skill's own Makefile: `c64-knowledge` assembles every example, `c64u-cli`
walks the CLI's command tree and confirms that every command its reference documents still
exists. `dist` runs `check` first, so a broken skill fails the packaging rather than shipping.

`check-hw` goes further and proves the reference values themselves. Assembling shows an example
is valid 6502; only running it on hardware and reading back what it changed shows that the
addresses and bit patterns in the reference files are right. It is skipped, not failed, when no
device answers.

Documentation that states a register value or a byte encoding should be verified against real
behaviour before being written down. The `c64-knowledge` references were checked against a
physical C64 Ultimate: a wrong constant in a reference file is worse than no reference file,
because it gets trusted.

## Available skills

| Skill | Covers |
|---|---|
| `c64-knowledge` | Memory map and banking, VIC-II, CIA, SID, PETSCII and screen codes, 6502 opcodes and cycles, graphics pipeline, disk formats, BASIC pitfalls, Kick Assembler, REU programming |
| `c64u-cli` | Driving real hardware with the `c64u` CLI: commands, verified workflows, and the limits of DMA and keystroke injection |

`c64-knowledge/references/opcodes.md` is generated from the disassembler's opcode table in
`tools/c64u`, so the documentation cannot drift from the code that decodes those bytes:

```sh
make -C skills/c64-knowledge opcodes
```
