// Author: Sunil Jain, 2026-10-28.
// Pure, evidence-limited Chapter 09 summaries. Missing values remain unknown, never zero.
(() => {
  'use strict';
  const AUTHORITY = 'REHEARSAL_ONLY_NOT_PRODUCTION_CLOSURE';
  const knownCount = (report, name) => {
    const stage = (report?.stages || []).find(s => s?.name === name);
    return stage && Number.isSafeInteger(stage.count) && stage.count >= 0 ? stage.count : null;
  };
  const summary = report => ({
    sewn: knownCount(report, 'Sewing completed'),
    accepted: knownCount(report, 'QC accepted'),
    hold: knownCount(report, 'QC held/reinspect'),
    packed: knownCount(report, 'Currently packed')
  });
  const showCount = n => n == null ? 'Not measured' : n.toLocaleString('en-US');
  const num = n => Number.isFinite(n) ? n.toFixed(2) : '—';
  const findings = report => {
    const issues = (Array.isArray(report?.alerts) ? report.alerts : [])
      .filter(s => typeof s === 'string' && s.trim()).map(s => s.trim());
    const unusual = (report?.residuals || []).filter(x => x && x.status === 'OPEN_WIP_OR_UNKNOWN_LOSS' && Number(x.unaccounted) > 0);
    // Never call residual WIP 'scrap' or infer a cause.
    for (const row of unusual) {
      issues.push(`${row.from} → ${row.to}: ${row.unaccounted} practice unit(s) not matched at the next step; check pending work and source evidence.`);
    }
    const negatives = (report?.residuals || []).filter(x => x && x.status === 'NEGATIVE_INVARIANT');
    for (const row of negatives) {
      issues.push(`${row.from} → ${row.to}: downstream count exceeds upstream; check duplicate or missing source records.`);
    }
    const unique = [...new Set(issues)];
    return unique.length ? unique : ['No mismatch was flagged in this practice report. This does not verify real production or prove all work is complete.'];
  };
  const question = report => {
    const s = summary(report);
    const items = findings(report).slice(0, 3).join('; ');
    return (`Help me review this PRACTICE-ONLY Agiparel Chapter 09 report for cutting lay ${String(report.lay_id || '').slice(0, 45)}. ` +
      `Source=${String(report.evidence || '').slice(0, 65)}; authority=${String(report.authority || '').slice(0, 60)}. ` +
      `Distinct simulated serial counts: sewing=${showCount(s.sewn)}, QC accepted=${showCount(s.accepted)}, QC held=${showCount(s.hold)}, packed=${showCount(s.packed)}. ` +
      `Checks: ${items.slice(0, 300)}. ` +
      'Explain what these records show, suggest specific checks, and label unknowns. Do not invent a cause, claim live output, or calculate unavailable KPIs.').slice(0, 980);
  };
  globalThis.AgiparelReportMetrics = Object.freeze({AUTHORITY, summary, showCount, num, findings, question});
})();
