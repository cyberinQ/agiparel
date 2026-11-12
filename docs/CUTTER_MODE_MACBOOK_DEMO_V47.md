# AGIPAREL Cutter Mode — guided local training demonstration (v47)

Author: Sunil Jain, 2026-11-12

## What this is

An actual, interactive five-step UI for demonstrating *Cutter Mode* to a factory operator. It runs inside the **existing MacBook-native AGIPAREL** server at `http://127.0.0.1:18531/cutting/demo`. The manager's existing `/cutting/sop2` control tower gains a local-only navigation link. No Docker, separate server, schema migration, external API, production credentials or real PO staging is necessary.

**Safety model:** A fake PO, fake rolls, hypothetical extras, operator-reported plies, NFC scans, sewing handoff and log are kept **only in page memory**. They are never written to PostgreSQL, filesystem, factory events or signed evidence systems. Reload resets the demonstration. The only network action is an optional call to AGIPAREL's existing `/api/v1/cutting/sop2/solve` **advisory** optimizer. If unavailable, the UI shows an explicit *preloaded training plan* fallback and does not claim solver verification.

## Demonstration (4–6 minutes)

1. **Customer order:** 400 navy T-shirts, S100/M200/L100. Hypothetical 2% cutting extras make S102/M204/L102 = 408. Click **Start**.
2. **Cut plan:** Show 1S:2M:1L marker, 7.5-m length, two 51-ply lays, same-shade rolls R-101/R-102. Click **Check with live SOP-2 optimizer**; this uses the real advisory solver for the specified training fixture. The UI's 51+51 split is a separate roll-feasible illustration, not necessarily the exact split returned by the solver. Or select the explicitly labeled preloaded plan.
3. **Actual plies:** Record 51 on R-101, then 51 on R-102. Alternatively click **Try 48 plies on first lay**, report 48, then report 54. The balance must reach exact zero, per size. A 55-ply second lay is blocked as over-cut; an underfilled final lay is blocked. Each consumed roll retains 1 m same-roll reserve and reconciles its end bit.
4. **Parts & NFC:** Tap Front, Back, Left sleeve, Right sleeve. Each simulated scan represents a 20-piece M-size sub-bundle bound to the same training garment family and roll/ply lineage. Try early issue: blocked. Four parts: eligible for a training handoff.
5. **Handoff:** Click **Simulate handoff**. The demo shows 20 training kits completed; **real signed released remains zero**. Do not claim physical cutting, binding, authenticated approvals, signed issue or sewing receipt.

At any step, the red buttons demonstrate wrong shade, over-cut, and missing sleeve blocking without modifying the simulated inventory. **Print summary** prints an operator-friendly report, visibly labeled simulation.

## How to install and restart

Save `AGIPAREL_Cutter_Mode_v47_MacBook.zip` in `~/Downloads`, then run:

```bash
# Author: Sunil Jain, 2026-11-12.
# Verify and install source additions with hash guards.
unzip -oq ~/Downloads/AGIPAREL_Cutter_Mode_v47_MacBook.zip -d /tmp/agiparel-cutter-v47
bash /tmp/agiparel-cutter-v47/install.sh ~/agiparel --check
bash /tmp/agiparel-cutter-v47/install.sh ~/agiparel --apply
cd ~/agiparel
node scripts/test_cutter_mode_v47.cjs
bash scripts/agiparel_macbook_localdev.sh down
bash scripts/agiparel_macbook_localdev.sh up
```

**Never run `--apply` if the verification fails.** Installer protects existing files by SHA-256 baseline, backs up any changed original files and does not touch Git history or remote iMac deployment. The `down` command stops the local AGIPAREL app and its private PostgreSQL cluster, not Docker or any other process. The `up` command rebuilds and starts the new native binary; it runs v46 checks first.

## Boundaries / what's next

The tool demonstrates SOP workflows and exceptions; it does **not** replace authoritative, source-backed cutting, physical inspections, CAD marker verification, fabric custody, authenticated approval, scanner/NFC reader signatures, or sewing acceptance. Converting this educational screen into a production operator station requires those authenticated integrations, offline/outbox resiliency, localization and real station hardware commissioning.
