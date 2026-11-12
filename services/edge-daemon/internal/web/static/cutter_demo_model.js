/* Author: Sunil Jain, 2026-11-12.
 * Isolated deterministic garment-cutting training simulator. No production APIs, tags, or DB writes.
 */
(function (root, factory) {
  'use strict';
  const exported = factory();
  if (typeof module === 'object' && module.exports) module.exports = exported;
  if (root) root.CutterDemoModel = exported;
})(typeof window !== 'undefined' ? window : undefined, function () {
  'use strict';
  const ORDER = Object.freeze({ id: 'PO-CUT-DEMO-001', style: 'TSH-101', color: 'NAVY', buyer: Object.freeze({ S: 100, M: 200, L: 100 }), target: Object.freeze({ S: 102, M: 204, L: 102 }) });
  const RATIO = Object.freeze({ S: 1, M: 2, L: 1 });
  const MARKER = Object.freeze({ id: 'TRAINING-MARK-121', lengthM: 7.5, widthM: 1.5, maxPlies: 55 });
  const RESERVE_M = 1;
  const ROLLS = Object.freeze([
    Object.freeze({ id: 'R-101', shade: 'NAVY-A', lengthM: 400, widthM: 1.55 }),
    Object.freeze({ id: 'R-102', shade: 'NAVY-A', lengthM: 420, widthM: 1.55 }),
    Object.freeze({ id: 'R-103', shade: 'NAVY-B', lengthM: 500, widthM: 1.55 })
  ]);
  const PARTS = Object.freeze([
    Object.freeze({ id: 'FRONT', label: 'Front panel', tag: 'SIM-001' }),
    Object.freeze({ id: 'BACK', label: 'Back panel', tag: 'SIM-002' }),
    Object.freeze({ id: 'SL', label: 'Left sleeve', tag: 'SIM-003' }),
    Object.freeze({ id: 'SR', label: 'Right sleeve', tag: 'SIM-004' })
  ]);
  const clone = value => JSON.parse(JSON.stringify(value));
  const initial = () => ({ started: false, optimized: false, optimizerSource: '', optimizerEvidence: null, lays: [], scanned: [], issued: false, alerts: [], activity: [], nextEvent: 1 });
  function log(s, kind, message) {
    s.activity.unshift({ sequence: s.nextEvent++, kind, message });
    s.activity = s.activity.slice(0, 30);
    if (kind === 'blocked') s.alerts.unshift(message);
    s.alerts = s.alerts.slice(0, 5);
    return s;
  }
  const total = qty => Object.values(qty).reduce((a, b) => a + b, 0);
  function produced(s) {
    const plies = s.lays.reduce((v, l) => v + l.plies, 0);
    return { S: plies, M: 2 * plies, L: plies };
  }
  function balance(s) {
    const actual = produced(s);
    return Object.fromEntries(Object.keys(RATIO).map(k => [k, ORDER.target[k] - actual[k]]));
  }
  function doneCutting(s) { return s.lays.length === 2 && Object.values(balance(s)).every(v => v === 0); }
  function nextPlies(s) { return balance(s).S; }
  function begin(s) {
    if (s.started) return s;
    s.started = true;
    return log(s, 'info', 'Training order loaded: 400 buyer pieces; teaching target 408 including 2% extras. No database record created.');
  }
  function markOptimized(s, result, source) {
    if (!s.started) return log(s, 'blocked', 'Load the sample order first.');
    if (s.lays.length) return log(s, 'blocked', 'Cannot replace a simulated plan after reporting cuts. Reset to start again.');
    if (source === 'engine') {
      const target = result && result.target_by_sku;
      if (!result || result.feasible !== true || !Array.isArray(result.steps) || result.cuts !== 2 ||
          !target || target['NAVY|S'] !== 102 || target['NAVY|M'] !== 204 || target['NAVY|L'] !== 102) {
        return log(s, 'blocked', 'Solver did not validate the demo quantity and two-cut objective; no plan accepted.');
      }
      s.optimizerEvidence = { cuts: result.cuts, proven: result.optimality_proven === true, truncated: result.search_truncated === true, steps: clone(result.steps) };
    } else if (source === 'training') {
      s.optimizerEvidence = null;
    } else {
      return log(s, 'blocked', 'Unknown planning evidence.');
    }
    s.optimized = true;
    s.optimizerSource = source;
    return log(s, 'ok', source === 'engine'
      ? 'SOP-2 solver found a two-cut candidate. Training selects a 51+51 ply split for roll-fit checks; no CAD approval is claimed.'
      : 'Illustrative 51+51 ply plan loaded WITHOUT solver verification. Training only; no cutting authority.');
  }
  function checkLay(s, plies, rollID) {
    if (!s.started || !s.optimized) return { ok: false, reason: 'First load the sample order and evaluate a plan.' };
    if (!Number.isSafeInteger(plies) || plies <= 0) return { ok: false, reason: 'Plies must be a positive whole number.' };
    const remaining = nextPlies(s);
    if (plies > remaining) return { ok: false, reason: `OVER-CUT BLOCKED: only ${remaining} plies (${remaining * 4} garments) remain; ${plies} would exceed frozen demo targets.` };
    if (s.lays.length >= 2) return { ok: false, reason: 'The two-cut plan is already complete. Extra cuts are blocked.' };
    if (plies > MARKER.maxPlies) return { ok: false, reason: `Cutter limit is ${MARKER.maxPlies} plies.` };
    const expected = ROLLS[s.lays.length];
    const roll = ROLLS.find(x => x.id === rollID);
    if (!roll) return { ok: false, reason: 'Select a registered demo roll.' };
    if (roll.shade !== 'NAVY-A') return { ok: false, reason: `SHADE BLOCKED: ${roll.id} is ${roll.shade}; this order requires NAVY-A.` };
    if (roll.id !== expected.id) return { ok: false, reason: `ROLL ALLOCATION BLOCKED: lay ${s.lays.length + 1} is reserved for ${expected.id}. Reallocation requires replanning.` };
    if (roll.widthM < MARKER.widthM) return { ok: false, reason: 'Usable roll width is smaller than the marker.' };
    const usedM = plies * MARKER.lengthM;
    if (usedM + RESERVE_M > roll.lengthM + 1e-9) return { ok: false, reason: `FABRIC SHORTAGE: ${roll.id} cannot supply ${plies} plies and its same-roll recut reserve.` };
    if (s.lays.length === 0) {
      const future = remaining - plies;
      const second = ROLLS[1];
      if (future < 1 || future > MARKER.maxPlies || future * MARKER.lengthM + RESERVE_M > second.lengthM) {
        return { ok: false, reason: `BALANCE BLOCKED: a first lay of ${plies} leaves ${future} plies, which the remaining approved roll cannot clear.` };
      }
    } else if (plies !== remaining) {
      return { ok: false, reason: `UNDER-CUT BLOCKED: the final lay must clear all ${remaining} plies. Replan if actual spreading differs.` };
    }
    return { ok: true, reason: 'Within exact target, shade, marker, cutter and fabric limits.', usedM, reserveM: RESERVE_M, endM: Math.round((roll.lengthM - usedM - RESERVE_M) * 1000) / 1000 };
  }
  function applyLay(s, plies, rollID) {
    const result = checkLay(s, plies, rollID);
    if (!result.ok) return { ok: false, reason: result.reason, state: log(s, 'blocked', result.reason) };
    const lay = { number: s.lays.length + 1, plies, rollID, shade: 'NAVY-A', usedM: result.usedM, reserveM: result.reserveM, endM: result.endM, garmentCount: plies * 4 };
    s.lays.push(lay);
    log(s, 'ok', `SIMULATED lay ${lay.number}: ${plies} plies, ${lay.garmentCount} garments, roll ${rollID}; not a physical cut.`);
    if (doneCutting(s)) log(s, 'ok', 'Exact-zero teaching balance achieved: 102 S / 204 M / 102 L reported in simulation. Signed release remains zero.');
    else log(s, 'info', `Simulated balance updated: ${nextPlies(s)} plies left. The next lay is recalculated from actual reported plies.`);
    return { ok: true, state: s, lay };
  }
  function scanPart(s, id) {
    if (!doneCutting(s)) return { ok: false, state: log(s, 'blocked', 'Finish two correctly balanced simulated lays before scanning a bundle.') };
    if (s.issued) return { ok: false, state: log(s, 'blocked', 'This training kit is already demonstrated as issued; reset to repeat.') };
    const part = PARTS.find(p => p.id === id);
    if (!part) return { ok: false, state: log(s, 'blocked', 'Unknown simulated sub-bundle tag.') };
    if (s.scanned.includes(id)) return { ok: false, state: log(s, 'blocked', `Duplicate scan ${part.label} ignored.`) };
    s.scanned.push(id);
    log(s, 'ok', `SIMULATED NFC scan: ${part.label} • ${part.tag} • 20 pieces • roll R-101 • first 10 plies of lay 1.`);
    return { ok: true, state: s };
  }
  function verifyBundle(s, scanned = s.scanned) {
    if (!doneCutting(s)) return { ok: false, reason: 'No completed simulated cut set.' };
    if (!Array.isArray(scanned) || new Set(scanned).size !== scanned.length || scanned.some(id => !PARTS.some(p => p.id === id))) return { ok: false, reason: 'Duplicate or unknown NFC identities.' };
    const missing = PARTS.filter(part => !scanned.includes(part.id));
    if (missing.length) return { ok: false, reason: `PARTIAL ISSUE BLOCKED: missing ${missing.map(p => p.label).join(', ')}; ${scanned.length}/4 sub-bundles scanned.` };
    return { ok: true, reason: 'All four sub-bundles scanned and married to the same 20 simulated garments. Signed release not established.' };
  }
  function issue(s) {
    if (s.issued) return { ok: false, state: log(s, 'blocked', 'Duplicate issue blocked.') };
    const verdict = verifyBundle(s);
    if (!verdict.ok) return { ok: false, state: log(s, 'blocked', verdict.reason), reason: verdict.reason };
    s.issued = true;
    log(s, 'ok', 'DEMO HANDOFF ONLY: 20 M-size kits passed the simulated marriage check. Real signed releases = 0; no sewing inventory changed.');
    return { ok: true, state: s };
  }
  function scenario(s, key) {
    if (key === 'shade') {
      const message = 'SHADE BLOCKED: test roll R-103 (NAVY-B) cannot be paired with NAVY-A garments.';
      return { ok: false, state: log(s, 'blocked', message), reason: message };
    }
    if (key === 'overcut') {
      const proposed = s.lays.length === 1 ? nextPlies(s) + 1 : s.lays.length === 2 ? 1 : 103;
      const message = `OVER-CUT BLOCKED: trying ${proposed} extra/incorrect ply count would exceed the remaining per-size target. No lay added.`;
      return { ok: false, state: log(s, 'blocked', message), reason: message };
    }
    if (key === 'missing') {
      const pretend = PARTS.filter(p => p.id !== 'SL').map(p => p.id);
      const message = 'PARTIAL ISSUE BLOCKED: Left sleeve is missing in the demonstration marriage family (3/4 scanned). No release.';
      // Explicit scenario check is independent of the actual scanning flow, and cannot change it.
      if (doneCutting(s) && verifyBundle(s, pretend).ok) throw Error('Safety assertion: missing part was incorrectly accepted');
      return { ok: false, state: log(s, 'blocked', message), reason: message };
    }
    return { ok: false, state: log(s, 'blocked', 'Unknown training exception.') };
  }
  function fabricBalance(s) {
    return s.lays.map(lay => ({
      rollID: lay.rollID, issuedM: ROLLS[lay.number - 1].lengthM, laidM: lay.usedM,
      reserveM: lay.reserveM, endM: lay.endM, deltaM: Math.round((ROLLS[lay.number - 1].lengthM - lay.usedM - lay.reserveM - lay.endM) * 1000) / 1000
    }));
  }
  function snapshot(s) {
    const actual = produced(s), remaining = balance(s);
    return { ordered: total(ORDER.buyer), target: total(ORDER.target), simulatedCut: total(actual), remaining: total(remaining), signedReleased: 0, scans: s.scanned.length, sampleKitCount: 20, cuts: s.lays.length, doneCutting: doneCutting(s), kitReady: verifyBundle(s).ok, issued: s.issued, fabric: fabricBalance(s) };
  }
  return Object.freeze({ ORDER, RATIO, MARKER, ROLLS, PARTS, RESERVE_M, initial, begin, markOptimized, produced, balance, nextPlies, doneCutting, checkLay, applyLay, scanPart, verifyBundle, issue, scenario, fabricBalance, snapshot });
});
