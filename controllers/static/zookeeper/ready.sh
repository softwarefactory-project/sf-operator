#!/bin/sh
if [ -f /tls/client/ca.crt ]; then
  echo "ruok" | openssl s_client -CAfile /tls/client/ca.crt -cert /tls/client/tls.crt -key /tls/client/tls.key -connect 127.0.0.1:"${1:-2281}" -quiet -ign_eof 2>/dev/null
else
  echo ruok | nc 127.0.0.1 "${1:-2181}"
fi
