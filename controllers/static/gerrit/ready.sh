#!/bin/sh

echo "Waiting for httpd"
curl http://localhost:8080/config/server/version

echo "Waiting for zuul account"
curl http://localhost:8080/accounts/?q=name:zuul

echo "Waiting for sshd"
python3 -c 'import socket; socket.socket(socket.AF_INET, socket.SOCK_STREAM).connect(("localhost", 29418))'
