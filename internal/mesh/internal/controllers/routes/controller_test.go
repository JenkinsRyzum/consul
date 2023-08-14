// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

type controllerSuite struct {
	suite.Suite

	ctx    context.Context
	client *rtest.Client
	rt     controller.Runtime
}

func (suite *controllerSuite) SetupTest() {
	suite.ctx = testutil.TestContext(suite.T())
	client := svctest.RunResourceService(suite.T(), types.Register, catalog.RegisterTypes)
	suite.rt = controller.Runtime{
		Client: client,
		Logger: testutil.Logger(suite.T()),
	}
	suite.client = rtest.NewClient(client)
}

func (suite *controllerSuite) TestController() {
	// TODO: tidy comment
	//
	// This test's purpose is to exercise the controller in a halfway realistic
	// way.

	// Run the controller manager
	mgr := controller.NewManager(suite.client, suite.rt.Logger)
	mgr.Register(Controller())
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	backendName := func(name, port string) string {
		return fmt.Sprintf("catalog.v1alpha1.Service/default.local.default/%s?port=%s", name, port)
	}

	var (
		apiServiceRef = rtest.Resource(catalog.ServiceType, "api").Reference("")
		fooServiceRef = rtest.Resource(catalog.ServiceType, "foo").Reference("")
		barServiceRef = rtest.Resource(catalog.ServiceType, "bar").Reference("")

		computedRoutesID = rtest.Resource(types.ComputedRoutesType, "api").ID()
	)

	// Start out by creating a single port service and let it create the
	// default mesh config for tcp.

	apiServiceData := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{"api-"},
		},
		Ports: []*pbcatalog.ServicePort{
			{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			// {TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
		},
	}

	_ = rtest.Resource(catalog.ServiceType, "api").
		WithData(suite.T(), apiServiceData).
		Write(suite.T(), suite.client)

	var lastVersion string
	testutil.RunStep(suite.T(), "default tcp route", func(t *testing.T) {
		// Check that the mesh config resource exists and it has one port that is the default.
		expect := &pbmesh.ComputedRoutes{
			PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
				"tcp": {
					UsingDefaultConfig: true,
					Config: &pbmesh.ComputedPortRoutes_Tcp{
						Tcp: &pbmesh.InterpretedTCPRoute{
							ParentRef: newParentRef(apiServiceRef, "tcp"),
							Rules: []*pbmesh.InterpretedTCPRouteRule{{
								BackendRefs: []*pbmesh.InterpretedTCPBackendRef{{
									BackendTarget: backendName("api", "tcp"),
								}},
							}},
						},
					},
					Targets: map[string]*pbmesh.BackendTargetDetails{
						backendName("api", "tcp"): {
							BackendRef: newBackendRef(apiServiceRef, "tcp", ""),
							Service:    apiServiceData,
						},
					},
				},
			},
		}

		lastVersion = requireNewComputedRoutesVersion(t, suite.client, computedRoutesID, "", expect)
	})

	// Let the default http/http2/grpc routes get created.

	apiServiceData = &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{"api-"},
		},
		Ports: []*pbcatalog.ServicePort{
			{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			{TargetPort: "http2", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP2},
			{TargetPort: "grpc", Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
		},
	}

	_ = rtest.Resource(catalog.ServiceType, "api").
		WithData(suite.T(), apiServiceData).
		Write(suite.T(), suite.client)

	// also create the fooService so we can point to it.
	fooServiceData := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{"foo-"},
		},
		Ports: []*pbcatalog.ServicePort{
			{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
			{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			{TargetPort: "http2", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP2},
			{TargetPort: "grpc", Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
		},
	}

	_ = rtest.Resource(catalog.ServiceType, "foo").
		WithData(suite.T(), fooServiceData).
		Write(suite.T(), suite.client)

	testutil.RunStep(suite.T(), "default other routes", func(t *testing.T) {
		// Check that the mesh config resource exists and it has one port that is the default.
		expect := &pbmesh.ComputedRoutes{
			PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
				"tcp": {
					UsingDefaultConfig: true,
					Config: &pbmesh.ComputedPortRoutes_Tcp{
						Tcp: &pbmesh.InterpretedTCPRoute{
							ParentRef: newParentRef(apiServiceRef, "tcp"),
							Rules: []*pbmesh.InterpretedTCPRouteRule{{
								BackendRefs: []*pbmesh.InterpretedTCPBackendRef{{
									BackendTarget: backendName("api", "tcp"),
								}},
							}},
						},
					},
					Targets: map[string]*pbmesh.BackendTargetDetails{
						backendName("api", "tcp"): {
							BackendRef: newBackendRef(apiServiceRef, "tcp", ""),
							Service:    apiServiceData,
						},
					},
				},
				"http": {
					UsingDefaultConfig: true,
					Config: &pbmesh.ComputedPortRoutes_Http{
						Http: &pbmesh.InterpretedHTTPRoute{
							ParentRef: newParentRef(apiServiceRef, "http"),
							Rules: []*pbmesh.InterpretedHTTPRouteRule{{
								Matches: []*pbmesh.HTTPRouteMatch{{
									Path: &pbmesh.HTTPPathMatch{
										Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
										Value: "/",
									},
								}},
								BackendRefs: []*pbmesh.InterpretedHTTPBackendRef{{
									BackendTarget: backendName("api", "http"),
								}},
							}},
						},
					},
					Targets: map[string]*pbmesh.BackendTargetDetails{
						backendName("api", "http"): {
							BackendRef: newBackendRef(apiServiceRef, "http", ""),
							Service:    apiServiceData,
						},
					},
				},
				"http2": {
					UsingDefaultConfig: true,
					Config: &pbmesh.ComputedPortRoutes_Http{
						Http: &pbmesh.InterpretedHTTPRoute{
							ParentRef: newParentRef(apiServiceRef, "http2"),
							Rules: []*pbmesh.InterpretedHTTPRouteRule{{
								Matches: []*pbmesh.HTTPRouteMatch{{
									Path: &pbmesh.HTTPPathMatch{
										Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
										Value: "/",
									},
								}},
								BackendRefs: []*pbmesh.InterpretedHTTPBackendRef{{
									BackendTarget: backendName("api", "http2"),
								}},
							}},
						},
					},
					Targets: map[string]*pbmesh.BackendTargetDetails{
						backendName("api", "http2"): {
							BackendRef: newBackendRef(apiServiceRef, "http2", ""),
							Service:    apiServiceData,
						},
					},
				},
				"grpc": {
					UsingDefaultConfig: true,
					Config: &pbmesh.ComputedPortRoutes_Grpc{
						Grpc: &pbmesh.InterpretedGRPCRoute{
							ParentRef: newParentRef(apiServiceRef, "grpc"),
							Rules: []*pbmesh.InterpretedGRPCRouteRule{{
								Matches: []*pbmesh.GRPCRouteMatch{{}},
								BackendRefs: []*pbmesh.InterpretedGRPCBackendRef{{
									BackendTarget: backendName("api", "grpc"),
								}},
							}},
						},
					},
					Targets: map[string]*pbmesh.BackendTargetDetails{
						backendName("api", "grpc"): {
							BackendRef: newBackendRef(apiServiceRef, "grpc", ""),
							Service:    apiServiceData,
						},
					},
				},
			},
		}

		lastVersion = requireNewComputedRoutesVersion(t, suite.client, computedRoutesID, lastVersion, expect)
	})

	// Customize each route type.

	tcpRoute1 := &pbmesh.TCPRoute{
		ParentRefs: []*pbmesh.ParentReference{
			newParentRef(newRef(catalog.ServiceType, "api"), "tcp"),
		},
		Rules: []*pbmesh.TCPRouteRule{{
			BackendRefs: []*pbmesh.TCPBackendRef{{
				BackendRef: newBackendRef(fooServiceRef, "", ""),
			}},
		}},
	}
	_ = rtest.Resource(types.TCPRouteType, "api-tcp-route1").
		WithData(suite.T(), tcpRoute1).
		Write(suite.T(), suite.client)

	httpRoute1 := &pbmesh.HTTPRoute{
		ParentRefs: []*pbmesh.ParentReference{
			newParentRef(newRef(catalog.ServiceType, "api"), "http"),
			newParentRef(newRef(catalog.ServiceType, "api"), "http2"),
		},
		Rules: []*pbmesh.HTTPRouteRule{{
			BackendRefs: []*pbmesh.HTTPBackendRef{{
				BackendRef: newBackendRef(fooServiceRef, "", ""),
			}},
		}},
	}
	_ = rtest.Resource(types.HTTPRouteType, "api-http-route1").
		WithData(suite.T(), httpRoute1).
		Write(suite.T(), suite.client)

	grpcRoute1 := &pbmesh.GRPCRoute{
		ParentRefs: []*pbmesh.ParentReference{
			newParentRef(newRef(catalog.ServiceType, "api"), "grpc"),
		},
		Rules: []*pbmesh.GRPCRouteRule{{
			BackendRefs: []*pbmesh.GRPCBackendRef{{
				BackendRef: newBackendRef(fooServiceRef, "", ""),
			}},
		}},
	}
	_ = rtest.Resource(types.GRPCRouteType, "api-grpc-route1").
		WithData(suite.T(), grpcRoute1).
		Write(suite.T(), suite.client)

	testutil.RunStep(suite.T(), "one of each", func(t *testing.T) {
		expect := &pbmesh.ComputedRoutes{
			PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
				"tcp": {
					Config: &pbmesh.ComputedPortRoutes_Tcp{
						Tcp: &pbmesh.InterpretedTCPRoute{
							ParentRef: newParentRef(apiServiceRef, "tcp"),
							Rules: []*pbmesh.InterpretedTCPRouteRule{{
								BackendRefs: []*pbmesh.InterpretedTCPBackendRef{{
									BackendTarget: backendName("foo", "tcp"),
								}},
							}},
						},
					},
					Targets: map[string]*pbmesh.BackendTargetDetails{
						backendName("foo", "tcp"): {
							BackendRef: newBackendRef(fooServiceRef, "tcp", ""),
							Service:    fooServiceData,
						},
					},
				},
				"http": {
					Config: &pbmesh.ComputedPortRoutes_Http{
						Http: &pbmesh.InterpretedHTTPRoute{
							ParentRef: newParentRef(apiServiceRef, "http"),
							Rules: []*pbmesh.InterpretedHTTPRouteRule{
								{
									Matches: defaultHTTPRouteMatches(),
									BackendRefs: []*pbmesh.InterpretedHTTPBackendRef{{
										BackendTarget: backendName("foo", "http"),
									}},
								},
								{
									Matches: defaultHTTPRouteMatches(),
									BackendRefs: []*pbmesh.InterpretedHTTPBackendRef{{
										BackendTarget: types.NullRouteBackend,
									}},
								},
							},
						},
					},
					Targets: map[string]*pbmesh.BackendTargetDetails{
						backendName("foo", "http"): {
							BackendRef: newBackendRef(fooServiceRef, "http", ""),
							Service:    fooServiceData,
						},
					},
				},
				"grpc": {
					Config: &pbmesh.ComputedPortRoutes_Grpc{
						Grpc: &pbmesh.InterpretedGRPCRoute{
							ParentRef: newParentRef(apiServiceRef, "grpc"),
							Rules: []*pbmesh.InterpretedGRPCRouteRule{
								{
									Matches: []*pbmesh.GRPCRouteMatch{{}},
									BackendRefs: []*pbmesh.InterpretedGRPCBackendRef{{
										BackendTarget: backendName("foo", "grpc"),
									}},
								},
								{
									Matches: []*pbmesh.GRPCRouteMatch{{}},
									BackendRefs: []*pbmesh.InterpretedGRPCBackendRef{{
										BackendTarget: types.NullRouteBackend,
									}},
								},
							},
						},
					},
					Targets: map[string]*pbmesh.BackendTargetDetails{
						backendName("foo", "grpc"): {
							BackendRef: newBackendRef(fooServiceRef, "grpc", ""),
							Service:    fooServiceData,
						},
					},
				},
				"http2": {
					Config: &pbmesh.ComputedPortRoutes_Http{
						Http: &pbmesh.InterpretedHTTPRoute{
							ParentRef: newParentRef(apiServiceRef, "http2"),
							Rules: []*pbmesh.InterpretedHTTPRouteRule{
								{
									Matches: defaultHTTPRouteMatches(),
									BackendRefs: []*pbmesh.InterpretedHTTPBackendRef{{
										BackendTarget: backendName("foo", "http2"),
									}},
								},
								{
									Matches: defaultHTTPRouteMatches(),
									BackendRefs: []*pbmesh.InterpretedHTTPBackendRef{{
										BackendTarget: types.NullRouteBackend,
									}},
								},
							},
						},
					},
					Targets: map[string]*pbmesh.BackendTargetDetails{
						backendName("foo", "http2"): {
							BackendRef: newBackendRef(fooServiceRef, "http2", ""),
							Service:    fooServiceData,
						},
					},
				},
			},
		}

		lastVersion = requireNewComputedRoutesVersion(t, suite.client, computedRoutesID, lastVersion, expect)
	})

	// Add another route, with a bad mapping.

	tcpRoute2 := &pbmesh.TCPRoute{
		ParentRefs: []*pbmesh.ParentReference{
			newParentRef(newRef(catalog.ServiceType, "api"), "tcp"),
		},
		Rules: []*pbmesh.TCPRouteRule{{
			BackendRefs: []*pbmesh.TCPBackendRef{{
				BackendRef: newBackendRef(barServiceRef, "", ""),
			}},
		}},
	}
	_ = rtest.Resource(types.TCPRouteType, "api-tcp-route2").
		WithData(suite.T(), tcpRoute2).
		Write(suite.T(), suite.client)

	httpRoute2 := &pbmesh.HTTPRoute{
		ParentRefs: []*pbmesh.ParentReference{
			newParentRef(newRef(catalog.ServiceType, "api"), "http"),
			newParentRef(newRef(catalog.ServiceType, "api"), "http2"),
		},
		Rules: []*pbmesh.HTTPRouteRule{{
			Matches: []*pbmesh.HTTPRouteMatch{{
				Path: &pbmesh.HTTPPathMatch{
					Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
					Value: "/healthz",
				},
			}},
			BackendRefs: []*pbmesh.HTTPBackendRef{{
				BackendRef: newBackendRef(barServiceRef, "", ""),
			}},
		}},
	}
	_ = rtest.Resource(types.HTTPRouteType, "api-http-route2").
		WithData(suite.T(), httpRoute2).
		Write(suite.T(), suite.client)

	grpcRoute2 := &pbmesh.GRPCRoute{
		ParentRefs: []*pbmesh.ParentReference{
			newParentRef(newRef(catalog.ServiceType, "api"), "grpc"),
		},
		Rules: []*pbmesh.GRPCRouteRule{{
			Matches: []*pbmesh.GRPCRouteMatch{{
				Method: &pbmesh.GRPCMethodMatch{
					Type:    pbmesh.GRPCMethodMatchType_GRPC_METHOD_MATCH_TYPE_EXACT,
					Service: "billing",
					Method:  "charge",
				},
			}},
			BackendRefs: []*pbmesh.GRPCBackendRef{{
				BackendRef: newBackendRef(barServiceRef, "", ""),
			}},
		}},
	}
	_ = rtest.Resource(types.GRPCRouteType, "api-grpc-route2").
		WithData(suite.T(), grpcRoute2).
		Write(suite.T(), suite.client)

	testutil.RunStep(suite.T(), "one good one bad route", func(t *testing.T) {
		expect := &pbmesh.ComputedRoutes{
			PortedConfigs: map[string]*pbmesh.ComputedPortRoutes{
				"tcp": {
					Config: &pbmesh.ComputedPortRoutes_Tcp{
						Tcp: &pbmesh.InterpretedTCPRoute{
							ParentRef: newParentRef(apiServiceRef, "tcp"),
							Rules: []*pbmesh.InterpretedTCPRouteRule{
								{
									BackendRefs: []*pbmesh.InterpretedTCPBackendRef{{
										BackendTarget: backendName("foo", "tcp"),
									}},
								},
								{
									BackendRefs: []*pbmesh.InterpretedTCPBackendRef{{
										BackendTarget: types.NullRouteBackend,
									}},
								},
							},
						},
					},
					Targets: map[string]*pbmesh.BackendTargetDetails{
						backendName("foo", "tcp"): {
							BackendRef: newBackendRef(fooServiceRef, "tcp", ""),
							Service:    fooServiceData,
						},
					},
				},
				"http": {
					Config: &pbmesh.ComputedPortRoutes_Http{
						Http: &pbmesh.InterpretedHTTPRoute{
							ParentRef: newParentRef(apiServiceRef, "http"),
							Rules: []*pbmesh.InterpretedHTTPRouteRule{
								{
									Matches: []*pbmesh.HTTPRouteMatch{{
										Path: &pbmesh.HTTPPathMatch{
											Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
											Value: "/healthz",
										},
									}},
									BackendRefs: []*pbmesh.InterpretedHTTPBackendRef{{
										BackendTarget: types.NullRouteBackend,
									}},
								},
								{
									Matches: defaultHTTPRouteMatches(),
									BackendRefs: []*pbmesh.InterpretedHTTPBackendRef{{
										BackendTarget: backendName("foo", "http"),
									}},
								},
								{
									Matches: defaultHTTPRouteMatches(),
									BackendRefs: []*pbmesh.InterpretedHTTPBackendRef{{
										BackendTarget: types.NullRouteBackend,
									}},
								},
							},
						},
					},
					Targets: map[string]*pbmesh.BackendTargetDetails{
						backendName("foo", "http"): {
							BackendRef: newBackendRef(fooServiceRef, "http", ""),
							Service:    fooServiceData,
						},
					},
				},
				"grpc": {
					Config: &pbmesh.ComputedPortRoutes_Grpc{
						Grpc: &pbmesh.InterpretedGRPCRoute{
							ParentRef: newParentRef(apiServiceRef, "grpc"),
							Rules: []*pbmesh.InterpretedGRPCRouteRule{
								{
									Matches: []*pbmesh.GRPCRouteMatch{{}},
									BackendRefs: []*pbmesh.InterpretedGRPCBackendRef{{
										BackendTarget: backendName("foo", "grpc"),
									}},
								},
								{
									Matches: []*pbmesh.GRPCRouteMatch{{
										Method: &pbmesh.GRPCMethodMatch{
											Type:    pbmesh.GRPCMethodMatchType_GRPC_METHOD_MATCH_TYPE_EXACT,
											Service: "billing",
											Method:  "charge",
										},
									}},
									BackendRefs: []*pbmesh.InterpretedGRPCBackendRef{{
										BackendTarget: types.NullRouteBackend,
									}},
								},
								{
									Matches: []*pbmesh.GRPCRouteMatch{{}},
									BackendRefs: []*pbmesh.InterpretedGRPCBackendRef{{
										BackendTarget: types.NullRouteBackend,
									}},
								},
							},
						},
					},
					Targets: map[string]*pbmesh.BackendTargetDetails{
						backendName("foo", "grpc"): {
							BackendRef: newBackendRef(fooServiceRef, "grpc", ""),
							Service:    fooServiceData,
						},
					},
				},
				"http2": {
					Config: &pbmesh.ComputedPortRoutes_Http{
						Http: &pbmesh.InterpretedHTTPRoute{
							ParentRef: newParentRef(apiServiceRef, "http2"),
							Rules: []*pbmesh.InterpretedHTTPRouteRule{
								{
									Matches: []*pbmesh.HTTPRouteMatch{{
										Path: &pbmesh.HTTPPathMatch{
											Type:  pbmesh.PathMatchType_PATH_MATCH_TYPE_PREFIX,
											Value: "/healthz",
										},
									}},
									BackendRefs: []*pbmesh.InterpretedHTTPBackendRef{{
										BackendTarget: types.NullRouteBackend,
									}},
								},
								{
									Matches: defaultHTTPRouteMatches(),
									BackendRefs: []*pbmesh.InterpretedHTTPBackendRef{{
										BackendTarget: backendName("foo", "http2"),
									}},
								},
								{
									Matches: defaultHTTPRouteMatches(),
									BackendRefs: []*pbmesh.InterpretedHTTPBackendRef{{
										BackendTarget: types.NullRouteBackend,
									}},
								},
							},
						},
					},
					Targets: map[string]*pbmesh.BackendTargetDetails{
						backendName("foo", "http2"): {
							BackendRef: newBackendRef(fooServiceRef, "http2", ""),
							Service:    fooServiceData,
						},
					},
				},
			},
		}

		lastVersion = requireNewComputedRoutesVersion(t, suite.client, computedRoutesID, lastVersion, expect)
	})

	// Remove the mesh port from api service.

	apiServiceData = &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{"api-"},
		},
		Ports: []*pbcatalog.ServicePort{
			{TargetPort: "tcp", Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
			{TargetPort: "http", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP},
			{TargetPort: "http2", Protocol: pbcatalog.Protocol_PROTOCOL_HTTP2},
			{TargetPort: "grpc", Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
		},
	}

	_ = rtest.Resource(catalog.ServiceType, "api").
		WithData(suite.T(), apiServiceData).
		Write(suite.T(), suite.client)

	testutil.RunStep(suite.T(), "entire generated resource is deleted", func(t *testing.T) {
		retry.Run(t, func(r *retry.R) {
			suite.client.RequireResourceNotFound(r, computedRoutesID)
		})
	})

}

func newParentRef(ref *pbresource.Reference, port string) *pbmesh.ParentReference {
	return &pbmesh.ParentReference{
		Ref:  ref,
		Port: port,
	}
}

func newBackendRef(ref *pbresource.Reference, port string, datacenter string) *pbmesh.BackendReference {
	return &pbmesh.BackendReference{
		Ref:        ref,
		Port:       port,
		Datacenter: datacenter,
	}
}

func requireNewComputedRoutesVersion(
	t *testing.T,
	client *rtest.Client,
	id *pbresource.ID,
	version string,
	expected *pbmesh.ComputedRoutes,
) string {
	t.Helper()

	var nextVersion string
	retry.Run(t, func(r *retry.R) {
		res := client.WaitForNewVersion(r, id, version)

		var mc pbmesh.ComputedRoutes
		require.NoError(r, res.Data.UnmarshalTo(&mc))
		prototest.AssertDeepEqual(r, expected, &mc)

		nextVersion = res.Version
	})
	return nextVersion
}

func TestController(t *testing.T) {
	suite.Run(t, new(controllerSuite))
}
