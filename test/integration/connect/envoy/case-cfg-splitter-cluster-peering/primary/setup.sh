#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


set -euo pipefail

function upsert_config_entry {
  local DC="$1"
  local BODY="$2"

  echo "$BODY" | docker_consul "$DC" config write -
}

function docker_exec {
  if ! docker.exe exec -i "$@"; then
    echo "Failed to execute: docker exec -i $@" 1>&2
    return 1
  fi
}

function docker_consul {
  local DC=$1
  shift 1
  docker_exec envoy_consul-${DC}_1 "$@"
}

upsert_config_entry primary '
kind = "proxy-defaults"
name = "global"
config {
  protocol = "http"
}
'

upsert_config_entry primary '
kind = "service-splitter"
name = "split-s2"
splits = [
  {
    Weight  = 50
    Service = "local-s2"
    ResponseHeaders {
      Set {
        "x-test-split" = "primary"
      }
    }
  },
  {
    Weight  = 50
    Service = "peer-s2"
    ResponseHeaders {
      Set {
        "x-test-split" = "alpha"
      }
    }
  },
]
'

upsert_config_entry primary '
kind = "service-resolver"
name = "local-s2"
redirect = {
  service = "s2"
}
'

upsert_config_entry primary '
kind = "service-resolver"
name = "peer-s2"
redirect = {
  service = "s2"
  peer    = "primary-to-alpha"
}
'

register_services primary

gen_envoy_bootstrap s1 19000 primary
gen_envoy_bootstrap s2 19001 primary
