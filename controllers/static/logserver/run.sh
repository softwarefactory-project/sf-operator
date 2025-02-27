#!/bin/sh

set -ex

# Setup zuul account
echo zuul:x:$(id -u):$(id -u):zuul:/tmp/home:/bin/bash >> /etc/passwd
mkdir -p -m 00700 /tmp/home/.ssh
echo $AUTHORIZED_KEY | base64 -d > /tmp/home/.ssh/authorized_keys
chmod 00400 /tmp/home/.ssh/authorized_keys
chmod 00700 /tmp/home
ln -s /home/data/rsync /tmp/home/rsync
cat /var/ssh-keys/priv > /tmp/ssh-key
chmod 400 /tmp/ssh-key

## Setup sshd service
cat > /tmp/home/sshd_config <<EOF
HostKey /tmp/ssh-key
Port 2222
UseDNS no
PasswordAuthentication no
AuthorizedKeysFile /tmp/home/.ssh/authorized_keys
EOF

exec /usr/sbin/sshd -D -f /tmp/home/sshd_config
