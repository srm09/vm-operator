package v1alpha1

const (
	// MachinesCreatedCondition documents that the machines controlled by the MachineSet are created.
	// When this condition is false, it indicates that there was an error when cloning the infrastructure/bootstrap template or
	// when generating the machine object.
	MachinesCreatedCondition ConditionType = "MachinesCreated"

	// MachinesReadyCondition reports an aggregate of current status of the machines controlled by the MachineSet.
	MachinesReadyCondition ConditionType = "MachinesReady"

	// BootstrapTemplateCloningFailedReason (Severity=Error) documents a MachineSet failing to
	// clone the bootstrap template.
	BootstrapTemplateCloningFailedReason = "BootstrapTemplateCloningFailed"

	// InfrastructureTemplateCloningFailedReason (Severity=Error) documents a MachineSet failing to
	// clone the infrastructure template.
	InfrastructureTemplateCloningFailedReason = "InfrastructureTemplateCloningFailed"

	// MachineCreationFailedReason (Severity=Error) documents a MachineSet failing to
	// generate a machine object.
	MachineCreationFailedReason = "MachineCreationFailed"

	// ResizedCondition documents a MachineSet is resizing the set of controlled machines.
	ResizedCondition ConditionType = "Resized"

	// ScalingUpReason (Severity=Info) documents a MachineSet is increasing the number of replicas.
	ScalingUpReason = "ScalingUp"

	// ScalingDownReason (Severity=Info) documents a MachineSet is decreasing the number of replicas.
	ScalingDownReason = "ScalingDown"
)
