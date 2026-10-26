(()=>{'use strict';
const $=id=>document.getElementById(id);
const add=(tag,text,parent,cls)=>{const node=document.createElement(tag);node.textContent=text??'';if(cls)node.className=cls;parent.append(node);return node};
const message=(text,cls='')=>{const e=$('message');e.textContent=text;e.className='message '+cls};
async function json(res){if(!res.ok)throw new Error((await res.text()).trim().slice(0,300)||`HTTP ${res.status}`);return res.json()}
const fetchData=url=>fetch(url,{credentials:'same-origin'}).then(json);
const clean=x=>String(x??'').replace(/\r|\n|,/g,'_');
let selected=null,preview=null;
function download(name,content){const url=URL.createObjectURL(new Blob([content],{type:'text/csv;charset=utf-8'}));const a=document.createElement('a');a.href=url;a.download=name;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000)}
function stockTemplate(){if(!selected){message('Choose a staged intake first.','error');return}
 const headers='lot_id,material_sku,uom,available_qty,fabric_spec,required_spec,gsm,required_gsm,gsm_tolerance_pct,shade_delta_e,shade_max_delta_e,usable_width_mm,marker_width_mm,inspection_status,age_days,max_age_days,buyer_escrow,held_for_po,source_reference';
 const rows=[headers];
 for(const [i,m] of selected.materials.filter(m=>m.category==='FABRIC').entries()){
  // Examples are NOT measured stock. Replace every submitted compatibility statement with verified sources.
  rows.push([`EXAMPLE-LOT-${i+1}`,m.material_sku,m.uom,'500.000','EXAMPLE-SPEC','EXAMPLE-SPEC','220.000','220.000','3.000','0.500','1.000','1650.000','1600.000','ACCEPTED_PASS','60','365','ALLOWED','','EXAMPLE-GRN-REPLACE'].map(clean).join(','));
 }
 download('agiparel_example_surplus_unverified.csv',rows.join('\n')+'\n');
}
function priceTemplate(){if(!selected){message('Choose a staged intake first.','error');return}
 const rows=['material_sku,category,uom,unit_price,currency,procurement_owner,source_reference'];
 for(const m of selected.materials)rows.push([m.material_sku,m.category,m.uom,m.category==='FABRIC'?'4.2500':'0.3300','USD','FACTORY','EXAMPLE-QUOTE-REPLACE'].map(clean).join(','));
 download('agiparel_example_prices_unverified.csv',rows.join('\n')+'\n')
}
function show(rep){
 $('scenario-title').textContent=`${rep.po_number} · ${rep.style_code}`;
 $('scenario-subtitle').textContent=`${Number(rep.ordered_qty).toLocaleString()} ordered garments · ${rep.evidence_class} · status ${rep.status}`;
 $('gate').textContent=rep.release_gate;
 $('gross-cost').textContent=rep.gross_material_cost;
 $('reuse-cost').textContent=rep.conditional_reuse_value;
 $('net-cost').textContent=rep.net_purchase_exposure;
 $('currency').textContent=rep.currency+' · CSV';
 $('hash').textContent=rep.scenario_sha256;
 const body=$('material-lines');body.replaceChildren();for(const m of rep.materials){const tr=add('tr','',body);for(const text of [`${m.material_sku} · ${m.category} · ${m.procurement_owner}`,`${m.gross} ${m.uom}`,`${m.conditional_surplus} ${m.uom}`,`${m.net_indent} ${m.uom}`,`${m.buyer_supply_gap} ${m.uom}`,`${m.unit_price} ${rep.currency}`,`${m.net_cost} ${rep.currency}`])add('td',text,tr)}
 const cand=$('candidates');cand.replaceChildren();for(const c of rep.candidates){const tr=add('tr','',cand);const lot=add('td',c.lot_id,tr);add('span',c.source_reference,lot,'lot-source');add('td',c.material_sku,tr);add('td',c.submitted_qty,tr);const applied=add('td',c.suggested_qty,tr);add('span',c.verdict.replaceAll('_',' ').toLowerCase(),applied,'lot-source');const checks=add('td','',tr);const badges=add('div','',checks,'checks');for(const ck of c.checks){const badge=add('span',ck.criterion.replaceAll('_',' '),badges,ck.pass?'check':'check fail');badge.title=ck.detail}}
 const holds=$('holds');holds.replaceChildren();for(const h of rep.holds)add('li',h,holds);
}
async function loadOrders(){const list=await fetchData('/api/v1/order-intake');const el=$('intake');el.replaceChildren();const opt=add('option','Choose a staged buyer order',el);opt.value='';for(const item of list.items){const o=add('option',`${item.po_number} · ${item.style_code} · ${Number(item.ordered_qty).toLocaleString()} pieces`,el);o.value=item.id}
 const wanted=new URLSearchParams(location.search).get('intake');if(wanted&&list.items.some(item=>item.id===wanted))el.value=wanted;
 if(el.value)await chooseOrder();else message('Select a staged order. Upload or stage one first on the Order Intake screen.');
}
async function chooseOrder(){preview=null;$('stage').disabled=true;selected=null;const id=$('intake').value;
 if(!id){$('order-summary').textContent='Select a staged order first.';$('history').textContent='No order selected.';return}
 try{selected=await fetchData('/api/v1/order-intake?id='+encodeURIComponent(id));$('order-summary').textContent=`${selected.ordered_qty.toLocaleString()} pieces · ${selected.materials.length} BOM lines · original intake SHA ${selected.source_sha256.slice(0,14)}…`;
 message('Order loaded from PostgreSQL. Download illustrative CSV inputs, then replace claims and prices with your evidence before assessing.');await refresh()}
 catch(err){message(err.message,'error')}
}
async function evaluate(action){const id=$('intake').value,stock=$('stock').files[0],prices=$('prices').files[0];if(!selected||!id||!stock||!prices){message('Select a staged order and both source CSV files.','error');return}
 const form=new FormData();form.append('intake_id',id);form.append('stock_file',stock);form.append('prices_file',prices);
 $('preview').disabled=true;$('stage').disabled=true;
 try{message(action==='preview'?'Evaluating six criteria and gross-to-net material economics…':'Saving immutable scenario sources, audit and calculation…');const result=await json(await fetch('/api/v1/material-scenario?action='+action,{method:'POST',credentials:'same-origin',body:form}));show(result);
  if(action==='preview'){preview={intake:id,stock,prices,digest:result.scenario_sha256};$('stage').disabled=false;message('Evaluation completed. These are CONDITIONAL calculations from submitted CSV assertions; no inventory was reserved.','success')}
  else{preview=null;message(`Saved immutable proposed scenario ${result.id}. All commitments remain on HOLD.`,'success');await refresh()}
 }catch(err){message(err.message,'error')}finally{$('preview').disabled=false}
}
async function refresh(){if(!$('intake').value){$('history').textContent='Select a staged order first.';return}try{const list=await fetchData('/api/v1/material-scenario?intake='+encodeURIComponent($('intake').value));const hist=$('history');hist.replaceChildren();if(!list.items.length){hist.textContent='No saved scenarios for this buyer order.';return}for(const item of list.items){const btn=add('button','',hist);btn.type='button';add('strong',item.id,btn);add('small',new Date(item.created_at).toLocaleString()+' · PROPOSED · HOLD',btn);btn.addEventListener('click',async()=>{try{const rep=await fetchData('/api/v1/material-scenario?id='+encodeURIComponent(item.id));show(rep);message(`Loaded immutable ${rep.id} from PostgreSQL.`, 'success')}catch(e){message(e.message,'error')}})}}catch(e){$('history').textContent='Cannot load scenario history: '+e.message}}
$('scenario-form').addEventListener('submit',e=>{e.preventDefault();void evaluate('preview')});
$('stage').addEventListener('click',()=>{if(!preview||preview.intake!==$('intake').value||preview.stock!==$('stock').files[0]||preview.prices!==$('prices').files[0]){message('Scenario inputs changed; preview again.','error');return}void evaluate('stage')});
$('intake').addEventListener('change',()=>void chooseOrder());
for(const name of ['stock','prices'])$(name).addEventListener('change',()=>{preview=null;$('stage').disabled=true;message('Inputs changed. Re-evaluate before saving.');});
$('template-stock').addEventListener('click',stockTemplate);$('template-prices').addEventListener('click',priceTemplate);$('refresh').addEventListener('click',()=>void refresh());
void loadOrders().catch(e=>message('Cannot load staged orders: '+e.message,'error'));
})();
