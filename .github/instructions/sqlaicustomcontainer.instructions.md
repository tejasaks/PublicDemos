---
description: "Use when working in SQLAICustomContainer for Dockerfile, build-and-run.sh, test scripts, and component toggles (Ollama, MinIO, Polybase, Caddy). Enforces plan-first edits and cross-file consistency."
name: "SQLAICustomContainer Instructions"
applyTo: "SQLAICustomContainer/**"
---
# SQLAICustomContainer Guidelines

- Start with a short implementation plan before any edits.
- Keep changes synchronized across `Dockerfile`, `build-and-run.sh`, and `README.md` when options or behavior change.
- If runtime flags, ports, volumes, or defaults change, update docs in `README.md`, test docs in `tests/*.md`, and sample command snippets in the same change set.
- For architectural or significant structural changes, also update `DESIGN-EVOLUTION.md`, the Design Evolution section in `.github/instructions/sqlaicustomcontainer.instructions.md`, and the Design Evolution Summary in `.github/copilot-instructions.md`.
- Include a concise devil's-advocate critique in responses for change requests where it adds value, covering expected gains, potential regressions, and operational risks. Skip the critique for straightforward tasks with nothing meaningful to critique.
- For architectural or strategic questions, include a brief comparison with competitive container solutions or industry-standard approaches, noting alignment or novel divergence.
- Preserve optional-service gating behavior for Ollama, MinIO, and Polybase.
- Be explicit about Ubuntu vs RHEL paths/package logic when modifying install or startup flow.
- Keep shell script behavior predictable and non-interactive for test automation.

## Documentation and Samples Sync Checklist

- Keep command examples runnable and aligned with current script options.
- Update all references to changed flags or files across root and tests docs.
- When adding/removing options, update usage/help text and every documented invocation pattern.

## Validation Commands

Run from `SQLAICustomContainer/` and choose the nearest relevant checks:

- `./build-and-run.sh --help`
- `./build-and-run.sh --sa-password 'YourStrong@Pass123' --install-ollama false --no-follow`
- `cd tests && ./test-prerequisites.sh`
- `cd tests && ./test-deployments.sh`
- `cd tests && ./run-all-tests.sh`

## Key File Anchors

- Build/runtime script: `build-and-run.sh`
- Image build and startup flow: `Dockerfile`
- Usage and options docs: `README.md`
- Test documentation and examples: `tests/*.md`
- Test harness and scenarios: `tests/`
- Design evolution context: `DESIGN-EVOLUTION.md`

## Design Evolution (Quick Reference)

The container project evolved through these phases (see `DESIGN-EVOLUTION.md` for detail):

1. Single-purpose container — SQL Server + Ollama + Caddy as a fixed bundle
2. Optional components — MinIO and Polybase made toggleable; Ollama made optional
3. Dual-OS support — Single Dockerfile with Ubuntu/RHEL runtime detection
4. Test automation — Prerequisite checks, deployment scenario matrix, cleanup scripts
5. Documentation maturity — Blog post, structured test docs (CHECKLIST, QUICKSTART, INDEX)

When making changes, verify which phase introduced the feature you're modifying to avoid regressing prior design decisions.
