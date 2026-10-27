'use strict';
// Read-only evidence and release register; secure signatures never originate in this page.
const cutReleaseEl=id=>document.getElementById(id);
let cutReleaseDraftID='';
function setCutBadge(released, message) {
 const badge = cutReleaseEl('metric-cut-release');
 if (!badge) return;
 badge.textContent = message || (released ? 'RELEASED TO SEWING' : 'NOT RELEASED');
 badge.classList.toggle('hold', !released);
 badge.classList.toggle('ag-status-good', !!released);
}
async function refreshCutRelease(){
 if(!cutReleaseDraftID)return;
 const id=cutReleaseDraftID;
 setCutBadge(false, 'CHECKING…');
 const box=cutReleaseEl('cut-release-evidence');
 try{
  const res=await fetch(`/api/v1/cutting/release-status?draft_id=${encodeURIComponent(id)}`,{cache:'no-store'});
  const json=await res.json();if(!res.ok)throw new Error(json.error||`HTTP ${res.status}`);
  if(id!==cutReleaseDraftID)return;
  const render=(label,val)=>{const p=document.createElement('p');p.textContent=`${label}: ${val}`;box.append(p);};
  box.replaceChildren();
  if(json.state==='NO_SPREAD_EVIDENCE'){setCutBadge(false);render('Status','No signed-issue-backed spread report yet');return;}
  setCutBadge(json.released===true);
  render('Bundles expected',json.expected_bundles);
  render('Human-signed EPC witnesses',json.human_witnessed_bundles);
  render('Enrolled gateway event reports',json.gateway_reported_bundles);
  render('Matching label/EPC pairs',json.matched_bundles);
  render('Cutting handoff',json.released?'RELEASED TO SEWING — independently signed':'HOLD — not released');
  render('Approval control','Separate, strongly authenticated supervisor and quality signatures required; no approval from this screen');
  const note=document.createElement('p');note.className='hint';note.textContent='A gateway signature establishes reporting provenance; physical RF capture still depends on verified reader installation and operation.';box.append(note);
 }catch(err){setCutBadge(false, 'CHECK STATUS');box.textContent=`Could not check cutting release: ${err.message}`;}
}
document.addEventListener('cut-draft-selected',({detail})=>{cutReleaseDraftID=detail.draft.draft_id;setCutBadge(false);refreshCutRelease();});
document.addEventListener('cut-spread-refreshed',()=>refreshCutRelease());
cutReleaseEl('release-refresh').addEventListener('click',refreshCutRelease);
