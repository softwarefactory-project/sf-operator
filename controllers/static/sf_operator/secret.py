# Copyright (C) 2022 Red Hat
# SPDX-License-Identifier: Apache-2.0

from urllib.request import Request
from urllib.request import urlopen

import re
import subprocess
import tempfile
import math
import os
import textwrap
import base64


def mk_secret(name, items, unencrypted_items=[]):
    # Borrowed from zuul/tools/encrypt_secret.py
    pubkey = urlopen(
        Request("http://zuul-web:9000/api/tenant/internal/key/system-config.pub")
    )
    pubkey_file = tempfile.NamedTemporaryFile(delete=False)
    pubkey_file.write(pubkey.read())
    pubkey_file.close()

    p = subprocess.Popen(
        ["openssl", "rsa", "-text", "-pubin", "-in", pubkey_file.name],
        stdout=subprocess.PIPE,
    )
    (stdout, stderr) = p.communicate()
    if p.returncode != 0:
        raise Exception("Return code %s from openssl" % p.returncode)
    output = stdout.decode("utf-8")

    key_length_re = r"^(|RSA )Public-Key: \((?P<key_length>\d+) bit\)$"
    m = re.match(key_length_re, output, re.MULTILINE)
    nbits = int(m.group("key_length"))
    nbytes = int(nbits / 8)
    max_bytes = nbytes - 42  # PKCS1-OAEP overhead

    secret_output = textwrap.dedent(
        """
        - secret:
            name: %s
            data:
        """
        % (name)
    )

    if unencrypted_items:
        for (key, value) in unencrypted_items:
            secret_output += "      " + key + ": " + value + "\n"

    for (key, value) in items:
        secret_output += "      " + key + ": !encrypted/pkcs1-oaep\n"
        chunks = int(math.ceil(float(len(value)) / max_bytes))

        ciphertext_chunks = []
        for count in range(chunks):
            chunk = value[int(count * max_bytes) : int((count + 1) * max_bytes)]
            p = subprocess.Popen(
                [
                    "openssl",
                    "rsautl",
                    "-encrypt",
                    "-oaep",
                    "-pubin",
                    "-inkey",
                    pubkey_file.name,
                ],
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
            )
            (stdout, stderr) = p.communicate(chunk.encode("utf-8"))
            if p.returncode != 0:
                raise Exception("Return code %s from openssl" % p.returncode)
            ciphertext_chunks.append(base64.b64encode(stdout).decode("utf-8"))

        twrap = textwrap.TextWrapper(
            width=79, initial_indent=" " * 8, subsequent_indent=" " * 10
        )
        for chunk in ciphertext_chunks:
            chunk = twrap.fill("- " + chunk)
            secret_output += chunk + "\n"

    os.unlink(pubkey_file.name)
    return secret_output
