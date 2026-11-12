#!/usr/bin/env python3
# Author: Sunil Jain, 2026-11-12.
# Deterministically archive the v50 pilot deployment contract with a SHA-256 manifest.
import hashlib
import io
import json
import sys
import zipfile
from pathlib import Path

root = Path(__file__).resolve().parent.parent
out = Path(sys.argv[1]).resolve() if len(sys.argv) > 1 else (root / 'dist' / 'agiparel-factory-v50-pilot-source.zip')
paths = [
    'compose/docker-compose.factory-v50.yml',
    'scripts/factory_pilot_v50.sh',
    'scripts/test_factory_v50_contract.cjs',
    'docs/FACTORY_EDGE_V50_RELEASE_CANDIDATE.md',
]
manifest = {'kind':'source-only-pilot-installer','version':'v50-rc','author':'Sunil Jain',
            'scope':'local evaluation only; no shipped Aerolake/Ollama images',
            'files':{p:hashlib.sha256((root/p).read_bytes()).hexdigest() for p in paths}}
out.parent.mkdir(parents=True,exist_ok=True)
with zipfile.ZipFile(out,'w',compression=zipfile.ZIP_DEFLATED,compresslevel=9) as archive:
    for p in paths + ['manifest.json']:
        payload = json.dumps(manifest,indent=2,sort_keys=True).encode() if p=='manifest.json' else (root/p).read_bytes()
        info = zipfile.ZipInfo(p, date_time=(2026,11,12,0,0,0))
        info.external_attr = (0o755 if p.endswith('.sh') else 0o644) << 16
        info.compress_type=zipfile.ZIP_DEFLATED
        archive.writestr(info,payload,compress_type=zipfile.ZIP_DEFLATED,compresslevel=9)
print(f'PASS: reproducible source-only pilot bundle {out} sha256={hashlib.sha256(out.read_bytes()).hexdigest()}')
