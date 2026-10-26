# Gate 06.5-A.1 — Unified Navigation & One Document Inbox

**Prerequisite:** Gate 06.5-A (`/documents` service and PostgreSQL migration 0009) is already installed. Apply this on top of Gate 06.5-A. The patch intentionally does **not** modify authentication (`portal.go`), database tables, Compose, or the ingestion worker.

## Purpose

- One global Home / Back / Upload documents / factory-workspace navigation across the seven operator HTML pages.
- One visible document upload starting point, `/documents`, for both customer and factory source files.
- Homepage has a prominent upload action, including at phone widths.
- Plant Overview and Onboarding general-upload buttons now point at central Document Intelligence.
- Existing CSV staging/calculators stay available but are clearly marked advanced/temporary; Onboarding retains legacy planning upload behind a collapsed Advanced panel (to avoid breaking the current simulator).
- Database semantics are unchanged: source extraction != approved order or inventory.

## Installation (recommended: ZIP)

Download `agiparel_gate065a1_ux_changes.zip` and save in `~/Downloads`.

```bash
cd ~/agiparel
unzip -l ~/Downloads/agiparel_gate065a1_ux_changes.zip
unzip -o ~/Downloads/agiparel_gate065a1_ux_changes.zip -d ~/agiparel
```

**Do not apply the patch if you installed the ZIP.**

Alternative:

```bash
cd ~/agiparel
git apply --check ~/Downloads/AGIPAREL_GATE065A1_unified_navigation_document_inbox.patch && \
git apply ~/Downloads/AGIPAREL_GATE065A1_unified_navigation_document_inbox.patch
```

The patch was verified to apply against the Gate 06.5-A source baseline. Local modifications to these same HTML/CSS files may cause `git apply --check` to fail; in that case, examine `git diff` before overwriting.

## Tests

```bash
cd ~/agiparel
go test -count=1 ./services/edge-daemon/internal/web
go test -count=1 ./...
go vet ./...
go build ./...
git diff --check
```

New `internal/web/navigation_test.go` validates the navigation hook on all seven restricted pages and the central upload entry points. No new SQL migration is required.

## Deploy ONLY the existing Agiparel edge service

```bash
cd ~/agiparel
docker compose -f compose/docker-compose.dev.yml up -d --build --no-deps --force-recreate plant-edge-os
```

Gate 06.5-A Compose defaults already use the development operator username `dev`, password `1212`, and tenant `dev`. If old shell environment variables are still set, they override those defaults. Before deploying, optionally check without disclosing secrets:

```bash
for key in AGIPAREL_OPERATOR_USER AGIPAREL_OPERATOR_PASSWORD AGIPAREL_OPERATOR_TENANT; do
  if [ -n "${(P)key}" ]; then echo "$key overridden by current zsh shell"; fi
done
```

Or explicitly clear obsolete overrides:

```bash
unset AGIPAREL_OPERATOR_USER AGIPAREL_OPERATOR_PASSWORD AGIPAREL_OPERATOR_TENANT
```

**Only do that if you intend to use the Gate 06.5-A Compose defaults.** Production deployments must not use the dev credentials.

## Manual acceptance

1. Visit `https://agiparel.empowerai.us/`: top-right link reads **Upload your documents**, sidebar links to the same destination.
2. Visit `/documents`: persistent global bar has **Home**, **← Back**, **Upload documents**, and direct factory navigation. Main screen explains that all customer/factory files go here. Upload remains evidence-only.
3. Directly open `/plant`, `/planning`, `/order-intake`, `/material-planning`, `/cutting`, `/sew01`: consistent bar appears; Home works; Back navigates to an appropriate previous page or Home.
4. On `/plant`, old general upload actions are replaced by one central upload link.
5. On `/planning`, old direct planning importer is behind the collapsed **Advanced · legacy planning import (temporary)** disclosure. The primary CTA goes to `/documents`.
6. On `/order-intake`, the CSV form remains available and is explicitly marked **Advanced · temporary**. On `/material-planning`, CSV source sheets remain available as what-if evidence until Gate 06.5-C.
7. Avoid sensitive employee/payroll data in this weakly protected dev instance.

## Scope and next gate

This increment changes **13** source files (11 web assets, one Go web test, one UX requirements doc). It does not add classification, data approval, OCR, legacy XLS extraction, Google connectors, or canonical handoff. Those remain explicit commitments for Gate 06.5-B/C, not current functionality.

## Local validation notes

- New embedded-web Go regression tests passed in isolated Go 1.23 harness.
- `workspace_nav.js` passed Node syntax validation.
- ZIP byte integrity verified; Git patch passed `git apply --check` on Gate 06.5-A baseline.
- Browser-level render verification and full Go 1.24 suite were not completed here. Run the commands above on Mac.
