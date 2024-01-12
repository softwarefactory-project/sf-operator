#!/bin/sh

set -ex

# config-update usage context required a specific git ref
REF=$1

# Clone or fetch config repository
if [ -d ~/config/.git ]; then
  pushd ~/config
  git remote | grep origin && git remote remove origin
  git remote add origin ${CONFIG_REPO_BASE_URL}/${CONFIG_REPO_NAME}
  if [ -z "$REF" ]; then
    # Discover default remote branch ref
    REF="origin/$(git remote show origin | sed -n '/HEAD branch/s/.*: //p')"
  fi
  if [ "$INIT_CONTAINER" == "1" ]; then
    git fetch origin || true
    git reset --hard $REF || true
  else
    git fetch origin
    git reset --hard $REF
  fi
  popd
else
  pushd ~/
  if [ "$INIT_CONTAINER" == "1" ]; then
    git clone ${CONFIG_REPO_BASE_URL}/${CONFIG_REPO_NAME} config || true
  else
    git clone ${CONFIG_REPO_BASE_URL}/${CONFIG_REPO_NAME} config
  fi
  popd
fi
