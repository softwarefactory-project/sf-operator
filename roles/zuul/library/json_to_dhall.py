#!/usr/bin/env python3
# Copyright 2020 Red Hat, Inc
#
# Licensed under the Apache License, Version 2.0 (the "License"); you may
# not use this file except in compliance with the License. You may obtain
# a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
# WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
# License for the specific language governing permissions and limitations
# under the License.

import argparse
import subprocess
import sys
from typing import List
from ansible.module_utils.basic import AnsibleModule  # type: ignore


def pread(args: List[str], stdin: str) -> str:
    proc = subprocess.Popen(args, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    stdout, stderr = proc.communicate(stdin.encode('utf-8'))
    if stderr:
        raise RuntimeError("Command failed: " + stderr.decode('utf-8'))
    return stdout.decode('utf-8')


def run(schema: str, json_input: str) -> str:
    return pread(['json-to-dhall', '--plain', schema], json_input)


def ansible_main():
    module = AnsibleModule(
        argument_spec=dict(
            schema=dict(required=True, type='str'),
            json=dict(required=True, type='str'),
        )
    )
    p = module.params
    try:
        module.exit_json(changed=True, result=run(p['schema'], p['json']))
    except Exception as e:
        module.fail_json(msg="Dhall expression failed:" + str(e))


def cli_main():
    parser = argparse.ArgumentParser()
    parser.add_argument('schema')
    parser.add_argument('--json')
    parser.add_argument('--file')
    args = parser.parse_args()
    if args.file:
        import yaml, json
        args.json = json.dumps(yaml.safe_load(open(args.file)))
    print(run(args.schema, args.json))


if __name__ == '__main__':
    if sys.stdin.isatty():
        cli_main()
    else:
        ansible_main()
