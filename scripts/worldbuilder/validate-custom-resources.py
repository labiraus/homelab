#!/usr/bin/env python3
"""Check every rendered custom resource against explicit local CRD schemas.
No cluster access. Kubernetes built-ins and admission/CEL rules are outside this check.
Usage: python validate-custom-resources.py SCHEMA_DIR RENDER_DIR
"""
import copy
import pathlib
import sys
import yaml
import jsonschema


class KubernetesLoader(yaml.SafeLoader):
    """Keep the legal bare '=' string used by upstream CRD enum schemas."""


KubernetesLoader.add_constructor('tag:yaml.org,2002:value', KubernetesLoader.construct_scalar)


def documents(text):
    return yaml.load_all(text, Loader=KubernetesLoader)


def structural(schema):
    schema = copy.deepcopy(schema)
    if isinstance(schema, dict):
        # Kubernetes structural schemas allow IntOrString and nullable fields.
        if schema.get('x-kubernetes-int-or-string'):
            schema['type'] = ['integer', 'string']
        elif schema.get('nullable') and isinstance(schema.get('type'), str):
            schema['type'] = [schema['type'], 'null']
        for key, value in list(schema.items()):
            schema[key] = structural(value)
    elif isinstance(schema, list):
        schema = [structural(value) for value in schema]
    return schema


def main(schema_dir, render_dir):
    schemas = {}
    for path in pathlib.Path(schema_dir).glob('*.yaml'):
        for doc in documents(path.read_text()):
            if isinstance(doc, dict) and doc.get('kind') == 'CustomResourceDefinition':
                for version in doc['spec']['versions']:
                    schemas[(doc['spec']['group']+'/'+version['name'], doc['spec']['names']['kind'])] = structural(version['schema']['openAPIV3Schema'])
    count = 0
    failures = []
    paths = list(pathlib.Path(render_dir).glob('*.template')) + list(pathlib.Path(render_dir).glob('*-enabled.yaml'))
    builtin = {'v1', 'apps/v1', 'batch/v1', 'networking.k8s.io/v1', 'rbac.authorization.k8s.io/v1', 'policy/v1', 'autoscaling/v2'}
    for path in paths:
        for doc in documents(path.read_text()):
            if not isinstance(doc, dict) or doc.get('apiVersion') in builtin:
                continue
            key = (doc.get('apiVersion'), doc.get('kind'))
            schema = schemas.get(key)
            if schema is None:
                failures.append(f'{path.name}: missing schema {key}')
                continue
            try:
                jsonschema.Draft7Validator(schema).validate(doc)
                count += 1
            except jsonschema.ValidationError as error:
                failures.append(f'{path.name}: {key} at {list(error.path)}: {error.message}')
    print(f'Checked {count} custom resources; built-ins and admission/CEL validation excluded.')
    for failure in failures:
        print(failure)
    return 1 if failures or not count else 0


if __name__ == '__main__':
    sys.exit(main(*sys.argv[1:]))
