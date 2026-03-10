#!/usr/bin/env python3
# Copyright (C) 2026 Red Hat
# SPDX-License-Identifier: Apache-2.0

"""
This script is meant to be used by the sf-operator rotate-projects-private-keys command line.
"""

import itertools
import base64
import textwrap
import zuul.lib.yamlutil as yaml
from zuul.lib import encryption
from pathlib import Path


class ProjectKey:
    "A new project key for inrepo secrets"

    def __init__(self):
        import time

        self.created = int(time.time())
        self.priv, self.pub = encryption.generate_rsa_keypair()

    def export(self, password):
        "Create the ZKNode to be stored in ZooKeeper"
        import json

        return json.dumps(
            dict(
                schema=1,
                keys=[
                    dict(
                        version=0,
                        created=self.created,
                        private_key=encryption.serialize_rsa_private_key(
                            self.priv, password
                        ).decode("utf-8"),
                    )
                ],
            )
        ).encode("utf-8")


class PKCS(yaml.EncryptedPKCS1_OAEP):
    "Helper to handle encrypted/pkcs1-oaep block"

    def __init__(self, indent, raw):
        "Decode the raw value."
        self.indent = indent
        dat = yaml.safe_load(raw)
        if isinstance(dat, list):
            self.ciphertext = [base64.b64decode(x) for x in dat]
        else:
            self.ciphertext = [base64.b64decode(dat)]

    def encrypt(self, plaintext, public_key):
        "Replace the ciphertext with a new plaintext to be encrypted with the public_key."
        import math

        nbytes = int(public_key.key_size / 8)
        max_bytes = nbytes - 42  # PKCS1-OAEP overhead
        chunks = int(math.ceil(float(len(plaintext)) / max_bytes))
        self.ciphertext = []
        for count in range(chunks):
            chunk = plaintext[int(count * max_bytes) : int((count + 1) * max_bytes)]
            self.ciphertext.append(encryption.encrypt_pkcs1_oaep(chunk, public_key))

    def render(self):
        "Produce the raw yaml to be injected in the final file."
        twrap = textwrap.TextWrapper(
            width=79,
            initial_indent=" " * self.indent,
            subsequent_indent=" " * (self.indent + 2),
        )
        output = []
        for chunk in self.ciphertext:
            output.append(twrap.fill("- " + base64.b64encode(chunk).decode("utf-8")))
        return "\n".join(output)


def parse_yaml(txt):
    "Split a yaml document into raw lines and pkcs1-oaep chunk."
    pos = 0
    lines = txt.split("\n")
    name = ""
    while pos < len(lines):
        line = lines[pos]
        if line.strip().startswith("name:"):
            try:
                name = line.split(":")[1].strip()
            except IndexError:
                name = ""
        pos += 1
        if line.rstrip().endswith("!encrypted/pkcs1-oaep"):
            yield ("raw", line)
            indent = len(lines[pos]) - len(lines[pos].lstrip())
            is_ssh = "ssh_private_key:" in line

            # Read all the lines in the indent layout and yield a secret chunk
            secret = []
            while pos < len(lines):
                line = lines[pos]
                pos += 1
                if len(line) < indent or line[indent - 1] not in [" ", "\t"]:
                    break
                secret.append(line[indent:])
            yield (
                "ssh" if (is_ssh and name == "site_sflogs") else "sec",
                PKCS(indent, "\n".join(secret)),
            )
        yield ("raw", line)


def render_yaml(xs):
    "Render a list of yaml chunk back into its original form."
    output = []
    for tag, val in xs:
        if tag in ["sec", "ssh"]:
            output.append(val.render())
        else:
            output.append(val)
    return "\n".join(output)


def roundtrip_test(txt):
    assert txt == render_yaml(list(parse_yaml(txt)))


def get_giturl(conn, project):
    match conn["driver"]:
        case "git":
            return conn["baseurl"].rstrip("/") + "/" + project
        case "gerrit":
            return (
                "ssh://"
                + conn["user"]
                + "@"
                + conn["server"]
                + ":"
                + str(conn.get("port", 29418))
                + "/"
                + project
            )
        case "gitlab":
            return "ssh://git@" + conn["server"] + "/" + project
        case "github":
            return "ssh://git@" + conn["server"] + "/" + project
        case default:
            raise RuntimeError(f"TODO: handle '{default}' source driver")


def yaml_walk(root):
    "Find all the yaml file inside the root directory."
    import os

    for base, _, files in os.walk(root):
        base = Path(base)
        for f in files:
            if f.endswith(".yaml") or f.endswith(".yml"):
                yield (base / f)


def do_rotate_inrepo_secret(repo_dir, private_key, logserver_key):
    "Re-encrypt secret and return the new project key if it was generated"
    new_key = None
    root = Path(repo_dir)
    for fp in itertools.chain(
        [root / "zuul.yaml", root / ".zuul.yaml"],
        yaml_walk(root / ".zuul.d"),
        yaml_walk(root / "zuul.d"),
    ):
        if not fp.exists():
            continue
        chunks = list(parse_yaml(open(fp).read()))
        has_secret = False
        for chunk in chunks:
            if chunk[0] in ["sec", "ssh"]:
                has_secret = True
                if new_key is None:
                    new_key = ProjectKey()
                if chunk[0] == "sec":
                    data = chunk[1].decrypt(private_key).encode("utf-8")
                else:
                    data = logserver_key
                chunk[1].encrypt(data, new_key.pub)
        if has_secret:
            print(f"[+] Re-Encrypting secret(s) in {fp}")
            open(fp, "w").write(render_yaml(chunks))
    return new_key


def wait_process(args, cwd=None):
    import subprocess

    if subprocess.Popen(args, cwd=cwd).wait() != 0:
        raise RuntimeError("Command failed: " + " ".join(args))


def rotate_inrepo_secret(author, ssh_key, git_url, private_key, logserver_key):
    "Rotate the secrets found in git_url, return the new project key if it was generated"
    dest_path = "/tmp/current-repo"
    wait_process(["rm", "-Rf", dest_path])
    print(f"[+] Cloning {git_url} to {dest_path}")
    git = [
        "env",
        "GIT_AUTHOR_NAME=" + author[0],
        "GIT_AUTHOR_EMAIL=" + author[1],
        "GIT_COMMITTER_NAME=" + author[0],
        "GIT_COMMITTER_EMAIL=" + author[1],
        f"GIT_SSH_COMMAND=ssh -i {ssh_key} -o StrictHostKeyChecking=no",
        "git",
    ]
    wait_process(git + ["clone", "--depth", "1", git_url, dest_path])
    if new_key := do_rotate_inrepo_secret(dest_path, private_key, logserver_key):
        wait_process(
            git
            + [
                "commit",
                "-a",
                "-m",
                "Automatic secret re-encryption",
            ],
            cwd=dest_path,
        )
        wait_process(git + ["push"], cwd=dest_path)
        return new_key


def get_projects(tenants):
    "Return the list of project that have secrets in their load_classes"
    from zuul.configloader import TenantParser

    class Source:
        def getProject(self, name):
            return name

    # We re-use the TenantParser getProjects to handle the complex yaml format,
    # fortunately this helper doesn't need any of the TenantParser global state, which can be set to None.
    tp = TenantParser(None, None, None, None, None, None, None, None)
    projects = dict()
    for tenant in tenants:
        for source, projs in tenant.get("tenant", {}).get("source", {}).items():
            projects.setdefault(source, [])
            default_include = frozenset(["secret"])
            for conf in projs.get("config-projects", []) + projs.get(
                "untrusted-projects", []
            ):
                for proj in tp._getProjects(Source(), conf, default_include):
                    if "secret" in proj.load_classes:
                        projects[source].append(proj.project)
    return projects


def read_configs():
    import configparser

    config = configparser.ConfigParser()
    config.read("/etc/zuul/zuul.conf")
    tenants = yaml.safe_load(open(config["scheduler"]["tenant_config"]))
    connections = dict()
    for name, section in config.items():
        if name.startswith("connection "):
            connections[name.split()[1]] = section
    return (config, tenants, connections)


def usage():
    import argparse

    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--age",
        type=int,
        help="The minimum age of key (in EPOCh second) to be rotated",
        required=True,
    )
    parser.add_argument("--author", help="The commit author name", default="admin")
    parser.add_argument(
        "--email", help="The commit author email", default="root@localhost"
    )
    parser.add_argument("--logserver-key", required=True)
    args = parser.parse_args()
    return (args.age, base64.b64decode(args.logserver_key), (args.author, args.email))


def main():
    (config, tenants, connections) = read_configs()
    projects = get_projects(tenants)

    # TODO: make these a command line argument
    (leaked_before, logserver_key, author) = usage()
    ssh_key = "/var/lib/zuul/.ssh_push_key"

    from zuul.zk import ZooKeeperClient
    from zuul.lib.keystorage import KeyStorage
    import urllib.parse

    zk_client = ZooKeeperClient.fromConfig(config)
    zk_client.connect()

    def delete(path, reason):
        print(f"[+] Deleting {path} because {reason}")
        zk_client.client.delete(path)

    to_be_rotated = []

    print("[+] Collecting keys from ZooKeeper")
    password = config["keystore"]["password"].encode("utf-8")
    for path, obj in KeyStorage(zk_client, "unused").exportKeys()["keys"].items():
        if "keys" not in obj:
            print(f"[E] {path}: unknown object, expected keys attribute: {obj}")
            continue
        if len(obj["keys"]) != 1:
            print(f"[E] {path}: unknown object, expected a single key in {obj}")

        if obj["keys"][0]["created"] > leaked_before:
            continue

        if path.endswith("/secrets"):
            match path.split("/"):
                case ["", "keystorage", conn, _, encoded_name, "secrets"]:
                    project = urllib.parse.unquote_plus(encoded_name)
                    private_key, _ = encryption.deserialize_rsa_keypair(
                        obj["keys"][0]["private_key"].encode("utf-8"), password
                    )
                    to_be_rotated.append((path, conn, project, private_key))
                case default:
                    print(f"[E] {path}: unknown secrets path: {default}")
        else:
            delete(path, "Non secrets private key")

    for path, conn, project, private_key in to_be_rotated:
        if conn not in projects or conn not in connections:
            delete(path, f"unknown project source connection {conn}")
        elif project not in projects[conn]:
            delete(path, f"unknown project {project} in {conn}")
        elif conn == "git-server":
            delete(path, "internal repo will be handled in reconcile")
        else:
            giturl = get_giturl(connections[conn], project)
            try:
                new_key = rotate_inrepo_secret(
                    author, ssh_key, giturl, private_key, logserver_key
                )
            except Exception as e:
                print(f"[E] Failed to rotate inrepo secrets {e}")
                continue

            if new_key:
                print(f"[+] Updating key for {path}")
                zk_client.client.set(path, value=new_key.export(password))
            else:
                delete(path, "project had no secret")


if __name__ == "__main__":
    main()
