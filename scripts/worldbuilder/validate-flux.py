#!/usr/bin/env python3
"""Validate rendered Flux resources against locally exported CRD schemas.
Usage: python validate-flux.py /tmp/flux-export.yaml /tmp/worldbuilder-manifests-...
Requires PyYAML/jsonschema. `flux install --export` is local-only.
"""
import pathlib,sys,yaml,jsonschema
schemas={}
for doc in yaml.safe_load_all(pathlib.Path(sys.argv[1]).read_text()):
 if doc and doc.get('kind')=='CustomResourceDefinition':
  for v in doc['spec']['versions']:
   schemas[(doc['spec']['group']+'/'+v['name'],doc['spec']['names']['kind'])]=v['schema']['openAPIV3Schema']
count=0
for path in pathlib.Path(sys.argv[2]).glob('*.template'):
 for doc in yaml.safe_load_all(path.read_text()):
  if not doc:continue
  schema=schemas.get((doc.get('apiVersion'),doc.get('kind')))
  if schema:
   jsonschema.Draft7Validator(schema).validate(doc);count+=1
print(f'PASS: {count} rendered Flux objects validated against installed Flux CRDs. Other CRDs are not covered.')
