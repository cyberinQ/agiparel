# Agiparel UX refresh — install and test

**Baseline:** `97e4cc3` (after repository cleanup). **No new SQL migration.** **No database reset.** **No changes to service APIs.**

Use the same delivery recipe you have used for earlier batches. The ZIP includes only changed/new paths inside the repository; no root directory is nested in it.

## 1 — Inflate on your development iMac

Place `agiparel_customer_ux_refresh.zip` in `~/Downloads`.

```bash
cd ~/agiparel
git status --short
git rev-parse --short HEAD
unzip -o ~/Downloads/agiparel_customer_ux_refresh.zip -d ~/agiparel
git diff --check
```

**Do not overwrite unrelated local edits blindly.** Keep your 12A/B implementation intact. This ZIP changes the static user interface, a new UI consistency test, and these docs only.

## 2 — Migrate

**Not needed.** The database remains exactly as it is (migrations `0001`–`0025`; your chosen seed/blank state untouched).

## 3 — Test

```bash
go test -count=1 ./...
go vet ./...
go build ./...
git diff --check
```

Review any failure before rebuilding. Tests assert important safeguards and page IDs; never solve a failure by deleting production checks.

## 4 — Deploy only the Agiparel web service

```bash
docker compose -f compose/docker-compose.dev.yml \
  up -d --build --no-deps --force-recreate plant-edge-os
```

## 5 — Smoke test with a browser

1. Open `/` and confirm there are **no sample orders/quantities or fake completed RFID checkpoints**.
2. Open `/factory-setup` and `/documents`; navigate Previous/Next and More tools; keep central upload and factory file discovery working.
3. On `/buyer-review`, choose buyer sources; confirm you can still check PO rows, materials, and save drafts. `/order-intake` remains available in More tools for paired `.csv` files.
4. On `/stores`, test roll form, roll history, reconciliation, admissions-readiness and candidate roll picks; no button should create approved stock.
5. On `/cutting`, verify lay/ply/bundle inputs, spread reports and **NOT RELEASED** status checks remain visible and functional.
6. On `/sewing`, `/quality` and `/packing`, confirm demo scan buttons, events and pending retry flow still work. Do not treat these as hardware evidence.
7. On `/intelligence`, verify the zero-data state, tables and snapshots. No saved/unknown time should be presented as a measured delay.
8. Test narrow/mobile width, keyboard Tab navigation and focus; all key buttons should stay reachable.

Note: no seed data is bundled. If the database is empty, many workflows will show an appropriate empty state. This is expected and preferable to fabricated business data.

## Change safety

- No new migrations or automatic production state changes.
- CSS uses scoped page selectors rather than global `section`/`button` `!important` overrides.
- A phase color is for navigation. PASS/HOLD/REJECT status colors and backend enforcement have their own meanings.
- `workspace_nav.js` retains all existing specialist routes and Shadow DOM style isolation.
