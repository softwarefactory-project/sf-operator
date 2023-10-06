# Delete SF-Operator related resources

The `sfconfig` command offers the following option:

```sh
./tools/sfconfig operator delete [OPTIONS]

OPTIONS
  --subscription, -s - deletes Software Factory Operator Subscription
  --catalogsource, -S - deletes Software Factory Catalog Source
  --clusterserviceversion, -c - deletes Software Factory Cluster Service Version
  --all, -a - executes all options in sequence
  --verbose, -v - verbose
```

To completely remove any resource related to software factory run the following command:

```sh
./tools/sfconfig operator delete -a
# OR
./tools/sfconfig operator delete -s -S -c
```

All the commands related to `sfconfig` with the operator option can be executed with the verbose option.

```sh
./tools/sfconfig sf delete [OPTIONS] -v
./tools/sfconfig operator delete [OPTIONS] -v
```