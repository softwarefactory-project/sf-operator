#!/bin/bash
if ! umurmurd -d -c /etc/umurmur/umurmur.conf 2>/dev/null
then
    exit 0
fi
exit 1