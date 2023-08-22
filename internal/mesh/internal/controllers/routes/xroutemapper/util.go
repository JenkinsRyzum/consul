// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xroutemapper

import (
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func parentRefSliceToRefSlice(parentRefs []*pbmesh.ParentReference) []resource.ReferenceOrID {
	if parentRefs == nil {
		return nil
	}
	parents := make([]resource.ReferenceOrID, 0, len(parentRefs))
	for _, parentRef := range parentRefs {
		if parentRef.Ref != nil && types.IsServiceType(parentRef.Ref.Type) {
			parents = append(parents, parentRef.Ref)
		}
	}
	return parents
}

func backendRefSliceToRefSlice(backendRefs []*pbmesh.BackendReference) []resource.ReferenceOrID {
	if backendRefs == nil {
		return nil
	}
	backends := make([]resource.ReferenceOrID, 0, len(backendRefs))
	for _, backendRef := range backendRefs {
		if backendRef.Ref != nil && types.IsServiceType(backendRef.Ref.Type) {
			backends = append(backends, backendRef.Ref)
		}
	}
	return backends
}

func changeType(id *pbresource.ID, typ *pbresource.Type) *pbresource.ID {
	return &pbresource.ID{
		Type:    typ,
		Tenancy: id.Tenancy,
		Name:    id.Name,
	}
}

func changeTypeForSlice(list []*pbresource.ID, typ *pbresource.Type) []*pbresource.ID {
	if list == nil {
		return nil
	}
	out := make([]*pbresource.ID, 0, len(list))
	for _, id := range list {
		out = append(out, changeType(id, typ))
	}
	return out
}

func makeControllerRequests[V resource.ReferenceOrID](
	typ *pbresource.Type,
	refs []V,
) []controller.Request {
	if len(refs) == 0 {
		return nil
	}

	out := make([]controller.Request, 0, len(refs))
	for _, ref := range refs {
		out = append(out, controller.Request{
			ID: &pbresource.ID{
				Type:    typ,
				Tenancy: ref.GetTenancy(),
				Name:    ref.GetName(),
			},
		})
	}

	return out
}
