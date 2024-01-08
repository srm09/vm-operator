// Copyright (c) 2020-2021 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package vsphere

import (
	"context"
	"fmt"

	vimtypes "github.com/vmware/govmomi/vim25/types"
	k8serrors "k8s.io/apimachinery/pkg/util/errors"

	vmopv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha1"
	"github.com/vmware-tanzu/vm-operator/pkg/topology"
	"github.com/vmware-tanzu/vm-operator/pkg/vmprovider/providers/vsphere/clustermodules"
	"github.com/vmware-tanzu/vm-operator/pkg/vmprovider/providers/vsphere/vcenter"
)

// IsVirtualMachineSetResourcePolicyReady checks if the VirtualMachineSetResourcePolicy for the AZ is ready.
func (vs *vSphereVMProvider) IsVirtualMachineSetResourcePolicyReady(
	ctx context.Context,
	azName string,
	resourcePolicy *vmopv1.VirtualMachineSetResourcePolicy) (bool, error) {

	client, err := vs.getVcClient(ctx)
	if err != nil {
		return false, err
	}

	folderMoID, rpMoID, err := topology.GetNamespaceFolderAndRPMoID(ctx, vs.k8sClient, azName, resourcePolicy.Namespace)
	if err != nil {
		return false, err
	}

	folderExists, err := vcenter.DoesChildFolderExist(ctx, client.VimClient(), folderMoID, resourcePolicy.Spec.Folder.Name)
	if err != nil {
		return false, err
	}

	rpExists, err := vcenter.DoesChildResourcePoolExist(ctx, client.VimClient(), rpMoID, resourcePolicy.Spec.ResourcePool.Name)
	if err != nil {
		return false, err
	}

	clusterRef, err := vcenter.GetResourcePoolOwnerMoRef(ctx, client.VimClient(), rpMoID)
	if err != nil {
		return false, err
	}

	modulesExist, err := vs.doClusterModulesExist(ctx, client.ClusterModuleClient(), clusterRef.Reference(), resourcePolicy)
	if err != nil {
		return false, err
	}

	if !rpExists || !folderExists || !modulesExist {
		log.V(4).Info("Resource policy is not ready", "resourcePolicy", resourcePolicy.Name,
			"namespace", resourcePolicy.Name, "az", azName, "resourcePool", rpExists, "folder", folderExists, "modules", modulesExist)
		return false, nil
	}

	return true, nil
}

// CreateOrUpdateVirtualMachineSetResourcePolicy creates if a VirtualMachineSetResourcePolicy doesn't exist, updates otherwise.
func (vs *vSphereVMProvider) CreateOrUpdateVirtualMachineSetResourcePolicy(
	ctx context.Context,
	resourcePolicy *vmopv1.VirtualMachineSetResourcePolicy) error {

	folderMoID, rpMoIDs, err := vs.getNamespaceFolderAndRPMoIDs(ctx, resourcePolicy.Namespace)
	if err != nil {
		return err
	}

	client, err := vs.getVcClient(ctx)
	if err != nil {
		return err
	}

	vimClient := client.VimClient()
	var errs []error

	_, err = vcenter.CreateFolder(ctx, vimClient, folderMoID, resourcePolicy.Spec.Folder.Name)
	if err != nil {
		errs = append(errs, err)
	}

	for _, rpMoID := range rpMoIDs {
		_, err := vcenter.CreateOrUpdateChildResourcePool(ctx, vimClient, rpMoID, &resourcePolicy.Spec.ResourcePool)
		if err != nil {
			errs = append(errs, err)
		}

		clusterRef, err := vcenter.GetResourcePoolOwnerMoRef(ctx, vimClient, rpMoID)
		if err == nil {
			err = vs.createClusterModules(ctx, client.ClusterModuleClient(), clusterRef.Reference(), resourcePolicy)
		}
		if err != nil {
			errs = append(errs, err)
		}
	}

	return k8serrors.NewAggregate(errs)
}

// DeleteVirtualMachineSetResourcePolicy deletes the VirtualMachineSetPolicy.
func (vs *vSphereVMProvider) DeleteVirtualMachineSetResourcePolicy(
	ctx context.Context,
	resourcePolicy *vmopv1.VirtualMachineSetResourcePolicy) error {

	folderMoID, rpMoIDs, err := vs.getNamespaceFolderAndRPMoIDs(ctx, resourcePolicy.Namespace)
	if err != nil {
		return err
	}

	client, err := vs.getVcClient(ctx)
	if err != nil {
		return err
	}

	vimClient := client.VimClient()
	var errs []error

	for _, rpMoID := range rpMoIDs {
		err := vcenter.DeleteChildResourcePool(ctx, vimClient, rpMoID, resourcePolicy.Spec.ResourcePool.Name)
		if err != nil {
			errs = append(errs, err)
		}
	}

	errs = append(errs, vs.deleteClusterModules(ctx, client.ClusterModuleClient(), resourcePolicy)...)

	if err := vcenter.DeleteChildFolder(ctx, vimClient, folderMoID, resourcePolicy.Spec.Folder.Name); err != nil {
		errs = append(errs, err)
	}

	return k8serrors.NewAggregate(errs)
}

// doClusterModulesExist checks whether all the ClusterModules for the given VirtualMachineSetResourcePolicy
// have been created and exist in VC for the Session's Cluster.
func (vs *vSphereVMProvider) doClusterModulesExist(
	ctx context.Context,
	clusterModProvider clustermodules.Provider,
	clusterRef vimtypes.ManagedObjectReference,
	resourcePolicy *vmopv1.VirtualMachineSetResourcePolicy) (bool, error) {

	for _, moduleSpec := range resourcePolicy.Spec.ClusterModules {
		_, moduleID := clustermodules.FindClusterModuleUUID(ctx, moduleSpec.GroupName, clusterRef, resourcePolicy)
		if moduleID == "" {
			return false, nil
		}

		exists, err := clusterModProvider.DoesModuleExist(ctx, moduleID)
		if !exists || err != nil {
			return false, err
		}
	}

	return true, nil
}

// createClusterModules creates all the ClusterModules that has not created yet for a
// given VirtualMachineSetResourcePolicy in VC.
func (vs *vSphereVMProvider) createClusterModules(
	ctx context.Context,
	clusterModProvider clustermodules.Provider,
	clusterRef vimtypes.ManagedObjectReference,
	resourcePolicy *vmopv1.VirtualMachineSetResourcePolicy) error {

	var errs []error

	// There is no way to give a name when creating a VC cluster module, so we have to
	// resort to using the status as the source of truth. This can result in orphaned
	// modules if, for instance, we fail to update the resource policy k8s object.
	for _, moduleSpec := range resourcePolicy.Spec.ClusterModules {
		idx, moduleID := clustermodules.FindClusterModuleUUID(ctx, moduleSpec.GroupName, clusterRef, resourcePolicy)

		if moduleID != "" {
			// Verify this cluster module exists on VC for this cluster.
			exists, err := clusterModProvider.DoesModuleExist(ctx, moduleID)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if !exists {
				// Status entry is stale. Create below.
				moduleID = ""
			}
		} else {
			var err error
			// See if there is already a module for this cluster but without the ClusterMoID field
			// set that we can claim.
			idx, moduleID, err = clustermodules.ClaimClusterModuleUUID(ctx, clusterModProvider,
				moduleSpec.GroupName, resourcePolicy)
			if err != nil {
				errs = append(errs, err)
				continue
			}
		}

		if moduleID == "" {
			var err error
			moduleID, err = clusterModProvider.CreateModule(ctx, clusterRef)
			if err != nil {
				errs = append(errs, err)
				continue
			}
		}

		if idx >= 0 {
			resourcePolicy.Status.ClusterModules[idx].ModuleUuid = moduleID
			resourcePolicy.Status.ClusterModules[idx].ClusterMoID = clusterRef.Value
		} else {
			status := vmopv1.ClusterModuleStatus{
				GroupName:   moduleSpec.GroupName,
				ModuleUuid:  moduleID,
				ClusterMoID: clusterRef.Value,
			}
			resourcePolicy.Status.ClusterModules = append(resourcePolicy.Status.ClusterModules, status)
		}
	}

	return k8serrors.NewAggregate(errs)
}

// deleteClusterModules deletes all the ClusterModules associated with a given VirtualMachineSetResourcePolicy in VC.
func (vs *vSphereVMProvider) deleteClusterModules(
	ctx context.Context,
	clusterModProvider clustermodules.Provider,
	resourcePolicy *vmopv1.VirtualMachineSetResourcePolicy) []error {

	var errModStatus []vmopv1.ClusterModuleStatus
	var errs []error

	for _, moduleStatus := range resourcePolicy.Status.ClusterModules {
		err := clusterModProvider.DeleteModule(ctx, moduleStatus.ModuleUuid)
		if err != nil {
			errModStatus = append(errModStatus, moduleStatus)
			errs = append(errs, err)
		}
	}

	resourcePolicy.Status.ClusterModules = errModStatus
	return errs
}

func (vs *vSphereVMProvider) getNamespaceFolderAndRPMoIDs(
	ctx context.Context,
	namespace string) (string, []string, error) {

	folderMoID, rpMoIDs, err := topology.GetNamespaceFolderAndRPMoIDs(ctx, vs.k8sClient, namespace)
	if err != nil {
		return "", nil, err
	}

	if folderMoID == "" {
		return "", nil, fmt.Errorf("namespace %s not present in any AvailabilityZones", namespace)
	}

	return folderMoID, rpMoIDs, nil
}
