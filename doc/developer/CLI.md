# Developing the sfconfig CLI tool

## Adding a new subcommand

Install cobra-cli:

```sh
go install github.com/spf13/cobra-cli@latest
```

Then, to add a new subcommand:

```sh
cd cli/sfconfig
~/go/bin/cobra-cli add myCommand
cd -
```

Edit `cli/sfconfig/cmd/myCommand.go` to implement the new subcommand.

The subcommand can be directly used after editing myCommand.go with

```sh
go run cli/sfconfig/main.go myCommand
```

A wrapper also exists in the `./tools` directory:

```sh
./tools/sfconfig myCommand
```