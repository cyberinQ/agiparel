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
 const skus=$('sku-rows');skus.replaceChildren();for(const s of rep.skus){const tr=add('tr','',skus);for(const v of [s.color,s.size,s.upc||'—',format(s.quantity)])add('td',v,tr)}
 const bom=$('bom-rows');bom.replaceChildren();for(const m of rep.materials){const tr=add('tr','',bom);for(const v of [m.material_sku,m.category,m.per_garment,`${m.required} ${m.uom}`])add('td',v,tr)}
 const holds=$('holds');holds.replaceChildren();for(const hold of rep.holds)add('li',hold,holds);
}
async function response(res){if(!res.ok)throw new Error((await res.text()).trim().slice(0,300)||`HTTP ${res.status}`);return res.json()}
async function submit(action){const po=$('po-file').files[0],bom=$('bom-file').files[0];if(!po||!bom){alert('Choose both a buyer PO CSV and a BOM CSV.','error');return}const form=new FormData();form.append('po_file',po);form.append('bom_file',bom);
 try{alert(action==='preview'?'Validating source files…':'Saving original source files and proposed facts…');$('preview-btn').disabled=true;$('stage-btn').disabled=true;
  const rep=await response(await fetch(`/api/v1/order-intake?action=${action}`,{method:'POST',body:form,credentials:'same-origin'}));show(rep);
  if(action==='preview'){previewed={po,bom,digest:rep.source_sha256};$('stage-btn').disabled=false;alert('Preview passed. Evidence has NOT been staged or approved.','success')}
  else{previewed=null;alert(`Staged as ${rep.id}. The release gate remains HOLD; production was not changed.`,'success');await refresh()}
 }catch(err){alert(err.message,'error')}finally{$('preview-btn').disabled=false;if(action==='preview'&&!previewed)$('stage-btn').disabled=true}
}
$('intake-form').addEventListener('submit',event=>{event.preventDefault();void submit('preview')});
$('stage-btn').addEventListener('click',()=>{if(!previewed||previewed.po!==$('po-file').files[0]||previewed.bom!==$('bom-file').files[0]){alert('Files changed: preview again before staging.','error');return}void submit('stage')});
for(const id of ['po-file','bom-file'])$(id).addEventListener('change',()=>{previewed=null;$('stage-btn').disabled=true;alert('Inputs changed. Revalidate before staging.')});
const templates={po:'po_number,style_code,color_code,size_code,ordered_qty,upc\nPO-DEMO-01,STYLE-100,NAVY,M,250,\nPO-DEMO-01,STYLE-100,NAVY,L,350,\n',bom:'style_code,material_sku,category,qty_per_garment,uom\nSTYLE-100,COTTON-TWILL,FABRIC,1.850,M\nSTYLE-100,ZIP-18CM,TRIM,1.000,PCS\n'};
document.querySelectorAll('[data-template]').forEach(btn=>btn.addEventListener('click',()=>{const key=btn.dataset.template;const url=URL.createObjectURL(new Blob([templates[key]],{type:'text/csv;charset=utf-8'}));const a=document.createElement('a');a.href=url;a.download=`agiparel_${key}_template.csv`;a.click();URL.revokeObjectURL(url)}));
async function refresh(){const history=$('history');try{const result=await response(await fetch('/api/v1/order-intake',{credentials:'same-origin'}));history.replaceChildren();if(!result.items.length){history.textContent='No staged intakes for this operator tenant.';return}for(const item of result.items){const row=add('button','',history);row.type='button';const title=add('strong',`${item.po_number} · ${item.style_code} · ${format(item.ordered_qty)} pcs`,row);const right=add('small',`${item.status} · ${new Date(item.captured_at).toLocaleString()}`,row);row.addEventListener('click',()=>void inspect(item.id))}}catch(e){history.textContent=`Cannot load staged intakes: ${e.message}`}}
async function inspect(id){try{show(await response(await fetch(`/api/v1/order-intake?id=${encodeURIComponent(id)}`,{credentials:'same-origin'})));alert(`Loaded ${id} from PostgreSQL. Source values remain PROPOSED.`,'success')}catch(e){alert(e.message,'error')}}
$('refresh').addEventListener('click',()=>void refresh());void refresh();
})();
