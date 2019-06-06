# solaris

`solaris` is a small command line tool which allows to keep track of dependencies
between related [terraform](https://www.terraform.io) configurations.

## Install

### Binary Download

Navigate to [Releases](https://github.com/unprofession-al/solaris/releases), grab
the package that matches your operating system and achitecture. Unpack the archive
and put the binary file somewhere in your `$PATH`

### From Source

Make sure you have [go](https://golang.org/doc/install) installed, then run: 

```
# go get -u https://github.com/unprofession-al/solaris
```

## Run

```
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
      --debug            write debug output to STDERR
  -h, --help             help for solaris
  -i, --ignore strings   ignore subdirectories that match the given patterns

Use "solaris [command] --help" for more information about a command.
```

## TODO

- Create Data Souces via solaris: `solaris refer service/test` -> creates `terraform_remote_state` data source
- Allow `terraform_remote_states` with backends other than s3
- Add tests

