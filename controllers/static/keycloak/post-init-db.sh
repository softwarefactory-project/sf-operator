#!/bin/bash

set -ex

ftype=$(mysql --host=mariadb \
  --user=keycloak \
  --password="$DB_PASSWORD" \
  --database=keycloak \
  -BNe "select data_type from information_schema.columns where table_name = 'USER_ATTRIBUTE' and column_name = 'VALUE';")

echo "USER_ATTRIBUTE table, VALUE field type: $ftype"

if [ "$ftype" == "varchar" ]; then
  mysql --host=mariadb \
    --user=keycloak \
    --password="$DB_PASSWORD" \
    --database=keycloak \
    -e "alter table USER_ATTRIBUTE drop index IDX_USER_ATTRIBUTE_NAME; alter table USER_ATTRIBUTE modify VALUE TEXT(100000) CHARACTER SET utf8 COLLATE utf8_general_ci; alter table USER_ATTRIBUTE ADD KEY IDX_USER_ATTRIBUTE_NAME (NAME, VALUE(400));"
fi