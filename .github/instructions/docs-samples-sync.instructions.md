---
description: "Use when making code changes that alter behavior, interfaces, flags, configuration, manifests, or workflows. Requires synchronized updates to docs, sample code/manifests, and cross-references."
name: "Docs Samples Sync"
applyTo:
  - "SQLK8sOperator/**"
  - "SQLAICustomContainer/**"
---
# Documentation and Samples Synchronization

When a code change modifies behavior, settings, contracts, examples, or operational steps, update documentation and samples in the same change set.

## Required Actions for Code Changes

- Update user-facing docs that describe the changed behavior.
- Update sample code/manifests/scripts that users are expected to copy.
- Update cross-references and links pointing to changed docs or samples.
- Keep command examples and expected outputs consistent with current implementation.
- Include a short devil's-advocate critique in the response that covers potential benefits, downsides, and edge-case impacts of the change.

## Verification Checklist

- No stale flags, field names, or paths remain in docs.
- No sample references point to removed or renamed files.
- Root README and subproject docs are internally consistent.
- Docs and samples match defaults and validation rules in code.

## Scope Notes

- SQLK8sOperator: ensure consistency across `README.md`, `docs/`, `samples/`, and CRD/manifests references.
- SQLAICustomContainer: ensure consistency across `README.md`, `build-and-run.sh --help`, `Dockerfile` behavior notes, and `tests/*.md` command examples.
