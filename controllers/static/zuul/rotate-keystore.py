#!/usr/bin/env python3
# Copyright (C) 2026 Red Hat
# SPDX-License-Identifier: Apache-2.0

import configparser
import sys
import json

from zuul.lib.keystorage import KeyStorage
from zuul.lib import encryption
from zuul.zk import ZooKeeperClient

try:
    old_password = sys.argv[1].encode("utf-8")
    new_password = sys.argv[2].encode("utf-8")
except IndexError:
    print("usage: rotate-keystore old-password new-password")
    sys.exit(1)


def encrypt_keys(keys):
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


# Connect to ZooKeeper
config = configparser.ConfigParser()
config.read("/etc/zuul/zuul.conf")
zk_client = ZooKeeperClient.fromConfig(config)
zk_client.connect()

# Read the keys
ks = KeyStorage(zk_client, "unused")
old_keys = ks.exportKeys()
with open("/var/lib/zuul/keys-backup.json", "w") as f:
    json.dump(old_keys, f)

# Write the new keys
new_keys = dict(keys=encrypt_keys(old_keys["keys"]))
ks.importKeys(new_keys, True)
