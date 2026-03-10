---
description: "Use when working in SQLAICustomContainer for Dockerfile, build-and-run.sh, test scripts, and component toggles (Ollama, MinIO, Polybase, Caddy). Enforces plan-first edits and cross-file consistency."
name: "SQLAICustomContainer Instructions"
applyTo: "SQLAICustomContainer/**"
---
# SQLAICustomContainer Guidelines

- Start with a short implementation plan before any edits.
- Keep changes synchronized across `Dockerfile`, `build-and-run.sh`, and `README.md` when options or behavior change.
- If runtime flags, ports, volumes, or defaults change, update docs in `README.md`, test docs in `tests/*.md`, and sample command snippets in the same change set.
- Include a concise devil's-advocate critique in responses for change requests, covering expected gains, potential regressions, and operational risks.
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
