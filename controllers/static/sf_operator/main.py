# Copyright (C) 2022 Red Hat
# SPDX-License-Identifier: Apache-2.0

import argparse
from pathlib import Path
import os
import sf_operator.secret

import pynotedb


def ensure_git_config():
    os.environ.setdefault("HOME", str(Path("~/").expanduser()))
    if any(
        map(
            lambda p: p.expanduser().exists(),
            [Path("~/.gitconfig"), Path("~/.config/git/config")],
        )
    ):
        return
    pynotedb.execute(
        ["git", "config", "--global", "user.email", "admin@" + os.environ["FQDN"]]
    )
    pynotedb.execute(
        ["git", "config", "--global", "user.name", "SoftwareFactory Admin"]
    )


def sshkey_scan(port: str, hostname: str) -> bytes:
    return pynotedb.pread(
        ["ssh-keyscan", "-T", "10", "-p", port, hostname])


def get_logserver_fingerprint() -> str:
    return " ".join(sshkey_scan("2222", "logserver").decode().split()[1:])


def mk_incluster_k8s_config():
    sa = Path("/run/secrets/kubernetes.io/serviceaccount")

    def add(n):
        return (n, (sa / n).read_text())

    api = os.environ.get("KUBERNETES_PUBLIC_API_URL")
    if not api:
        api = (
            "https://"
            + os.environ["KUBERNETES_SERVICE_HOST"]
            + ":"
            + os.environ["KUBERNETES_SERVICE_PORT"]
        )

    return [
        add("ca.crt"),
        add("namespace"),
        ("token", os.environ["SERVICE_ACCOUNT_TOKEN"]),
        ("server", api)]


def create_zuul_secrets():
    clone = pynotedb.mk_clone("git://git-server-rw:9419/system-config")
    # FQDN to access the logserver
    logserver_host = os.environ.get("ZUUL_LOGSERVER_HOST", "logserver")
    logserver_fqdn = "[%s]:2222" % logserver_host
    # K8s secret
    k8s_secret = clone / "zuul.d" / "k8s-secret.yaml"
    secret = sf_operator.secret.mk_secret(
        "k8s_config", mk_incluster_k8s_config())
    k8s_secret.write_text(secret)
    # log server secret
    logserver_secret = clone / "zuul.d" / "sf-logserver-secret.yaml"
    secret = sf_operator.secret.mk_secret(
        "site_sflogs",
        items=[
            ("ssh_private_key", os.environ["ZUUL_LOGSERVER_PRIVATE_KEY"])
        ],
        unencrypted_items=[
            ("fqdn", "\"" + logserver_fqdn + "\""),
            ("path", "rsync"),
            ("ssh_known_hosts", "\"%s %s\"" %
             (logserver_fqdn, get_logserver_fingerprint())),
            ("ssh_username", "zuul")
        ]
    )
    logserver_secret.write_text(secret)
    pynotedb.git(
        clone,
        ["add",
         "zuul.d/k8s-secret.yaml",
         "zuul.d/sf-logserver-secret.yaml"])
    pynotedb.commit_and_push(
        clone, "Update internal Zuul secrets", "refs/heads/master")


def main():
    parser = argparse.ArgumentParser(description="notedb-tools")
    parser.add_argument("action", choices=["config-create-zuul-secrets"])
    args = parser.parse_args()

    ensure_git_config()

    if args.action == "config-create-zuul-secrets":
        create_zuul_secrets()


if __name__ == "__main__":
    main()
