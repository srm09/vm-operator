// Copyright (c) 2024 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// VirtualMachineManagementPolicy controls how the VMs are created during
// initial scale up and scale down.
// +kubebuilder:validation:Enum=OrderedReady;Parallel
type VirtualMachineManagementPolicy string

const (
	// OrderedReadyVirtualMachineManagementPolicy causes the VMs to be created in
	// increasing order and new VM creation waits until each VM is ready before
	// continuing. Similarly, on a scale down, the VMs are deleted in the reverse
	// order.
	OrderedReadyVirtualMachineManagementPolicy VirtualMachineManagementPolicy = "OrderedReady"

	// ParallelVirtualMachineManagementPolicy causes the VMs to be created in parallel
	// to match the desired scale without waiting for the VMs to be ready. A scale down
	// will delete all the VMs at once.
	ParallelVirtualMachineManagementPolicy VirtualMachineManagementPolicy = "Parallel"
)

// VirtualMachineStatefulSetUpdateStrategyType defines the type of stateful set
// update strategy.
// +kubebuilder:validation:Enum=Recreate;RollingUpdate
type VirtualMachineStatefulSetUpdateStrategyType string

const (
	// VirtualMachineStatefulSetUpdateStrategyTypeRecreate indicates to delete all existing VMs
	// before creating new ones.
	VirtualMachineStatefulSetUpdateStrategyTypeRecreate VirtualMachineStatefulSetUpdateStrategyType = "Recreate"

	// VirtualMachineStatefulSetUpdateStrategyTypeRollingUpdate indicates to replace
	// the old VMs with a new one using a rolling update, i.e.
	// gradually scale down the old VMs and scaling up the new one.
	VirtualMachineStatefulSetUpdateStrategyTypeRollingUpdate VirtualMachineStatefulSetUpdateStrategyType = "RollingUpdate"
)

type RollingUpdateVirtualMachineStatefulSetStrategy struct {
	// MaxUnavailable is the maximum number of VMs that can be unavailable
	// during the update.
	//
	// Value can be an absolute number (ex: 5) or a percentage of desired VMs
	// (ex: 10%).
	//
	// Absolute number is calculated from percentage by rounding down.
	//
	// Defaults to 25%.
	//
	// Example: when this is set to 30%, the old VMs can be scaled down
	// 70% of desired VMs immediately when the rolling update starts. Once new
	// VMs are ready, the old VMs can be scaled down further, followed
	// by scaling up the new VMs, ensuring the total number of VMs
	// available at all times during the update is at least 70% of desired VMs.
	//
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`

	// Partition indicates the ordinal at which the StatefulSet should be partitioned for updates.
	// During a rolling update, all VMs from ordinal Replicas-1 to Partition are updated while VMs from
	// ordinal Partition-1 to 0 remain untouched.
	//
	// This is helpful in being able to do a canary based deployment.
	//
	//The default value is 0.
	//
	// +optional
	Partition int32 `json:"partition,omitempty"`
}

type VirtualMachineStatefulSetUpdateStrategy struct {
	// Type of update strategy. Can be "Recreate" or "RollingUpdate".
	//
	// +optional
	// +kubebuilder:default=RollingUpdate
	Type VirtualMachineStatefulSetUpdateStrategyType `json:"type,omitempty"`

	// RollingUpdate is used to configure the deployment of new VMs.
	//
	// This field is used only if Type is RollingUpdate
	//
	// +optional
	RollingUpdate *RollingUpdateVirtualMachineStatefulSetStrategy `json:"rollingUpdate,omitempty"`
}

type VirtualMachineStatefulSetOrdinals struct {
	// Start is the number representing the first replica's index.
	// It can be used to override the default 0-indexed replica names
	// with an alternate index (eg: 1-indexed)
	//
	// If set, replica indices will be in the range:
	//   [.spec.ordinals.start, .spec.ordinals.start + .spec.replicas).
	// If unset, defaults to 0. Replica indices will be in the range:
	//   [0, .spec.replicas).
	Start int32 `json:"start"`
}

type VirtualMachineStatefulSetSpec struct {
	// MinReadySeconds is the minimum number of seconds for which a newly
	// created VM should be ready for it to be considered available.
	//
	// Defaults to 0 (VM will be considered available as soon as it is ready)
	//
	// +optional
	MinReadySeconds int32 `json:"minReadySeconds,omitempty"`

	// Ordinals controls the numbering of the replica indices in a
	// VirtualMachineStatefulSet. The default behavior assigns a "0" index to the
	// first replicas and increments the index by one for each additional replica
	// requested.
	//
	// +optional
	Ordinals VirtualMachineStatefulSetOrdinals `json:"ordinals,omitempty"`

	// ManagementPolicy defines the policy used to create/delete the VM instances
	// to match the desired scale during initial creation, scale up and scale down operations.
	//
	// If unset, it defaults to OrderedReady.
	//
	// +optional
	// +kubebuilder:default=OrderedReady
	ManagementPolicy VirtualMachineManagementPolicy `json:"managementPolicy,omitempty"`

	// Replicas specifies the number of desired VMs.
	// This is a pointer to distinguish between explicit zero and unspecified.
	//
	// +optional
	// +kubebuilder:default=1
	Replicas *int32 `json:"replicas,omitempty"`

	// RevisionHistoryLimit is the number of old stateful sets to retain to allow
	// rollback.
	//
	// This is a pointer to distinguish between explicit zero and not specified.
	//
	// +optional
	// +kubebuilder:default=10
	RevisionHistoryLimit *int32 `json:"revisionHistoryLimit,omitempty"`

	// Selector is a label query over VMs that should match the replica count.
	// Label keys and values that must match in order to be controlled by this
	// stateful set.
	//
	// It must match the VM template's labels.
	//
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
	Selector *metav1.LabelSelector `json:"selector"`

	// Template describes the VMs that will be created.
	Template VirtualMachineTemplateSpec `json:"template"`

	// Strategy is the stateful set update strategy to use to replace existing VMs with
	// new ones.
	//
	// +optional
	Strategy VirtualMachineStatefulSetUpdateStrategy `json:"strategy,omitempty"`

	// VolumeClaimTemplates is a list of claims that the VMs are allowed to reference. A claim in this list
	// takes precedence over any volumes in the template, with the same name.
	//
	// +optional
	VolumeClaimTemplates []VirtualMachineVolume `json:"volumeClaimTemplates,omitempty"`
}

type VirtualMachineStatefulSetStatus struct {
	// AvailableReplicas is the total number of available VMs (ready for at
	// least minReadySeconds) targeted by this stateful set.
	//
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// TODO
	// CollisionCount is the number of hash collisions for the Stateful Set.
	//
	// The Stateful Set controller uses this field as a collision avoidance
	// mechanism when it needs to create the name for the newest ControllerRevision.
	//
	// +optional
	CollisionCount *int32 `json:"collisionCount,omitempty"`

	// CurrentReplicas...
	//
	// +optional
	CurrentReplicas int32 `json:"currentReplicas,omitempty"`

	// ObservedGeneration is the most recent generation observed for this VirtualMachineStatefulSet.
	//
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ReadyReplicas is the number of VMs targeted by this stateful set with a
	// Ready Condition.
	//
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Replicas is the total number of non-terminated VMs targeted by this
	// stateful set (their labels match the selector).
	//
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// UpdatedReplicas is the total number of non-terminated VMs targeted by
	// this stateful set that have the desired template spec.
	//
	// +optional
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty"`

	// Conditions represents the latest available observations of a stateful set's
	// current state.
	//
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,shortName=vmstatefulset
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Replicas",type="integer",priority=1,JSONPath=".status.replicas"
// +kubebuilder:printcolumn:name="Ready-Replicas",type="integer",priority=1,JSONPath=".status.readyReplicas"
// +kubebuilder:printcolumn:name="Available-Replicas",type="integer",JSONPath=".status.availableReplicas"

// VirtualMachineStatefulSet is the schema for the virtualmachinestatefulsets API
// and represents the desired state and observed status of a
// virtualmachinestatefulsets resource.
type VirtualMachineStatefulSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualMachineStatefulSetSpec   `json:"spec,omitempty"`
	Status VirtualMachineStatefulSetStatus `json:"status,omitempty"`
}

func (vmrs VirtualMachineStatefulSet) NamespacedName() string {
	return vmrs.Namespace + "/" + vmrs.Name
}

// +kubebuilder:object:root=true

// VirtualMachineStatefulSetList contains a list of VirtualMachineStatefulSet resources.
type VirtualMachineStatefulSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualMachineStatefulSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(
		&VirtualMachineStatefulSet{},
		&VirtualMachineStatefulSetList{},
	)
}
