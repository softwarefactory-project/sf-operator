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

import json
import time
from typing import Any
import gear  # type: ignore


def connect(host : str) -> Any:
    client = gear.Client()
    client.addServer(host, 4730, 'client.key', 'client.pem', 'ca.pem')
    client.waitForServer(timeout=10)
    return client


def run(client : Any, job_name : str, args : Any = dict()) -> Any:
    job = gear.Job(job_name.encode('utf-8'), json.dumps(args).encode('utf-8'))
    client.submitJob(job, timeout=300)
    while not job.complete:
        time.sleep(0.1)
    return json.loads(job.data[0])


if __name__ == '__main__':
    print(run(connect("scheduler"), "status"))
