#!/bin/sh

echo "ruok" | openssl s_client -CAfile /tls/client/ca.crt -cert /tls/client/tls.crt -key /tls/client/tls.key \
  -connect 127.0.0.1:2281 -quiet 2>/dev/null | grep "imok"
