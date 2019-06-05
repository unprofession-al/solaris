# solaris

`solaris` is a small command line tool which allows to keep track of dependencies
between related [terraform](https://www.terraform.io) configurations.

## Install

Make sure you have [go](https://golang.org/doc/install) installed, then run: 

```
# go get -u https://github.com/unprofession-al/solaris
```

## Run

```
# solaris -h
handle dependencies between multiple terraform workspaces

Usage:
  solaris [command]

Available Commands:
  graph       generate dot output of terraform workspace dependencies
  help        Help about any command
  json        print a json representation of terraform workspace dependencies
  lint        lint terraform workspace dependencies
  plan        print execution order of terraform workspaces

Flags:
  -b, --base string      the base directory (default ".")
  -h, --help             help for solaris
  -i, --ignore strings   ignore subdirectories that match the given patterns

Use "solaris [command] --help" for more information about a command.
```

