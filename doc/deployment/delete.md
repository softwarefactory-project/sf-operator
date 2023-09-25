# Delete a deployment

The `sfconfig` cli command provides several levels of deletion.


```sh
./tools/sfconfig sf delete [OPTIONS]

OPTIONS
  --instance, -i - deletes Software Factory Instance only
  --pvcs, -p - deletes Software Factory including PVCs and PVs
  --all, -a - executes --instance and --pvcs options in sequence
  --verbose, -v - verbose
```

This removes the `my-sf` Custom Resource instance.

```sh
./tools/sfconfig sf delete -i
```

However, **Persistent Volumes Claims** linked to the resource are not cleaned after the deletion of the software factory instance.

> This is intended, so that it is easy to re-spin an instance with the same configuration and data.

To delete the software factory instance including the PVCs and PVs run the following command:

```sh
./tools/sfconfig sf delete -p
```

Or to delete everything in one go just run the following command:

```sh
./tools/sfconfig sf delete -i -p
OR
./tools/sfconfig sf delete -a
```