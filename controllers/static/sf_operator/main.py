# Copyright (C) 2022 Red Hat
# SPDX-License-Identifier: Apache-2.0

import argparse
from pathlib import Path
import sys
import sf_operator.secret

import pynotedb


def mk_incluster_k8s_config():
    sa = Path("/run/secrets/kubernetes.io/serviceaccount")

    def add(n):
        return (n, (sa / n).read_text())

    return [add("ca.crt"), add("namespace"), add("token")]


def create_k8s_secret():
    clone = pynotedb.mk_clone("git://git-server/system-config")
    k8s_secret = clone / "zuul.d" / "k8s-secret.yaml"
    if not k8s_secret.exists():
        secret = sf_operator.secret.mk_secret("k8s_config", mk_incluster_k8s_config())
        k8s_secret.write_text(secret)
        pynotedb.git(clone, ["add", "zuul.d/k8s-secret.yaml"])
        pynotedb.commit_and_push(clone, "Add kubernetes secret", "refs/heads/master")


def main():
    parser = argparse.ArgumentParser(description="notedb-tools")
    parser.add_argument("action", choices=["config-create-k8s-secret"])
    args = parser.parse_args()

    if args.action == "config-create-k8s-secret":
        create_k8s_secret()


if __name__ == "__main__":
    main()
