#!/usr/bin/env python3
# Copyright (C) 2026 Red Hat
# SPDX-License-Identifier: Apache-2.0

"""
A script to provide access to ZooKeeper through localhost.
Usage:

sudo mkdir -p /etc/zuul /var/lib/zuul
sudo chown $USER /etc/zuul /var/lib/zuul
python3 setup-kazoo.py
"""

import os, base64, subprocess
from kubernetes import client, config

config.load_kube_config()
v1 = client.CoreV1Api()
ns = config.list_kube_config_contexts()[1]["context"]["namespace"]


def setup_zuul_conf():
    os.makedirs("/etc/zuul", exist_ok=True)
    open("/etc/zuul/zuul.conf", "w").write(
        base64.b64decode(
            v1.read_namespaced_secret("zuul-config", ns).data["zuul.conf"]
        ).decode("utf-8")
    )


def setup_zk_tls():
    os.makedirs("/tls/client", exist_ok=True)
    secret = v1.read_namespaced_secret("zookeeper-client-tls", ns)
    for fp in ("ca.crt", "tls.crt", "tls.key"):
        open("/tls/client/" + fp, "w").write(
            base64.b64decode(secret.data[fp]).decode("utf-8")
        )


def ensure_portforward():
    # It's fine if it is already running
    subprocess.Popen(["kubectl", "port-forward", "pod/zookeeper-0", "2281:2281"])


def read_tenant_config():
    os.makedirs("/var/lib/zuul", exist_ok=True)
    subprocess.Popen(
        [
            "kubectl",
            "cp",
            "zuul-scheduler-0:/var/lib/zuul/main.yaml",
            "/var/lib/zuul/main.yaml",
        ]
    ).wait()


setup_zuul_conf()
setup_zk_tls()
read_tenant_config()
ensure_portforward()
