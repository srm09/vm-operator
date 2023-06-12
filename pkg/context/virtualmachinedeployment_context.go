package context

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	vmopv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
)

// VirtualMachineDeploymentContext is the context used for VirtualMachineDeploymentController.
type VirtualMachineDeploymentContext struct {
	context.Context
	Logger                   logr.Logger
	VirtualMachineDeployment *vmopv1.VirtualMachineDeployment
}

func (v *VirtualMachineDeploymentContext) String() string {
	return fmt.Sprintf("%s %s/%s",
		v.VirtualMachineDeployment.GroupVersionKind(),
		v.VirtualMachineDeployment.Namespace,
		v.VirtualMachineDeployment.Name)
}
