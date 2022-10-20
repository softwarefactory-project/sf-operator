# Copyright (C) 2022 Red Hat
# SPDX-License-Identifier: Apache-2.0

if ! test -f /var/lib/zuul/main.yaml; then
    cat <<EOF> /var/lib/zuul/main.yaml
- admin-rule:
    name: admin-user
    conditions:
      realm_access.roles: admin

- tenant:
    name: internal
    admin-rules:
      - admin-user
    source:
      git-server:
        config-projects:
          - system-config
EOF
fi
