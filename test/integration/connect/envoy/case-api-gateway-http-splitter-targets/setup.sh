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
kind = "api-gateway"
name = "api-gateway"
listeners = [
  {
    name = "listener-one"
    port = 9999
    protocol = "http"
  }
]
'

upsert_config_entry primary '
Kind      = "proxy-defaults"
Name      = "global"
Config {
  protocol = "http"
}
'

upsert_config_entry primary '
kind = "http-route"
name = "api-gateway-route-one"
rules = [
  {
    services = [
      {
        name = "splitter-one"
      }
    ]
  }
]
parents = [
  {
    name = "api-gateway"
    sectionName = "listener-one"
  }
]
'

upsert_config_entry primary '
kind = "service-splitter"
name = "splitter-one"
splits = [
  {
    weight = 50,
    service = "s1"
  },
  {
    weight = 50,
    service = "splitter-two"
  },
]
'

upsert_config_entry primary '
kind = "service-splitter"
name = "splitter-two"
splits = [
  {
    weight = 50,
    service = "s2"
  },
  {
    weight = 50,
    service = "s3"
  },
]
'

register_services primary

gen_envoy_bootstrap api-gateway 20000 primary true
gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001
gen_envoy_bootstrap s3 19002