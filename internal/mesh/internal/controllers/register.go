// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package controllers

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/cache/sidecarproxycache"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/sidecarproxy"
	"github.com/hashicorp/consul/internal/mesh/internal/mappers/sidecarproxymapper"
)

type Dependencies struct {
	TrustDomainFetcher sidecarproxy.TrustDomainFetcher
	LocalDatacenter    string
}

func Register(mgr *controller.Manager, deps Dependencies) {
	destinationsCache := sidecarproxycache.NewDestinationsCache()
	proxyCfgCache := sidecarproxycache.NewProxyConfigurationCache()
	m := sidecarproxymapper.New(destinationsCache, proxyCfgCache)

	mgr.Register(
		sidecarproxy.Controller(destinationsCache, proxyCfgCache, m, deps.TrustDomainFetcher, deps.LocalDatacenter),
	)
}
