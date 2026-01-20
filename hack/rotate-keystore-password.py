#!/usr/bin/env python3
# Copyright (C) 2022 Red Hat
# SPDX-License-Identifier: Apache-2.0

"""
This script rotates the keystore password by re-encrypting every data in ZooKeeper.

Usage: to perform the rotation, run the following command with a valid ~/.kube/config pointing at the cluster namespace.

  podman run -it --security-opt label=disable --volume $(pwd)/hack:/hack --volume $HOME/.kube:/root/.kube quay.io/software-factory/zuul-executor:13.1.0-20250925-1 /hack/rotate-keystore-password.py

Then rollout the deployment with:

  go run ./main.go deploy <path-to-sf-cr.yaml>

Note that after rotating the secrets, the sf-operator reconcile loop must complete in order for the change to take effect.
"""

import uuid
import configparser
import base64
import time
import os
import json

from kubernetes import client, config

from zuul.lib.keystorage import KeyStorage
from zuul.lib import encryption
from zuul.zk import ZooKeeperClient

config.load_kube_config()
ns = config.list_kube_config_contexts()[1]["context"]["namespace"]
v1 = client.CoreV1Api()


def mk_secret(name, password):
    sec = client.V1Secret()
    sec.metadata = client.V1ObjectMeta(name=name)
    sec.type = "Opaque"
    sec.data = {
        "zuul-keystore-password": base64.b64encode(password.encode("utf-8")).decode()
    }
    return sec


def get_zuul_conf():
    return base64.b64decode(
        v1.read_namespaced_secret("zuul-config", ns).data["zuul.conf"]
    ).decode("utf-8")


def setup_zk_tls():
    os.makedirs("/tls/client", exist_ok=True)
    secret = v1.read_namespaced_secret("zookeeper-client-tls", ns)
    for fp in ("ca.crt", "tls.crt", "tls.key"):
        open("/tls/client/" + fp, "w").write(
            base64.b64decode(secret.data[fp]).decode("utf-8")
        )


def ensure_portforward():
    import subprocess

    # It's fine if it is already running
    subprocess.Popen(["kubectl", "port-forward", "pod/zookeeper-0", "2281:2281"])
    time.sleep(1)


def encrypt_keys(keys, old_password, new_password):
    new_keys = dict()
    for path, obj in keys.items():
        new_keys[path] = dict(keys=[])
        for key in obj["keys"]:
            pem_private_key = key.get("private_key").encode("utf-8")
            private_key, public_key = encryption.deserialize_rsa_keypair(
                pem_private_key, old_password
            )
            encrypted_private_key = encryption.serialize_rsa_private_key(
                private_key, new_password
            )
            new_key = key.copy()
            new_key["private_key"] = encrypted_private_key.decode("utf-8")
            new_keys[path]["keys"].append(new_key)
    return new_keys


def main():
    ensure_portforward()

    print("[+] load zuul config")
    config = configparser.ConfigParser()
    config.read_string(get_zuul_conf())

    print("[+] use the local port-forwarded endpoint")
    config["zookeeper"]["hosts"] = "localhost:2281"
    setup_zk_tls()

    print("[+] connect to ZooKeeper")
    zk_client = ZooKeeperClient.fromConfig(config)
    zk_client.connect()
    ks = KeyStorage(zk_client, "unused")

    print("[+] read keys")
    old_keys = ks.exportKeys()
    with open("/hack/keys-backup.json", "w") as f:
        json.dump(old_keys, f)
    old_password = config["keystore"]["password"]

    print("[+] re-encrypt with new password")
    new_password = str(uuid.uuid4())
    new_keys = dict(
        keys=encrypt_keys(
            old_keys["keys"], old_password.encode("utf-8"), new_password.encode("utf-8")
        )
    )

    print(
        "[+] Keep a copy of the password in case there is a problem when loading the new keys"
    )
    try:
        v1.create_namespaced_secret(
            body=mk_secret("zuul-keystore-password-backup", old_password), namespace=ns
        )
    except Exception as e:
        print(e)
        print("Make sure to kubectl delete secret zuul-keystore-password-backup")
        exit(1)
    v1.replace_namespaced_secret(
        name="zuul-keystore-password",
        body=mk_secret("zuul-keystore-password", new_password),
        namespace=ns,
    )
    ks.importKeys(new_keys, True)
    print("[+] All done!")


if __name__ == "__main__":
    main()
