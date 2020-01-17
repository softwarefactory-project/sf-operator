#!/bin/env python3
#
# Copyright 2019 Red Hat
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

import math
import socket

from ansible.module_utils.basic import AnsibleModule


def gearman_status(host):
    skt = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    skt.connect((host, 4730))
    skt.send(b"status\n")
    status = {}
    while True:
        data = skt.recv(4096)
        for line in data.split(b"\n"):
            if line == b".":
                skt.close()
                return status
            if line == b"":
                continue
            name, queue, running, worker = line.decode('ascii').split()
            status[name] = {
                "queue": int(queue),
                "running": int(running),
                "worker": int(worker),
            }
    skt.close()
    return status


def ansible_main():
    module = AnsibleModule(
        argument_spec=dict(
            service=dict(required=True),
            gearman=dict(required=True),
            min=dict(required=True, type='int'),
            max=dict(required=True, type='int'),
        )
    )

    try:
        status = gearman_status(module.params.get('gearman'))
    except Exception as e:
        module.fail_json(msg="Couldn't get gearman status: %s" % e)

    service = module.params.get('service')
    scale_min = module.params.get('min')
    scale_max = module.params.get('max')

    count = 0
    if service == "merger":
        jobs = 0
        for job in status:
            if job.startswith("merger:"):
                stat = status[job]
                jobs += stat["queue"] + stat["running"]
        count = math.ceil(jobs / 5)
    elif service == "executor":
        stat = status.get("executor:execute")
        if stat:
            count = math.ceil((stat["queue"] + stat["running"]) / 10)

    module.exit_json(
        changed=False, count=int(min(max(count, scale_min), scale_max)))


if __name__ == '__main__':
    ansible_main()
