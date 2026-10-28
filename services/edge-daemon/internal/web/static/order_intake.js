// Author: Sunil Jain, 2026-10-28 — Accept Excel templates and source files for proposed buyer-order intake.
(()=>{'use strict';
const $=id=>document.getElementById(id);
const format=n=>Number(n).toLocaleString('en-US');
const alert=(text,type='')=>{const el=$('alert');el.textContent=text;el.className='alert '+type};
const add=(tag,text,parent,cls)=>{const node=document.createElement(tag);node.textContent=text??'';if(cls)node.className=cls;parent.append(node);return node};
let previewed=null;
function show(rep){
 $('po-name').textContent=rep.po_number;$('style-name').textContent=rep.style_code;
 $('quantity').textContent=format(rep.ordered_qty);$('release').textContent=rep.readiness;
 $('digest').textContent=rep.source_sha256;
 $('plan-link').href=rep.id?`/material-planning?intake=${encodeURIComponent(rep.id)}`:'/material-planning';
 const skus=$('sku-rows');skus.replaceChildren();for(const s of rep.skus){const tr=add('tr','',skus);for(const v of [s.color,s.size,s.upc||'—',format(s.quantity)])add('td',v,tr)}
 const bom=$('bom-rows');bom.replaceChildren();for(const m of rep.materials){const tr=add('tr','',bom);for(const v of [m.material_sku,m.category,m.per_garment,`${m.required} ${m.uom}`])add('td',v,tr)}
 const holds=$('holds');holds.replaceChildren();for(const hold of rep.holds)add('li',hold,holds);
}
async function response(res){if(!res.ok)throw new Error((await res.text()).trim().slice(0,300)||`HTTP ${res.status}`);return res.json()}
async function submit(action){const po=$('po-file').files[0],bom=$('bom-file').files[0];if(!po||!bom){alert('Choose both a buyer order sheet and a material list.','error');return}const form=new FormData();form.append('po_file',po);form.append('bom_file',bom);
 try{alert(action==='preview'?'Validating source files…':'Saving original source files and proposed facts…');$('preview-btn').disabled=true;$('stage-btn').disabled=true;
  const rep=await response(await fetch(`/api/v1/order-intake?action=${action}`,{method:'POST',body:form,credentials:'same-origin'}));show(rep);
  if(action==='preview'){previewed={po,bom,digest:rep.source_sha256};$('stage-btn').disabled=false;alert('Preview passed. Evidence has NOT been staged or approved.','success')}
  else{previewed=null;alert(`Staged as ${rep.id}. The release gate remains HOLD; production was not changed.`,'success');await refresh()}
 }catch(err){alert(err.message,'error')}finally{$('preview-btn').disabled=false;if(action==='preview'&&!previewed)$('stage-btn').disabled=true}
}
$('intake-form').addEventListener('submit',event=>{event.preventDefault();void submit('preview')});
$('stage-btn').addEventListener('click',()=>{if(!previewed||previewed.po!==$('po-file').files[0]||previewed.bom!==$('bom-file').files[0]){alert('Files changed: preview again before staging.','error');return}void submit('stage')});
for(const id of ['po-file','bom-file'])$(id).addEventListener('change',()=>{previewed=null;$('stage-btn').disabled=true;alert('Inputs changed. Revalidate before staging.')});
document.querySelectorAll('[data-template]').forEach(btn=>btn.addEventListener('click',()=>{
 const files={po:'agiparel_buyer_po_template.xlsx',bom:'agiparel_style_bom_template.xlsx'};
 const name=files[btn.dataset.template];const a=document.createElement('a');a.href='/static/'+name;a.download=name;a.click();
}));
async function refresh(){const history=$('history');try{const result=await response(await fetch('/api/v1/order-intake',{credentials:'same-origin'}));history.replaceChildren();if(!result.items.length){history.textContent='No staged intakes for this operator tenant.';return}for(const item of result.items){const row=add('button','',history);row.type='button';const title=add('strong',`${item.po_number} · ${item.style_code} · ${format(item.ordered_qty)} pcs`,row);const right=add('small',`${item.status} · ${new Date(item.captured_at).toLocaleString()}`,row);row.addEventListener('click',()=>void inspect(item.id))}}catch(e){history.textContent=`Cannot load staged intakes: ${e.message}`}}
async function inspect(id){try{show(await response(await fetch(`/api/v1/order-intake?id=${encodeURIComponent(id)}`,{credentials:'same-origin'})));alert(`Loaded ${id} from PostgreSQL. Source values remain PROPOSED.`,'success')}catch(e){alert(e.message,'error')}}
$('refresh').addEventListener('click',()=>void refresh());void refresh();
})();
