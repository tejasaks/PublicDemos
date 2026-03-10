---
description: "Guided walkthrough for API version graduation milestones (v1alpha1 → v1beta1 → v1) in the SQLK8sOperator."
---

# Version Graduation Walkthrough

I want to graduate the SQLK8sOperator API version: {{input}}

Walk me through every file that needs updating for this version bump. Show me the exact changes for each step and confirm before proceeding.

## Steps

1. **API types** — Rename or copy the API package under `pkg/apis/` to the new version directory. Update `GroupVersion` in `groupversion_info.go`.
2. **CRD markers** — Update kubebuilder version markers in type structs. Decide whether the old version is still served ( `+kubebuilder:storageversion` placement).
3. **Code generation** — Run `make manifests generate` to produce new CRD YAML files reflecting the version change.
4. **Controller registration** — Update scheme registration and controller `SetupWithManager` calls in `cmd/mssql-operator/main.go` to reference the new API version.
5. **Webhook configuration** — Update webhook paths, conversion webhook if multi-version serving, and TLS configuration references.
6. **install.yaml** — Regenerate via `scripts/generate-install-yaml.sh` or `.ps1`.
7. **Sample manifests** — Update all YAML files in `samples/` to use the new `apiVersion`.
8. **Documentation** — Update `docs/` references (getting-started, CRD design, user guide) to reflect the new version. Add migration notes if breaking changes exist.
9. **CHANGELOG** — Add a version graduation entry with a summary of breaking changes and migration guidance.
10. **Design evolution** — Update `docs/development/design-evolution.md` with the version bump rationale and any API changes introduced.
11. **Copilot instructions** — Update version references in `.github/copilot-instructions.md` and `.github/instructions/sqlk8soperator.instructions.md`.
12. **Tests** — Update test fixtures in `tests/fixtures/` to use the new version. Run `make test` and `make lint`.

After all steps, run `make manifests generate test lint` to validate the full change set.
