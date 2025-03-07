#!/bin/sh

set -ex

# config-update usage context required a specific git ref
REF=$1

if [[ "$CONFIG_REPO_BASE_URL" =~ https://gerrit.sfop.me.* ]]; then
    # FIXME: use internal CA to secure that connection when it is available.
    # Until then, we know for sure that this domain can't be verified
    export GIT_SSL_NO_VERIFY=true
fi

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
