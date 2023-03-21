# Contributing

This document provides some instructions to get started with sf-operator development.

## Config Update Job

To modify or debug the config-update job you need a copy of the system-config:

```ShellSession
kubectl port-forward service/git-server 9418 &
git clone git://localhost:9418/system-config /tmp/system-config
```

After changing the playbooks or tasks, just `git push`.

Finally, trigger a new `config-update` by running the following command:

```ShellSession
( cd /tmp/config &&
  date > trigger &&
  git add trigger && git commit -m "Trigger job" && git review && sleep 1 && git push gerrit
)
```
