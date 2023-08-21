// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/command/flags"
)

const (
	ResourceFormatJSON   = "json"
	ResourceFormatPretty = "pretty"
)

func GetSupportedFormats() []string {
	return []string{ResourceFormatJSON, ResourceFormatPretty}
}

func New() *cmd {
	return &cmd{}
}

type cmd struct{}

func (c *cmd) Run(args []string) int {
	return cli.RunResultHelp
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(help, nil)
}

const synopsis = "Interact with Consul's resources"
const help = `
Usage: consul resource <subcommand> [options]

This command has subcommands for interacting with Consul's resources.
Here are some simple examples, and more detailed examples are available
in the subcommands or the documentation.

Read a resource:

$ consul resource write [type] [name] -partition=<default> -namespace=<default> -peer=<local> -consistent=<false> -json

Run

consul resource <subcommand> -h 

for help on that subcommand.
`
