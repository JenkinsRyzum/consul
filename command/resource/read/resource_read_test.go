// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package read

import (
	"errors"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestResourceReadCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	cases := []struct {
		name      string
		extraArgs []string
		output    string
		err       error
	}{
		{
			name:   "basic output",
			output: "Billable Service Instances Total: 2",
		},
		{
			name:      "billable and connect flags together are invalid",
			extraArgs: []string{"-billable", "-connect"},
			err:       errors.New("Cannot specify both -billable and -connect"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			c := New(ui)
			args := []string{
				"demo.v2.artist",
				"keith",
				"-http-addr=" + a.HTTPAddr(),
				"-token=root",
				"-namespace=default",
				"-peer=local",
				"-partition=default",
			}
			args = append(args, tc.extraArgs...)

			code := c.Run(args)
			if tc.err != nil {
				require.Equal(t, 1, code)
				require.Contains(t, ui.ErrorWriter.String(), tc.err.Error())
			} else {
				require.Equal(t, 0, code)
				require.Contains(t, ui.OutputWriter.String(), tc.output)
			}
		})
	}
}
