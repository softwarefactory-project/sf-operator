#!/usr/bin/env python3
# Copyright 2020 Red Hat
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

from ansible.module_utils.basic import AnsibleModule
from ansible.module_utils import gearlib


def gearman_dump():
    client = gearlib.connect("scheduler")
    queues = dict()
    for tenant in gearlib.run(client, "zuul:tenant_list"):
        name = tenant['name']
        queues[name] = gearlib.run(client, "zuul:status_get", {"tenant": name})
    return queues


def ansible_main():
    module = AnsibleModule(
        argument_spec=dict()
    )

    try:
        module.exit_json(changed=False, changes=gearman_dump())
    except Exception as e:
        module.fail_json(msg="Couldn't get gearman status: %s" % e)


if __name__ == '__main__':
    ansible_main()
