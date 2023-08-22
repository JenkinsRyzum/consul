// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package apply

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/mitchellh/cli"

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/consul/command/helpers"
	"github.com/hashicorp/consul/internal/resourcehcl"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func New(ui cli.Ui) *cmd {
	c := &cmd{UI: ui}
	c.init()
	return c
}

type cmd struct {
	UI    cli.Ui
	flags *flag.FlagSet
	http  *flags.HTTPFlags
	help  string

	filePath  string
}

func (c *cmd) init() {
	c.flags = flag.NewFlagSet("", flag.ContinueOnError)
	c.flags.StringVar(&c.filePath, "f", "",
		"File path with resource definition")

	c.http = &flags.HTTPFlags{}
	flags.Merge(c.flags, c.http.ClientFlags())
	flags.Merge(c.flags, c.http.ServerFlags())
	flags.Merge(c.flags, c.http.MultiTenancyFlags())
	flags.Merge(c.flags, c.http.AddPeerName())
	c.help = flags.Usage(help, c.flags)
}

func (c *cmd) Run(args []string) int {
	if err := c.flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		c.UI.Error(fmt.Sprintf("Failed to parse args: %v", err))
		return 1
	}

	var parsedResource *pbresource.Resource

	if c.filePath != "" {
		data, err := helpers.LoadDataSourceNoRaw(c.filePath, nil)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Failed to load data: %v", err))
			return 1
		}

		parsedResource, err = parseResource(data)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Your argument format is incorrect: %s", err))
			return 1
		}
	}

	fmt.Printf("**** parsed resource: %+v", parsedResource)

	client, err := c.http.APIClient()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error connect to Consul agent: %s", err))
		return 1
	}

	opts := &api.QueryOptions{
		Namespace: parsedResource.Id.Tenancy.GetNamespace(),
		Partition: parsedResource.Id.Tenancy.GetPartition(),
		Peer:      parsedResource.Id.Tenancy.GetPeerName(),
		Token:     c.http.Token(),
	}

	gvk := &api.GVK{
		Group: parsedResource.Id.Type.GetGroup(),
		Version: parsedResource.Id.Type.GetGroupVersion(),
		Kind: parsedResource.Id.Type.GetKind(),
	}

	writeRequest := &api.WriteRequest{
		Data: parsedResource.GetData(),
		Metadata: parsedResource.GetMetadata(),
		Owner: parsedResource.GetOwner(),
	}

	entry, _, err := client.Resource().Apply(gvk,  parsedResource.Id.GetName(), opts, writeRequest)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error writing resource %s/%s: %v", gvk, parsedResource.Id.GetName(), err))
		return 1
	}

	b, err := json.MarshalIndent(entry, "", "    ")
	if err != nil {
		c.UI.Error("Failed to encode output data")
		return 1
	}

	c.UI.Info(string(b))
	return 0
}

func parseResource(data string) (resource *pbresource.Resource, e error) {
	// parse the data
	raw := []byte(data)
	resource, err := resourcehcl.Unmarshal(raw, consul.NewTypeRegistry())
	if err != nil {
		return nil, fmt.Errorf("Failed to decode resource from input file: %v", err)
	}

	return resource, nil
}

func (c *cmd) Synopsis() string {
	return synopsis
}

func (c *cmd) Help() string {
	return flags.Usage(c.help, nil)
}

const synopsis = "Writes/updates resource information"
const help = `
Usage: consul resource apply [type] [name] -partition=<default> -namespace=<default> -peer=<local> -consistent=<false> -json

Reads the resource specified by the given type, name, partition, namespace, peer and reading mode
and outputs its JSON representation.

Example:

$ consul resource apply catalog.v1alpha1.Service card-processor -partition=billing -namespace=payments -peer=eu
`
