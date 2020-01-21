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

import time
from ansible.module_utils.basic import AnsibleModule
from ansible.module_utils import gearlib


def gearman_load(changes):
    for retry in range(120):
        try:
            client = gearlib.connect("scheduler")
        except Exception:
            time.sleep(1)
    for tenant, status in changes.items():
        for pipeline in status['pipelines']:
            for queue in pipeline['change_queues']:
                for head in queue['heads']:
                    for change in head:
                        if (not change['live'] or
                                not change.get('id') or
                                ',' not in change['id']):
                            continue
                        cid, cps = change['id'].split(',')
                        gearlib.run(client, "zuul:enqueue", dict(
                            tenant=tenant,
                            pipeline=pipeline['name'],
                            project=change['project_canonical'],
                            trigger='gerrit',
                            change=cid + ',' + cps
                        ))


def ansible_main():
    module = AnsibleModule(
        argument_spec=dict(
            changes=dict(required=True)
        )
    )

    try:
        module.exit_json(changed=False, changes=gearman_load(module.params['changes']))
    except Exception as e:
        module.fail_json(msg="Couldn't get gearman status: %s" % e)


if __name__ == '__main__':
    ansible_main()
