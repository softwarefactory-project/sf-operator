#!/bin/env python3
# Copyright (C) 2025 Red Hat
# SPDX-License-Identifier: Apache-2.0

# A small script to force zookeeper reconnection after a service restart.

import socket
import os


def main():
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)

    def send_cmd(code):
        if not code:
            s.connect(("127.0.0.1", 3000))
        else:
            s.send(b"print(self.server.scheduler." + code + b")\r\n")
        return s.recv(1024)

    send_cmd(None)
    send_cmd(b"log.info('Restarting zookeeper client')")
    send_cmd(b"zk_client.client.stop()")
    send_cmd(b"zk_client.client.start()")
    return b"imok" in send_cmd(b"zk_client.client.command(b'ruok')")


os.system("zuul-scheduler repl")
try:
    result = main()
finally:
    os.system("zuul-scheduler norepl")
exit(result)
