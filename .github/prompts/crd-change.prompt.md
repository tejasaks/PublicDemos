---
description: "Guided checklist for adding or modifying a CRD field in the SQLK8sOperator — walks through types, code-gen, validation, docs, samples, and tests."
---

# CRD Field Change Checklist

I need to add or modify a CRD field in the SQLK8sOperator.

**Field details**: {{input}}

Walk me through the full change set following this checklist. For each step, show me the exact edits needed and confirm before proceeding to the next step.

## Checklist

1. **API types** — Update the Go struct in `SQLK8sOperator/pkg/apis/` with the new/modified field, including JSON tags, kubebuilder markers, and godoc comments.
2. **Code generation** — Run `make manifests generate` to regenerate CRDs under `deploy/crds/` and any deepcopy functions.
3. **Webhook validation** — Update admission webhooks in `internal/validation/` and `internal/webhook/` to validate the new field (required checks, value ranges, immutability if applicable).
4. **Controller logic** — Update reconciliation logic in `internal/controller/` to handle the new field. Ensure idempotent behavior.
5. **Documentation** — Update relevant docs under `docs/` (CRD design, user guide, getting-started) to describe the field's purpose, defaults, and constraints.
6. **Samples** — Update or add sample YAML manifests in `samples/` that exercise the new field.
7. **install.yaml** — Regenerate the consolidated install manifest via `scripts/generate-install-yaml.sh` or `.ps1`.
8. **Tests** — Add or update test fixtures in `tests/fixtures/` and validation tests. Run `make test` and `make lint`.
9. **CHANGELOG** — Add an entry under the Unreleased section of `CHANGELOG.md`.
10. **Design evolution** — If this is a significant structural change, update `docs/development/design-evolution.md` and the Design Evolution Summary in `.github/copilot-instructions.md`.

After completing all steps, run the validation commands to confirm everything passes.
