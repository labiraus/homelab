#!/usr/bin/env python3
import contextlib
import importlib.util
import io
import pathlib
import tempfile
import unittest
import yaml

spec=importlib.util.spec_from_file_location('validator',pathlib.Path(__file__).with_name('validate-custom-resources.py'))
module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
class ResourceValidation(unittest.TestCase):
    def test_upstream_equality_enum_remains_a_string(self):
        schema=next(module.documents('type: string\nenum:\n  - =\n  - "!="\n'))
        self.assertEqual(schema['enum'],['=','!='])
        module.jsonschema.Draft7Validator(schema).validate('=')
        with self.assertRaises(module.jsonschema.ValidationError):
            module.jsonschema.Draft7Validator(schema).validate('unlisted')
        with self.assertRaises(yaml.constructor.ConstructorError):
            list(module.documents('!!python/object:unsafe {}'))

    def test_missing_and_invalid_resources_fail_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            root=pathlib.Path(directory);schemas=root/'schemas';renders=root/'renders';schemas.mkdir();renders.mkdir()
            resource={'apiVersion':'test.example/v1','kind':'Fixture','spec':{'replicas':2}}
            path=renders/'fixture.template';path.write_text(yaml.safe_dump(resource))
            with contextlib.redirect_stdout(io.StringIO()):self.assertEqual(module.main(schemas,renders),1)
            crd={'kind':'CustomResourceDefinition','spec':{'group':'test.example','names':{'kind':'Fixture'},'versions':[{'name':'v1','schema':{'openAPIV3Schema':{'type':'object','properties':{'spec':{'type':'object','required':['replicas'],'properties':{'replicas':{'type':'integer','minimum':1}}}}}}}]}}
            (schemas/'test.yaml').write_text(yaml.safe_dump(crd))
            with contextlib.redirect_stdout(io.StringIO()):self.assertEqual(module.main(schemas,renders),0)
            resource['spec']['replicas']='invalid';path.write_text(yaml.safe_dump(resource))
            with contextlib.redirect_stdout(io.StringIO()):self.assertEqual(module.main(schemas,renders),1)
if __name__=='__main__':unittest.main()
