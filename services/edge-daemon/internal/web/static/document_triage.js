// Source-supported routing suggestions, not confirmation of document meaning.
(() => {
 'use strict';
 const host=document.getElementById('document-triage');if(!host)return;
 let sequence=0;
 document.addEventListener('agiparel-document-selected',async ev=>{
  const id=ev.detail?.id;if(!id)return;const seq=++sequence;
  host.replaceChildren();const heading=document.createElement('strong');heading.textContent='Possible document purpose';host.append(heading);
  const message=document.createElement('p');message.textContent='Examining source text for useful clues…';host.append(message);
  try {
   const response=await fetch(`/api/v1/document-triage?document_id=${encodeURIComponent(id)}`,{credentials:'same-origin',headers:{Accept:'application/json'}});
   if(!response.ok)throw Error(`HTTP ${response.status}`);
   const data=await response.json();if(seq!==sequence)return;message.remove();
   const hints=data.possible_categories||[];
   if(!hints.length){const p=document.createElement('p');p.textContent=data.source_status==='QUEUED'||data.source_status==='PROCESSING'?'Extraction is not complete. Check back after processing.':'No category could be determined confidently from extracted text. You can still review the original and tell Agiparel about the factory manually.';host.append(p);return;}
   const ul=document.createElement('ul');
   for(const hint of hints){const li=document.createElement('li');const link=document.createElement('a');link.href=`#source-segment-${hint.source_ordinal}`;link.textContent=`Possible ${hint.label} · source segment ${hint.source_ordinal}`;li.append(link);ul.append(li);}
   host.append(ul);const caption=document.createElement('p');caption.textContent='These are navigation suggestions—not verified document classifications or extracted business records.';host.append(caption);
  }catch(err){if(seq!==sequence)return;message.textContent=`Cannot examine document categories: ${err.message}`;}
 });
})();
