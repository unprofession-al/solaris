# solaris

![solaris](./solaris.svg "solaris")

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

## How it works

`solaris` analyzes the current directory and its subdirectories. Based on the 
[terraform outputs](https://www.terraform.io/docs/configuration/outputs.html) and
[remote states](https://www.terraform.io/docs/state/remote.html) `solaris` is 
able to discover dependencies between those configurations.

In addition to that `solaris` allows you do document manual work required to be
executed before or after a terraform configuration has been applied. This 
documentation can again refere to terraform outputs in order to project dependencies
between a terraform configuration and manual tasks.

Based on this information you can...

* ... draw a graph to visualize your dependencies
* ... 'lint' your dependencies in order do avoid confusion
* ... generate a step-by-step documentation that allows you to bootstrap your environment

## Run

Execute just `solaris` to get a general help. Append `--help` for more infromation
of each sub command.


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

