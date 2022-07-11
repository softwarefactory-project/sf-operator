# Copyright (C) 2022 Red Hat
# SPDX-License-Identifier: Apache-2.0

if ! test -f /var/lib/zuul/main.yaml; then
    cat <<EOF> /var/lib/zuul/main.yaml
- tenant:
    name: internal
    source:
      git-server:
        config-projects:
          - system-config
EOF
fi
