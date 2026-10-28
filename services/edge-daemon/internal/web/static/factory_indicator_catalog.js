// Author: Sunil Jain, 2026-10-28.
// Governed, chapter-linked factory indicator catalog. Missing production evidence is never zero.
(() => {
  'use strict';
  const chapter = [
    ['01', 'Factory setup', '#16869A', '/factory-setup'],
    ['02', 'Buyer order', '#326DB3', '/buyer-review'],
    ['03', 'Plan material', '#7759B2', '/material-planning'],
    ['04', 'Fabric store', '#A87A25', '/stores'],
    ['05', 'Cutting', '#C76B3D', '/cutting'],
    ['06', 'Sewing', '#16847D', '/sewing'],
    ['07', 'Quality', '#3C855E', '/quality'],
    ['08', 'Packing', '#B34E7A', '/packing']
  ].map(([number, title, color, url]) => ({number, title, color, url}));
  // Do not turn these definitions into a scored employee or discipline ranking.
  const indicators = [
    ['setup_coverage',1,'Setup completeness','Required factory facts reviewed / required facts','%','Factory profile and approved setup records','No production-ready count in Chapter 09 yet.'],
    ['machine_readiness',1,'Machine readiness','Verified available machines / machines planned for use','%','Machine roster, inspection and downtime records','A roster entry does not establish machine availability.'],
    ['setup_changes',1,'Pending setup changes','Count of setup changes awaiting review','changes','Factory setup change history','An unapproved change is not an employee failure.'],
    ['order_confirmation',2,'Orders confirmed','Accepted buyer-order revisions with valid due dates','orders','Buyer orders and signed revision history','Draft buyer orders are not confirmed customer demand.'],
    ['bom_coverage',2,'Materials confirmed','Approved material lines / required material lines','%','Source-backed BOM reviews','Uncertain tech-pack suggestions must not be counted as confirmed.'],
    ['order_change_age',2,'Change review time','Time awaiting buyer-order change decisions, excluding nonworking gaps','hours','Buyer revision lifecycle and factory calendar','Elapsed waiting is a workflow signal, not proof of fault.'],
    ['material_coverage',3,'Materials covered','Eligible verified stock / approved gross material need, by unit and item','%','Approved material plan and admitted inventory','Do not combine meters, kilograms and pieces.'],
    ['material_shortage',3,'Uncovered material need','Approved net need minus eligible and reserved supply, by item','item/unit','Approved material plan and stock eligibility checks','Unknown stock is not zero stock.'],
    ['material_plan_age',3,'Plans awaiting answers','Age of material plans waiting for required evidence or approval','plans/days','Material plan versions and approval queue','Exclude time outside the defined working calendar.'],
    ['stock_admitted',4,'Fabric ready for use','Verified, admitted usable roll quantity by material and unit','m/roll','Roll observations and signed stock admission','Observed rolls are not automatically approved inventory.'],
    ['stock_traceability',4,'Roll trace coverage','Accepted roll observations with complete required provenance / checked rolls','%','Roll, supplier and receipt evidence','Do not report unverified origins as complete.'],
    ['roll_reconciliation',4,'Rolls needing checks','Open roll-length or identity mismatches with supporting evidence','rolls','Store reconciliation and adjustment history','A mismatch is a check, not an accusation.'],
    ['cut_positions',5,'Marker positions','Count of saved marker positions for the selected cutting plan','positions','cut_lay_positions_v8','Positions are not cut garments and do not imply actual production.','cut_positions'],
    ['cut_candidates',5,'Draft bundle groups','Distinct saved candidate marker-position and ply-group combinations','groups','cut_bundle_drafts_v8','Draft bundle candidates are not verified cut quantities.','cut_candidates'],
    ['cut_handoff_gaps',5,'Handoffs to check','Number of stage-to-stage count comparisons with mismatches','checks','Chapter 09 residual checks','Unmatched is not automatically lost, scrapped, or a human error.','handoff_gaps'],
    ['sew_complete',6,'Sewing completed','Distinct completed garment serials for the selected rehearsal lay','garments','sew_sim_credits_v9','Practice output only, not paid labor or physical output.','sewn'],
    ['sew_pace',6,'Sewing time spread','Per-operation measured p50 / p90 / p95 active time from completed practice cycles','minutes','sew_sim_cycles_v9 and credited operations','Clock and sample limits apply; not labor efficiency or an operator score.','operation_pace'],
    ['sew_issues',6,'Recorded machine issues','Issue events linked to operation practice cycles','events','sew_sim_events_v9','Missing issue logs do not prove uninterrupted production.','issue_events'],
    ['qc_accepted',7,'Quality accepted','Units currently accepted in rehearsal QC','garments','qc_sim_units_v10','Current acceptance is not first-pass yield.','accepted'],
    ['qc_holds',7,'Waiting for QC answer','Units currently held or awaiting reinspection','garments','qc_sim_units_v10','A hold is a protective quality action, not a worker defect.','hold'],
    ['qc_first_pass',7,'First-time quality pass','Units accepted at first inspection / units first inspected','%','First-inspection event genealogy','Not measured until inspection-first-pass events are reconstructed.'],
    ['packing_current',8,'In cartons','Distinct rehearsal garment serials currently packed','garments','pack_sim_items_v11','Packed does not mean shipped.','packed'],
    ['packing_sealed',8,'In sealed cartons','Distinct rehearsal garment serials in sealed cartons','garments','pack_sim_cartons_v11','Sealed does not establish buyer receipt.','sealed'],
    ['packing_delivery',8,'On-time delivery','Buyer-accepted, on-time and in-full deliveries / committed deliveries','%','Buyer milestones, dispatched manifests and signed receipt','Dispatch rehearsal is not a verified delivery.']
  ].map(([id, c, title, definition, unit, source, caution, selector]) => Object.freeze({id, chapter:chapter[c-1], title, definition, unit, source, caution, selector:selector||null}));

  const stage = (report, name) => {
    const s = (report?.stages||[]).find(x=>x?.name===name);
    return s && Number.isSafeInteger(s.count) && s.count>=0 ? s.count : null;
  };
  const finite = x => typeof x==='number' && Number.isFinite(x) && x>=0;
  function resolve(indicator, report) {
    const unknown = {status:'NOT_MEASURED',value:null,unit:indicator.unit,details:[],plot:[],reason:'This indicator is defined, but this report does not yet contain the validated facts needed to calculate it.'};
    if (!report || report.authority!=='REHEARSAL_ONLY_NOT_PRODUCTION_CLOSURE' || !indicator.selector) return unknown;
    let value=null, details=[], plot=[];
    switch(indicator.selector) {
      case 'cut_positions': value=finite(report.cut_marker_positions)?report.cut_marker_positions:null; break;
      case 'cut_candidates': value=finite(report.cut_bundle_candidate_units)?report.cut_bundle_candidate_units:null; break;
      case 'handoff_gaps': {
        const rows=(report.residuals||[]).filter(x=>x && x.status!=='BALANCED' && Number.isSafeInteger(x.unaccounted));
        if (!Array.isArray(report.residuals)) break;
        value=rows.length;
        details=rows.map(x=>`${x.from} → ${x.to}: ${x.unaccounted} (${x.status})`);
        plot=rows.map(x=>({label:`${x.from} → ${x.to}`,value:Math.abs(x.unaccounted)}));
        break;
      }
      case 'sewn': value=stage(report,'Sewing completed'); break;
      case 'accepted': value=stage(report,'QC accepted'); break;
      case 'hold': value=stage(report,'QC held/reinspect'); break;
      case 'packed': value=stage(report,'Currently packed'); break;
      case 'sealed': value=stage(report,'In sealed carton'); break;
      case 'issue_events': {
        if (!Array.isArray(report.operation_stats)) break;
        const ops=report.operation_stats.filter(x=>x && Number.isSafeInteger(x.issue_events) && x.issue_events>=0);
        if (!ops.length || ops.length !== report.operation_stats.length) break;
        value=ops.reduce((s,x)=>s+x.issue_events,0);
        plot=ops.map(x=>({label:String(x.operation).slice(0,30),value:x.issue_events}));
        break;
      }
      case 'operation_pace': {
        const rows=(report.operation_stats||[]).filter(x=>x?.stats && x.stats.sample_count>=5 && finite(x.stats.p50_minutes) && finite(x.stats.p90_minutes) && finite(x.stats.p95_minutes));
        if (!rows.length) {unknown.reason='At least five valid practice cycles per operation are needed for a comparable time distribution.';return unknown;}
        value=rows.length;
        details=rows.map(x=>`${x.operation}: n=${x.stats.sample_count}; p50 ${x.stats.p50_minutes.toFixed(2)} min, p90 ${x.stats.p90_minutes.toFixed(2)} min, p95 ${x.stats.p95_minutes.toFixed(2)} min`);
        plot=rows.map(x=>({label:String(x.operation).slice(0,30),value:x.stats.p90_minutes}));
        break;
      }
      default: return unknown;
    }
    if (!finite(value)) return unknown;
    if (!plot.length && !['operation_pace','cut_positions','cut_candidates','handoff_gaps','issue_events'].includes(indicator.selector)) {
      const peers=['Sewing completed','QC accepted','QC held/reinspect','Currently packed','In sealed carton'];
      plot=peers.map(name=>({label:name,value:stage(report,name)})).filter(x=>finite(x.value));
    }
    return {status:'SIMULATION',value,unit:indicator.selector==='operation_pace'?'operations':indicator.unit,details,plot,reason:'Source-backed, selected cutting-lay REHEARSAL ONLY. This is not verified factory production.'};
  }
  globalThis.AgiparelIndicatorCatalog=Object.freeze({chapter:Object.freeze(chapter),indicators:Object.freeze(indicators),resolve});
})();
