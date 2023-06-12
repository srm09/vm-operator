// Copyright (c) 2024 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/vmware-tanzu/vm-operator/api/v1alpha1"
)

const (
	// VirtualMachineDeploymentDeploymentAvailable means the deployment is
	// available, ie. at least the minimum available replicas required are up
	//and running for at least minReadySeconds.
	VirtualMachineDeploymentDeploymentAvailable = "Available"

	// VirtualMachineDeploymentDeploymentProgressing means the deployment is
	// progressing. Progress for a deployment is considered when a new replica
	// set is created or adopted, and when new VMs scale up or old VMs scale
	// down. Progress is not estimated for paused deployments or when
	// progressDeadlineSeconds is not specified.
	VirtualMachineDeploymentDeploymentProgressing = "Progressing"

	// VirtualMachineDeploymentDeploymentReplicaFailure is added in a deployment
	// when one of its VMs fails to be created or deleted.
	VirtualMachineDeploymentDeploymentReplicaFailure = "ReplicaFailure"
)

// VirtualMachineDeploymentStrategyType defines the type of deployment strategy.
// +kubebuilder:validation:Enum=Recreate;RollingUpdate
type VirtualMachineDeploymentStrategyType string

const (
	// VirtualMachineDeploymentStrategyTypeRecreate indicates to delete all existing VMs
	// before creating new ones.
	VirtualMachineDeploymentStrategyTypeRecreate VirtualMachineDeploymentStrategyType = "Recreate"

	// VirtualMachineDeploymentStrategyTypeRollingUpdate indicates to replace
	// the old replica sets with a new one using a rolling update, i.e.
	// gradually scale down the old replica sets and scaling up the new one.
	VirtualMachineDeploymentStrategyTypeRollingUpdate VirtualMachineDeploymentStrategyType = "RollingUpdate"
)

const (
	VirtualMachineDeploymentNameLabel = "vmoperator.vmware.com/deployment-name"

	// RevisionAnnotation is the revision annotation of a machine deployment's machine sets which records its rollout sequence.
	RevisionAnnotation = "virtualmachinedeployment.vmoperator.vmware.com/revision"

	// RevisionHistoryAnnotation maintains the history of all old revisions that a machine set has served for a machine deployment.
	RevisionHistoryAnnotation = "virtualmachinedeployment.vmoperator.vmware.com/revision-history"

	// DesiredReplicasAnnotation is the desired replicas for a machine deployment recorded as an annotation
	// in its machine sets. Helps in separating scaling events from the rollout process and for
	// determining if the new machine set for a deployment is really saturated.
	DesiredReplicasAnnotation = "virtualmachinedeployment.vmoperator.vmware.com/desired-replicas"

	// MaxReplicasAnnotation is the maximum replicas a deployment can have at a given point, which
	// is machinedeployment.spec.replicas + maxSurge. Used by the underlying machine sets to estimate their
	// proportions in case the deployment has surge replicas.
	MaxReplicasAnnotation = "virtualmachinedeployment.vmoperator.vmware.com/max-replicas"

	// MachineDeploymentUniqueLabel is used to uniquely identify the Machines of a MachineSet.
	// The MachineDeployment controller will set this label on a MachineSet when it is created.
	// The label is also applied to the Machines of the MachineSet and used in the MachineSet selector.
	// Note: For the lifetime of the MachineSet the label's value has to stay the same, otherwise the
	// MachineSet selector would no longer match its Machines.
	// Note: In previous Cluster API versions (< v1.4.0), the label value was the hash of the full machine template.
	// With the introduction of in-place mutation the machine template of the MachineSet can change.
	// Because of that it is impossible that the label's value to always be the hash of the full machine template.
	// (Because the hash changes when the machine template changes).
	// As a result, we use the hash of the machine template while ignoring all in-place mutable fields, i.e. the
	// machine template with only fields that could trigger a rollout for the machine-template-hash, making it
	// independent of the changes to any in-place mutable fields.
	// A random string is appended at the end of the label value (label value format is "<hash>-<random string>"))
	// to distinguish duplicate MachineSets that have the exact same spec but were created as a result of rolloutAfter.
	VirtualMachineDeploymentUniqueLabel = "machine-template-hash"
)

// VirtualMachineDeploymentRollingUpdateStrategy specifies the desired behavior
// of a rolling update.
type VirtualMachineDeploymentRollingUpdateStrategy struct {

	// MaxUnavailable is the maximum number of VMs that can be unavailable
	// during the update.
	//
	// Value can be an absolute number (ex: 5) or a percentage of desired VMs
	// (ex: 10%).
	//
	// Absolute number is calculated from percentage by rounding down.
	//
	// This can not be 0 if MaxSurge is 0.
	//
	// Defaults to 25%.
	//
	// Example: when this is set to 30%, the old replica set can be scaled down
	// 70% of desired VMs immediately when the rolling update starts. Once new
	// VMs are ready, the old replica set can be scaled down further, followed
	// by scaling up the new replica set, ensuring the total number of VMs
	// available at all times during the update is at least 70% of desired VMs.
	//
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`

	// MaxSurge is the maximum number of VMs that can be scheduled above the
	// desired number of VMs.
	//
	// Value can be an absolute number (ex: 5) or a percentage of desired VMs
	// (ex: 10%).
	//
	// This can not be 0 if MaxUnavailable is 0.
	//
	// Absolute number is calculated from percentage by rounding up.
	//
	// Defaults to 25%.
	//
	// Example: when this is set to 30%, the new replica set can be scaled up
	// immediately when the rolling update starts, such that the total number
	// of old and new VMs do not exceed 130% of desired VMs. Once old VMs have
	// been killed, the new replica set can be scaled up further, ensuring the
	// total number of VMs running at any time during the update is at most
	// 130% of desired VMs.
	//
	// +optional
	MaxSurge *intstr.IntOrString `json:"maxSurge,omitempty"`
}

// VirtualMachineDeploymentStrategy describes how to replace existing VMs with
// new ones.
type VirtualMachineDeploymentStrategy struct {
	// Type of deployment. Can be "Recreate" or "RollingUpdate".
	//
	// +optional
	// +kubebuilder:default=RollingUpdate
	Type VirtualMachineDeploymentStrategyType `json:"type,omitempty"`

	// RollingUpdate is used to configure the deployment of new VMs when the
	// deployment strategy is config params.
	//
	// This field is only used if Type is RollingUpdate.
	//
	// +optional
	RollingUpdate *VirtualMachineDeploymentRollingUpdateStrategy `json:"rollingUpdate,omitempty"`
}

// VirtualMachineDeploymentSpec is the specification of a
// VirtualMachineDeployment.
type VirtualMachineDeploymentSpec struct {
	// Replicas specifies the number of desired VMs.
	// This is a pointer to distinguish between explicit zero and unspecified.
	//
	// +optional
	// +kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`

	// Selector is a label selector for VMs.
	//
	// Existing VirtualMachineReplicaSets whose VMs are selected by this will be
	// the ones affected by this deployment.
	//
	// It must match the VM template's labels.
	//
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`

	// Template describes the VMs that will be created.
	Template VirtualMachineTemplateSpec `json:"template"`

	// Strategy is the deployment strategy to use to replace existing VMs with
	// new ones.
	//
	// +optional
	Strategy VirtualMachineDeploymentStrategy `json:"strategy,omitempty"`

	// MinReadySeconds is the minimum number of seconds for which a newly
	// created VM should be ready for it to be considered available.
	//
	// Defaults to 0 (VM will be considered available as soon as it is ready)
	//
	// +optional
	MinReadySeconds int32 `json:"minReadySeconds,omitempty"`

	// RevisionHistoryLimit is the number of old replica sets to retain to allow
	// rollback.
	//
	// This is a pointer to distinguish between explicit zero and not specified.
	//
	// +optional
	// +kubebuilder:default=10
	RevisionHistoryLimit *int32 `json:"revisionHistoryLimit,omitempty"`

	// Paused indicates that the deployment is paused.
	//
	// +optional
	Paused bool `json:"paused,omitempty"`

	// ProgressDeadlineSeconds is the maximum time in seconds for a deployment
	// to make progress before it is considered to be failed.
	//
	// The deployment controller will continue to process failed deployments
	// and a condition with a ProgressDeadlineExceeded reason will be surfaced
	// in the deployment status.
	//
	// Please note that progress will not be estimated during the time a
	// deployment is paused.
	//
	// +optional
	// +kubebuilder:default=600
	ProgressDeadlineSeconds *int32 `json:"progressDeadlineSeconds,omitempty"`
}

// VirtualMachineDeploymentStatus represents the observed state of a
// VirtualMachineDeployment resource.
type VirtualMachineDeploymentStatus struct {
	// ObservedGeneration is the generation observed by the deployment
	// controller.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Replicas is the total number of non-terminated VMs targeted by this
	// deployment (their labels match the selector).
	//
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// UpdatedReplicas is the total number of non-terminated VMs targeted by
	// this deployment that have the desired template spec.
	//
	// +optional
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`

	// ReadyReplicas is the number of VMs targeted by this Deployment with a
	// Ready Condition.
	//
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// AvailableReplicas is the total number of available VMs (ready for at
	// least minReadySeconds) targeted by this deployment.
	//
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// UnavailableReplicas is the total number of unavailable VMs targeted by
	// this deployment.
	//
	// This is the total number of VMs that are still required for the
	// deployment to have 100% available capacity. They may either be VMs that
	// are running but not yet available or VMs that still have not been
	// created.
	//
	// +optional
	UnavailableReplicas int32 `json:"unavailableReplicas,omitempty"`

	// Conditions represents the latest available observations of a deployment's
	// current state.
	//
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// CollisionCount is the number of hash collisions for the Deployment.
	//
	// The Deployment controller uses this field as a collision avoidance
	// mechanism when it needs to create the name for the newest replica set.
	//
	// +optional
	CollisionCount *int32 `json:"collisionCount,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,shortName=vmdeployment
// +kubebuilder:storageversion
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Replicas",type="integer",priority=1,JSONPath=".status.replicas"
// +kubebuilder:printcolumn:name="Ready-Replicas",type="integer",priority=1,JSONPath=".status.readyReplicas"
// +kubebuilder:printcolumn:name="Available-Replicas",type="integer",JSONPath=".status.availableReplicas"

// VirtualMachineDeployment is the schema for the virtualmachinedeployments API
// and represents the desired state and observed status of a
// virtualmachinedeployments resource.
type VirtualMachineDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualMachineDeploymentSpec   `json:"spec,omitempty"`
	Status VirtualMachineDeploymentStatus `json:"status,omitempty"`
}

func (vmrs VirtualMachineDeployment) NamespacedName() string {
	return vmrs.Namespace + "/" + vmrs.Name
}

// +kubebuilder:object:root=true

// VirtualMachineDeploymentList contains a list of VirtualMachineDeployment.
type VirtualMachineDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualMachineDeployment `json:"items"`
}

func init() {
	v1alpha1.SchemeBuilder.Register(
		&VirtualMachineDeployment{},
		&VirtualMachineDeploymentList{},
	)
}
