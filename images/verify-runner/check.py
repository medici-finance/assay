"""Release-owned Python capability probe; no repository inputs or network needed."""
import importlib.metadata
import json
import platform

import jsonschema
import yaml

schema = {
    '$schema': 'https://json-schema.org/draft/2020-12/schema',
    'type': 'object',
    'required': ['nodes'],
    'properties': {'nodes': {'type': 'array', 'items': {'type': 'string'}}},
    'additionalProperties': False,
}
jsonschema.Draft202012Validator.check_schema(schema)
validator = jsonschema.Draft202012Validator(schema)
validator.validate(yaml.safe_load('nodes: [example]'))
try:
    validator.validate(yaml.safe_load('nodes: [42]'))
except jsonschema.ValidationError:
    pass
else:
    raise RuntimeError('invalid schema instance was accepted')
print(json.dumps({'python': platform.python_version(),
                  'PyYAML': importlib.metadata.version('PyYAML'),
                  'jsonschema': importlib.metadata.version('jsonschema')}, sort_keys=True))
