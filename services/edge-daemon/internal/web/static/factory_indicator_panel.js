// Author: Sunil Jain, 2026-10-28.
// Accessible eight-chapter indicator index and full-viewport evidence viewer; no invented metrics.
(() => {
  'use strict';
  const C=globalThis.AgiparelIndicatorCatalog;
  const $=id=>document.getElementById(id);
  const grid=$('indicator-grid');
  if (!C || !grid) return;
  let report=null,active=null,previousFocus=null;
  const pretty=n=>Number.isFinite(n)?n.toLocaleString('en-US'):'—';
  const set=(id,value)=>{$(id).textContent=String(value);};
  function element(tag,cls,txt) {
    const node=document.createElement(tag);
    if (cls) node.className=cls;
    if (txt!=null) node.textContent=txt;
    return node;
  }
  function renderGrid() {
    grid.replaceChildren();
    for (const chapter of C.chapter) {
      const group=element('section','ag-indicator-group');
      group.style.setProperty('--indicator-accent',chapter.color);
      const heading=element('h3','ag-indicator-group-name',`${chapter.number} · ${chapter.title}`);
      group.append(heading);
      for (const item of C.indicators.filter(x=>x.chapter===chapter)) {
        const value=C.resolve(item,report);
        const button=element('button','ag-indicator-stamp');
        button.type='button';
        button.setAttribute('aria-label',`${chapter.title}: ${item.title}; ${value.status==='SIMULATION'?'practice evidence':'not measured'}`);
        const title=element('span','ag-indicator-stamp-title',item.title);
        const status=element('small','ag-indicator-stamp-status',value.status==='SIMULATION'?`Practice · ${pretty(value.value)} ${value.unit}`:'Not measured');
        button.append(title,status);
        button.addEventListener('click',()=>open(item,button));
        group.append(button);
      }
      grid.append(group);
    }
  }
  function graph(rows, indicator) {
    const host=$('indicator-plot');
    host.replaceChildren();
    const safe=(Array.isArray(rows)?rows:[]).filter(x=>x && typeof x.label==='string' && Number.isFinite(x.value) && x.value>=0).slice(0,16);
    if (!safe.length) {
      set('indicator-graph-note','No comparable values in this report. A chart would be misleading.');
      host.append(element('p','ag-indicator-empty','No chart available from the selected evidence.'));
      return;
    }
    const note = indicator.selector==='operation_pace' ? 'p90 active-cycle minutes by operation (at least 5 valid cycles each). One report, not a dated trend.'
      : indicator.selector==='issue_events' ? 'Issue-event count per practice operation. Missing logs are not proof of uninterrupted work.'
      : indicator.selector==='handoff_gaps' ? 'Unmatched practice serial count per handoff. Not evidence of scrap or a cause.'
      : 'Distinct simulated garment serials by stage. States can overlap; do not add counts together. One report, not a dated trend.';
    set('indicator-graph-note', note);
    const max=Math.max(...safe.map(x=>x.value));
    for (const row of safe) {
      const bar=element('div','ag-indicator-chart-row');
      const lbl=element('span','ag-indicator-chart-label',row.label);
      const track=element('div','ag-indicator-chart-track');
      const fill=element('div','ag-indicator-chart-fill');
      fill.style.width=(max>0?(row.value/max)*100:0)+'%';
      track.append(fill);
      const count=element('strong','ag-indicator-chart-value',pretty(row.value));
      bar.append(lbl,track,count);
      host.append(bar);
    }
  }
  function open(item,trigger) {
    active=item;
    previousFocus=trigger || document.activeElement;
    const result=C.resolve(item,report);
    const view=$('indicator-view');
    view.style.setProperty('--indicator-accent',item.chapter.color);
    set('indicator-chapter',`${item.chapter.number} · ${item.chapter.title}`);
    set('indicator-title',item.title);
    set('indicator-state',result.status==='SIMULATION'?'SIMULATION / PRACTICE EVIDENCE':'NOT MEASURED');
    set('indicator-value',result.status==='SIMULATION'?pretty(result.value):'—');
    set('indicator-unit',result.status==='SIMULATION'?result.unit:'No validated value');
    set('indicator-definition',item.definition);
    set('indicator-caution',item.caution);
    set('indicator-explanation',result.reason);
    set('indicator-source',`Source requirements: ${item.source}.`);
    set('indicator-evidence',result.status==='SIMULATION'?
      `Selected rehearsal lay ${String(report.lay_id||'—')}. ${String(report.evidence||'No evidence code')}. Captured at ${String(report.generated_at||'unknown')}.`:
      'Source not joined to the selected Chapter 09 report. No data should be inferred from an uploaded document alone.');
    $('indicator-source-link').href=item.chapter.url;
    const checks=$('indicator-checks');
    checks.replaceChildren();
    for (const line of (result.details.length?result.details.slice(0,12):['Review source evidence and missing records before drawing a conclusion.',item.caution])) checks.append(element('li',null,line));
    graph(result.plot,item);
    view.hidden=false;
    document.body.classList.add('ag-indicator-open');
    $('indicator-close').focus();
  }
  function close() {
    if ($('indicator-view').hidden) return;
    $('indicator-view').hidden=true;
    document.body.classList.remove('ag-indicator-open');
    active=null;
    if (previousFocus?.isConnected) previousFocus.focus();
  }
  $('indicator-close').addEventListener('click',close);
  $('indicator-report-link').addEventListener('click',close);
  document.addEventListener('keydown',event=>{
    if ($('indicator-view').hidden) return;
    if (event.key==='Escape') {event.preventDefault();close();}
    if (event.key==='Tab') {
      const focusable=[...$('indicator-view').querySelectorAll('a[href],button:not([disabled])')].filter(n=>!n.hidden);
      const first=focusable[0],last=focusable[focusable.length-1];
      if (event.shiftKey && document.activeElement===first) {event.preventDefault();last.focus();}
      else if (!event.shiftKey && document.activeElement===last) {event.preventDefault();first.focus();}
    }
  });
  document.addEventListener('ag:report-loaded',event=>{report=event.detail;renderGrid();if(active)open(active,previousFocus);});
  document.addEventListener('ag:report-cleared',()=>{report=null;renderGrid();if(active)close();});
  renderGrid();
})();
