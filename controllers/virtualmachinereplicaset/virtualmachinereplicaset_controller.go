package virtualmachinereplicaset

import (
	goctx "context"
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	vmopv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha1"
	"github.com/vmware-tanzu/vm-operator/api/v1alpha2"

	"github.com/vmware-tanzu/vm-operator/pkg/conditions"
	"github.com/vmware-tanzu/vm-operator/pkg/context"
	"github.com/vmware-tanzu/vm-operator/pkg/patch"
	"github.com/vmware-tanzu/vm-operator/pkg/record"
)

/*import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/controllers/noderefutil"
	"sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/cluster-api/internal/controllers/machine"
	capilabels "sigs.k8s.io/cluster-api/internal/labels"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/collections"
	"sigs.k8s.io/cluster-api/util/conditions"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	clog "sigs.k8s.io/cluster-api/util/log"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
)*/

var (
	// machineSetKind contains the schema.GroupVersionKind for the MachineSet type.
	machineSetKind = vmopv1.SchemeGroupVersion.WithKind("VirtualMachineReplicaSet")

	// stateConfirmationTimeout is the amount of time allowed to wait for desired state.
	stateConfirmationTimeout = 10 * time.Second

	// stateConfirmationInterval is the amount of time between polling for the desired state.
	// The polling is against a local memory cache.
	stateConfirmationInterval = 100 * time.Millisecond
)

const finalizerName = "virtualmachinereplicaset.vmoperator.vmware.com"

// AddToManager adds this package's controller to the provided manager.
func AddToManager(ctx *context.ControllerManagerContext, mgr manager.Manager) error {
	var (
		controlledType     = &v1alpha2.VirtualMachineReplicaSet{}
		controlledTypeName = reflect.TypeOf(controlledType).Elem().Name()

		controllerNameShort = fmt.Sprintf("%s-controller", strings.ToLower(controlledTypeName))
		controllerNameLong  = fmt.Sprintf("%s/%s/%s", ctx.Namespace, ctx.Name, controllerNameShort)
	)

	r := &Reconciler{
		Client: mgr.GetClient(),
		//ctrl.Log.WithName("controllers").WithName(controlledTypeName),
		recorder: record.New(mgr.GetEventRecorderFor(controllerNameLong)),
	}

	builder := ctrl.NewControllerManagedBy(mgr).
		For(controlledType).
		Owns(&vmopv1.VirtualMachine{}).
		Watches(
			&source.Kind{Type: &vmopv1.VirtualMachine{}},
			handler.EnqueueRequestsFromMapFunc(r.MachineToMachineSets),
		).
		WithOptions(controller.Options{MaxConcurrentReconciles: ctx.MaxConcurrentReconciles})

	return builder.Complete(r)
}

// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups=vmoperator.vmware.com,resources=virtualmachinedeployments,verbs=get;list;watch;
// +kubebuilder:rbac:groups=vmoperator.vmware.com,resources=virtualmachines,verbs=get;list;watch;create;update;patch;delete

// Reconciler reconciles a MachineSet object.
type Reconciler struct {
	Client    client.Client
	APIReader client.Reader

	recorder record.Recorder
}

/*func (r *Reconciler) SetupWithManager(ctx goctx.Context, mgr ctrl.Manager, options controller.Options) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&vmopv1.VirtualMachineReplicaSet{}).
		Owns(&vmopv1.VirtualMachine{}).
		Watches(
			&source.Kind{Type: &vmopv1.VirtualMachine{}},
			handler.EnqueueRequestsFromMapFunc(r.MachineToMachineSets),
		).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Build(r)
	if err != nil {
		return errors.Wrap(err, "failed setting up with a controller manager")
	}

	err = c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(clusterToMachineSets),
		// TODO: should this wait for Cluster.Status.InfrastructureReady similar to Infra Machine resources?
		predicates.All(ctrl.LoggerFrom(ctx),
			predicates.ClusterUnpaused(ctrl.LoggerFrom(ctx)),
			predicates.ResourceHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue),
		),
	)
	if err != nil {
		return errors.Wrap(err, "failed to add Watch for Clusters to controller manager")
	}

	r.recorder = mgr.GetEventRecorderFor("machineset-controller")
	return nil
}*/

func (r *Reconciler) Reconcile(ctx goctx.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	machineSet := &v1alpha2.VirtualMachineReplicaSet{}
	if err := r.Client.Get(ctx, req.NamespacedName, machineSet); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return. Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	log := ctrl.LoggerFrom(ctx)
	ctx = ctrl.LoggerInto(ctx, log)

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(machineSet, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	defer func() {
		// Always attempt to patch the object and status after each reconciliation.
		if err := patchMachineSet(ctx, patchHelper, machineSet); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// Ignore deleted MachineSets, this can happen when foregroundDeletion
	// is enabled
	if !machineSet.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	result, err := r.reconcile(ctx, machineSet)
	if err != nil {
		log.Error(err, "Failed to reconcile MachineSet")
		r.recorder.Eventf(machineSet, corev1.EventTypeWarning, "ReconcileError", "%v", err)
	}
	return result, err
}

func patchMachineSet(ctx goctx.Context, patchHelper *patch.Helper, machineSet *v1alpha2.VirtualMachineReplicaSet, options ...patch.Option) error {
	// Always update the readyCondition by summarizing the state of other conditions.
	/*conditions.SetSummary(machineSet,
		conditions.WithConditions(
			clusterv1.MachinesCreatedCondition,
			clusterv1.ResizedCondition,
			clusterv1.MachinesReadyCondition,
		),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	options = append(options,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			clusterv1.MachinesCreatedCondition,
			clusterv1.ResizedCondition,
			clusterv1.MachinesReadyCondition,
		}},
	)*/
	return patchHelper.Patch(ctx, machineSet, options...)
}

func (r *Reconciler) reconcile(ctx goctx.Context, machineSet *v1alpha2.VirtualMachineReplicaSet) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Reconcile and retrieve the Cluster object.
	if machineSet.Labels == nil {
		machineSet.Labels = make(map[string]string)
	}

	// Make sure selector and template to be in the same cluster.
	if machineSet.Spec.Selector.MatchLabels == nil {
		machineSet.Spec.Selector.MatchLabels = make(map[string]string)
	}

	if machineSet.Spec.Template.Labels == nil {
		machineSet.Spec.Template.Labels = make(map[string]string)
	}

	selectorMap, err := metav1.LabelSelectorAsMap(machineSet.Spec.Selector)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to convert MachineSet %q label selector to a map", machineSet.Name)
	}

	// Get all Machines linked to this MachineSet.
	allMachines := &vmopv1.VirtualMachineList{}
	err = r.Client.List(ctx,
		allMachines,
		client.InNamespace(machineSet.Namespace),
		client.MatchingLabels(selectorMap),
	)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to list machines")
	}

	// Filter out irrelevant machines (i.e. IsControlledBy something else) and claim orphaned machines.
	// Machines in deleted state are deliberately not excluded https://github.com/kubernetes-sigs/cluster-api/pull/3434.
	filteredMachines := make([]*vmopv1.VirtualMachine, 0, len(allMachines.Items))
	for idx := range allMachines.Items {
		machine := &allMachines.Items[idx]
		log := log.WithValues("Machine", klog.KObj(machine))
		if shouldExcludeMachine(machineSet, machine) {
			continue
		}

		// Attempt to adopt machine if it meets previous conditions and it has no controller references.
		if metav1.GetControllerOf(machine) == nil {
			if err := r.adoptOrphan(ctx, machineSet, machine); err != nil {
				log.Error(err, "Failed to adopt Machine")
				r.recorder.Eventf(machineSet, corev1.EventTypeWarning, "FailedAdopt", "Failed to adopt Machine %q: %v", machine.Name, err)
				continue
			}
			log.Info("Adopted Machine")
			r.recorder.Eventf(machineSet, corev1.EventTypeNormal, "SuccessfulAdopt", "Adopted Machine %q", machine.Name)
		}

		filteredMachines = append(filteredMachines, machine)
	}

	// If not already present, add a label specifying the MachineSet name to Machines.
	// Ensure all required labels exist on the controlled Machines.
	// This logic is needed to add the `cluster.x-k8s.io/set-name` label to Machines
	// which were created before the `cluster.x-k8s.io/set-name` label was added to
	// all Machines created by a MachineSet or if a user manually removed the label.
	for _, machine := range filteredMachines {
		mdNameOnMachineSet, mdNameSetOnMachineSet := machineSet.Labels[v1alpha2.VirtualMachineDeploymentNameLabel]
		mdNameOnMachine := machine.Labels[v1alpha2.VirtualMachineDeploymentNameLabel]

		// Note: MustEqualValue is used here as the value of this label will be a hash if the MachineSet name is longer than 63 characters.
		if msNameLabelValue, ok := machine.Labels[v1alpha2.VirtualMachineReplicaSetNameLabel]; ok && MustEqualValue(machineSet.Name, msNameLabelValue) &&
			(!mdNameSetOnMachineSet || mdNameOnMachineSet == mdNameOnMachine) {
			// Continue if the MachineSet name label is already set correctly and
			// either the MachineDeployment name label is not set on the MachineSet or
			// the MachineDeployment name label is set correctly on the Machine.
			continue
		}

		helper, err := patch.NewHelper(machine, r.Client)
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to apply %s label to Machine %q", v1alpha2.VirtualMachineReplicaSetNameLabel, machine.Name)
		}
		// Note: MustFormatValue is used here as the value of this label will be a hash if the MachineSet name is longer than 63 characters.
		machine.Labels[v1alpha2.VirtualMachineReplicaSetNameLabel] = MustFormatValue(machineSet.Name)
		// Propagate the MachineDeploymentLabelName from MachineSet to Machine if it is set on the MachineSet.
		if mdNameSetOnMachineSet {
			machine.Labels[v1alpha2.VirtualMachineDeploymentNameLabel] = mdNameOnMachineSet
		}
		if err := helper.Patch(ctx, machine); err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to apply %s label to Machine %q", v1alpha2.VirtualMachineReplicaSetNameLabel, machine.Name)
		}
	}

	// Remediate failed Machines by deleting them.
	/*var errs []error
	for _, machine := range filteredMachines {
		log := log.WithValues("Machine", klog.KObj(machine))
		// filteredMachines contains machines in deleting status to calculate correct status.
		// skip remediation for those in deleting status.
		if !machine.DeletionTimestamp.IsZero() {
			continue
		}
		if conditions.IsFalse(machine, clusterv1.MachineOwnerRemediatedCondition) {
			log.Info("Deleting machine because marked as unhealthy by the MachineHealthCheck controller")
			patch := client.MergeFrom(machine.DeepCopy())
			if err := r.Client.Delete(ctx, machine); err != nil {
				errs = append(errs, errors.Wrap(err, "failed to delete"))
				continue
			}
			conditions.MarkTrue(machine, clusterv1.MachineOwnerRemediatedCondition)
			if err := r.Client.Status().Patch(ctx, machine, patch); err != nil && !apierrors.IsNotFound(err) {
				errs = append(errs, errors.Wrap(err, "failed to update status"))
			}
		}
	}

	err = kerrors.NewAggregate(errs)
	if err != nil {
		log.Info("Failed while deleting unhealthy machines", "err", err)
		return ctrl.Result{}, errors.Wrap(err, "failed to remediate machines")
	}*/

	syncErr := r.syncReplicas(ctx, machineSet, filteredMachines)

	// Always updates status as machines come up or die.
	if err := r.updateStatus(ctx, machineSet, filteredMachines); err != nil {
		return ctrl.Result{}, errors.Wrapf(kerrors.NewAggregate([]error{err, syncErr}), "failed to update MachineSet's Status")
	}

	if syncErr != nil {
		return ctrl.Result{}, errors.Wrapf(syncErr, "failed to sync MachineSet replicas")
	}

	var replicas int32
	if machineSet.Spec.Replicas != nil {
		replicas = *machineSet.Spec.Replicas
	}

	// Resync the MachineSet after MinReadySeconds as a last line of defense to guard against clock-skew.
	// Clock-skew is an issue as it may impact whether an available replica is counted as a ready replica.
	// A replica is available if the amount of time since last transition exceeds MinReadySeconds.
	// If there was a clock skew, checking whether the amount of time since last transition to ready state
	// exceeds MinReadySeconds could be incorrect.
	// To avoid an available replica stuck in the ready state, we force a reconcile after MinReadySeconds,
	// at which point it should confirm any available replica to be available.
	if machineSet.Spec.MinReadySeconds > 0 &&
		machineSet.Status.ReadyReplicas == replicas &&
		machineSet.Status.AvailableReplicas != replicas {
		return ctrl.Result{RequeueAfter: time.Duration(machineSet.Spec.MinReadySeconds) * time.Second}, nil
	}

	// Quickly reconcile until the nodes become Ready.
	if machineSet.Status.ReadyReplicas != replicas {
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// syncReplicas scales Machine resources up or down.
func (r *Reconciler) syncReplicas(ctx goctx.Context, ms *v1alpha2.VirtualMachineReplicaSet, machines []*vmopv1.VirtualMachine) error {
	log := ctrl.LoggerFrom(ctx)
	if ms.Spec.Replicas == nil {
		return errors.Errorf("the Replicas field in Spec for machineset %v is nil, this should not be allowed", ms.Name)
	}
	diff := len(machines) - int(*(ms.Spec.Replicas))
	switch {
	case diff < 0:
		diff *= -1
		log.Info(fmt.Sprintf("MachineSet is scaling up to %d replicas by creating %d machines", *(ms.Spec.Replicas), diff), "replicas", *(ms.Spec.Replicas), "machineCount", len(machines))
		var (
			machineList []*vmopv1.VirtualMachine
			errs        []error
		)

		for i := 0; i < diff; i++ {
			// Create a new logger so the global logger is not modified.
			log := log
			machine := r.getNewMachine(ms)

			if err := r.Client.Create(ctx, machine); err != nil {
				log.Error(err, "Error while creating a machine")
				r.recorder.Eventf(ms, corev1.EventTypeWarning, "FailedCreate", "Failed to create machine: %v", err)
				errs = append(errs, err)
				conditions.MarkFalse(ms, vmopv1.MachinesCreatedCondition, vmopv1.MachineCreationFailedReason,
					vmopv1.ConditionSeverityError, err.Error())
				continue
			}

			log.Info(fmt.Sprintf("Created machine %d of %d", i+1, diff), "Machine", klog.KObj(machine))
			r.recorder.Eventf(ms, corev1.EventTypeNormal, "SuccessfulCreate", "Created machine %q", machine.Name)
			machineList = append(machineList, machine)
		}

		if len(errs) > 0 {
			return kerrors.NewAggregate(errs)
		}
		return r.waitForMachineCreation(ctx, machineList)
	case diff > 0:
		log.Info(fmt.Sprintf("MachineSet is scaling down to %d replicas by deleting %d machines", *(ms.Spec.Replicas), diff), "replicas", *(ms.Spec.Replicas), "machineCount", len(machines), "deletePolicy", "oldestFirst" /*ms.Spec.DeletePolicy*/)

		deletePriorityFunc, err := getDeletePriorityFunc(ms)
		if err != nil {
			return err
		}

		var errs []error
		machinesToDelete := getMachinesToDeletePrioritized(machines, diff, deletePriorityFunc)
		for i, machine := range machinesToDelete {
			log := log.WithValues("Machine", klog.KObj(machine))
			if machine.GetDeletionTimestamp().IsZero() {
				log.Info(fmt.Sprintf("Deleting machine %d of %d", i+1, diff))
				if err := r.Client.Delete(ctx, machine); err != nil {
					log.Error(err, "Unable to delete Machine")
					r.recorder.Eventf(ms, corev1.EventTypeWarning, "FailedDelete", "Failed to delete machine %q: %v", machine.Name, err)
					errs = append(errs, err)
					continue
				}
				r.recorder.Eventf(ms, corev1.EventTypeNormal, "SuccessfulDelete", "Deleted machine %q", machine.Name)
			} else {
				log.Info(fmt.Sprintf("Waiting for machine %d of %d to be deleted", i+1, diff))
			}
		}

		if len(errs) > 0 {
			return kerrors.NewAggregate(errs)
		}
		return r.waitForMachineDeletion(ctx, machinesToDelete)
	}

	return nil
}

// getNewMachine creates a new Machine object. The name of the newly created resource is going
// to be created by the API server, we set the generateName field.
func (r *Reconciler) getNewMachine(machineSet *v1alpha2.VirtualMachineReplicaSet) *vmopv1.VirtualMachine {
	gv := vmopv1.SchemeGroupVersion
	machine := &vmopv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", machineSet.Name),
			// Note: by setting the ownerRef on creation we signal to the Machine controller that this is not a stand-alone Machine.
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(machineSet, machineSetKind)},
			Namespace:       machineSet.Namespace,
			Labels:          make(map[string]string),
			Annotations:     machineSet.Spec.Template.Annotations,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       gv.WithKind("VirtualMachine").Kind,
			APIVersion: gv.String(),
		},
		Spec: machineSet.Spec.Template.Spec,
	}

	// Set the labels from machineSet.Spec.Template.Labels as labels for the new Machine.
	// Note: We can't just set `machineSet.Spec.Template.Labels` directly and thus "share" the labels
	// map between Machine and machineSet.Spec.Template.Labels. This would mean that adding the
	// MachineSetLabelName and MachineDeploymentLabelName later on the Machine would also add the labels
	// to machineSet.Spec.Template.Labels and thus modify the labels of the MachineSet.
	for k, v := range machineSet.Spec.Template.Labels {
		machine.Labels[k] = v
	}

	// Enforce that the MachineSetLabelName label is set
	// Note: the MachineSetLabelName is added by the default webhook to MachineSet.spec.template.labels if a spec.selector is empty.
	machine.Labels[v1alpha2.VirtualMachineReplicaSetNameLabel] = MustFormatValue(machineSet.Name)
	// Propagate the MachineDeploymentNameLabel from MachineSet to Machine if it exists.
	if mdName, ok := machineSet.Labels[v1alpha2.VirtualMachineDeploymentNameLabel]; ok {
		machine.Labels[v1alpha2.VirtualMachineDeploymentNameLabel] = mdName
	}

	return machine
}

// shouldExcludeMachine returns true if the machine should be filtered out, false otherwise.
func shouldExcludeMachine(machineSet *v1alpha2.VirtualMachineReplicaSet, machine *vmopv1.VirtualMachine) bool {
	if metav1.GetControllerOf(machine) != nil && !metav1.IsControlledBy(machine, machineSet) {
		return true
	}

	return false
}

// adoptOrphan sets the MachineSet as a controller OwnerReference to the Machine.
func (r *Reconciler) adoptOrphan(ctx goctx.Context, machineSet *v1alpha2.VirtualMachineReplicaSet, machine *vmopv1.VirtualMachine) error {
	patch := client.MergeFrom(machine.DeepCopy())
	newRef := *metav1.NewControllerRef(machineSet, machineSetKind)
	machine.OwnerReferences = append(machine.OwnerReferences, newRef)
	return r.Client.Patch(ctx, machine, patch)
}

func (r *Reconciler) waitForMachineCreation(ctx goctx.Context, machineList []*vmopv1.VirtualMachine) error {
	log := ctrl.LoggerFrom(ctx)

	for i := 0; i < len(machineList); i++ {
		machine := machineList[i]
		pollErr := wait.PollImmediate(stateConfirmationInterval, stateConfirmationTimeout, func() (bool, error) {
			key := client.ObjectKey{Namespace: machine.Namespace, Name: machine.Name}
			if err := r.Client.Get(ctx, key, &vmopv1.VirtualMachine{}); err != nil {
				if apierrors.IsNotFound(err) {
					return false, nil
				}
				return false, err
			}

			return true, nil
		})

		if pollErr != nil {
			log.Error(pollErr, "Failed waiting for machine object to be created")
			return errors.Wrap(pollErr, "failed waiting for machine object to be created")
		}
	}

	return nil
}

func (r *Reconciler) waitForMachineDeletion(ctx goctx.Context, machineList []*vmopv1.VirtualMachine) error {
	log := ctrl.LoggerFrom(ctx)

	for i := 0; i < len(machineList); i++ {
		machine := machineList[i]
		pollErr := wait.PollImmediate(stateConfirmationInterval, stateConfirmationTimeout, func() (bool, error) {
			m := &vmopv1.VirtualMachine{}
			key := client.ObjectKey{Namespace: machine.Namespace, Name: machine.Name}
			err := r.Client.Get(ctx, key, m)
			if apierrors.IsNotFound(err) || !m.DeletionTimestamp.IsZero() {
				return true, nil
			}
			return false, err
		})

		if pollErr != nil {
			log.Error(pollErr, "Failed waiting for machine object to be deleted")
			return errors.Wrap(pollErr, "failed waiting for machine object to be deleted")
		}
	}
	return nil
}

// MachineToMachineSets is a handler.ToRequestsFunc to be used to enqueue requests for reconciliation
// for MachineSets that might adopt an orphaned Machine.
func (r *Reconciler) MachineToMachineSets(o client.Object) []ctrl.Request {
	result := []ctrl.Request{}

	m, ok := o.(*vmopv1.VirtualMachine)
	if !ok {
		panic(fmt.Sprintf("Expected a Machine but got a %T", o))
	}

	// This won't log unless the global logger is set
	ctx := goctx.Background()
	log := ctrl.LoggerFrom(ctx, "Machine", klog.KObj(m))

	// Check if the controller reference is already set and
	// return an empty result when one is found.
	for _, ref := range m.ObjectMeta.OwnerReferences {
		if ref.Controller != nil && *ref.Controller {
			return result
		}
	}

	mss, err := r.getMachineSetsForMachine(ctx, m)
	if err != nil {
		log.Error(err, "Failed getting MachineSets for Machine")
		return nil
	}
	if len(mss) == 0 {
		return nil
	}

	for _, ms := range mss {
		name := client.ObjectKey{Namespace: ms.Namespace, Name: ms.Name}
		result = append(result, ctrl.Request{NamespacedName: name})
	}

	return result
}

func (r *Reconciler) getMachineSetsForMachine(ctx goctx.Context, m *vmopv1.VirtualMachine) ([]*v1alpha2.VirtualMachineReplicaSet, error) {
	if len(m.Labels) == 0 {
		return nil, fmt.Errorf("machine %v has no labels, this is unexpected", client.ObjectKeyFromObject(m))
	}

	msList := &v1alpha2.VirtualMachineReplicaSetList{}
	if err := r.Client.List(ctx, msList, client.InNamespace(m.Namespace)); err != nil {
		return nil, errors.Wrapf(err, "failed to list MachineSets")
	}

	var mss []*v1alpha2.VirtualMachineReplicaSet
	for idx := range msList.Items {
		ms := &msList.Items[idx]
		if HasMatchingLabels(*ms.Spec.Selector, m.Labels) {
			mss = append(mss, ms)
		}
	}

	return mss, nil
}

// shouldAdopt returns true if the MachineSet should be adopted as a stand-alone MachineSet directly owned by the Cluster.
/*func (r *Reconciler) shouldAdopt(ms *vmopv1.VirtualMachineReplicaSet) bool {
	// if the MachineSet is controlled by a MachineDeployment, or if it is a stand-alone MachinesSet directly owned by the Cluster, then no-op.
	if util.HasOwner(ms.OwnerReferences, clusterv1.GroupVersion.String(), []string{"MachineDeployment", "Cluster"}) {
		return false
	}

	// If the MachineSet is originated by a MachineDeployment object, it should not be adopted directly by the Cluster as a stand-alone MachineSet.
	// Note: this is required because after restore from a backup both the MachineSet controller and the
	// MachineDeployment controller are racing to adopt MachineSets, see https://github.com/kubernetes-sigs/cluster-api/issues/7529
	if _, ok := ms.Labels[clusterv1.MachineDeploymentLabelName]; ok {
		return false
	}
	return true
}*/

// updateStatus updates the Status field for the MachineSet
// It checks for the current state of the replicas and updates the Status of the MachineSet.
func (r *Reconciler) updateStatus(ctx goctx.Context, ms *v1alpha2.VirtualMachineReplicaSet, filteredMachines []*vmopv1.VirtualMachine) error {
	log := ctrl.LoggerFrom(ctx)
	newStatus := ms.Status.DeepCopy()

	// TODO(muchhals): Will be needed if we want to include support for kubectl scale
	// Copy label selector to its status counterpart in string format.
	// This is necessary for CRDs including scale subresources.
	/*selector, err := metav1.LabelSelectorAsSelector(&ms.Spec.Selector)
	if err != nil {
		return errors.Wrapf(err, "failed to update status for MachineSet %s/%s", ms.Namespace, ms.Name)
	}
	newStatus.Selector = selector.String()*/

	// Count the number of machines that have labels matching the labels of the machine
	// template of the replica set, the matching machines may have more
	// labels than are in the template. Because the label of machineTemplateSpec is
	// a superset of the selector of the replica set, so the possible
	// matching machines must be part of the filteredMachines.
	fullyLabeledReplicasCount := 0
	readyReplicasCount := 0
	availableReplicasCount := 0
	desiredReplicas := *ms.Spec.Replicas
	templateLabel := labels.Set(ms.Spec.Template.Labels).AsSelectorPreValidated()

	for _, machine := range filteredMachines {
		//log := log.WithValues("Machine", klog.KObj(machine))

		if templateLabel.Matches(labels.Set(machine.Labels)) {
			fullyLabeledReplicasCount++
			readyReplicasCount++
			availableReplicasCount++
		}

		// TODO(muchhals): How do we decide when a Virtual Machine becomes Ready
		/*if machine.Status.NodeRef == nil {
			log.V(4).Info("Waiting for the machine controller to set status.NodeRef on the Machine")
			continue
		}

		node, err := r.getMachineNode(ctx, cluster, machine)
		if err != nil && machine.GetDeletionTimestamp().IsZero() {
			log.Error(err, "Unable to retrieve Node status", "node", klog.KObj(node))
			continue
		}

		if noderefutil.IsNodeReady(node) {
			readyReplicasCount++
			if noderefutil.IsNodeAvailable(node, ms.Spec.MinReadySeconds, metav1.Now()) {
				availableReplicasCount++
			}
		} else if machine.GetDeletionTimestamp().IsZero() {
			log.Info("Waiting for the Kubernetes node on the machine to report ready state")
		}*/
	}

	newStatus.Replicas = int32(len(filteredMachines))
	newStatus.FullyLabeledReplicas = int32(fullyLabeledReplicasCount)
	newStatus.ReadyReplicas = int32(readyReplicasCount)
	newStatus.AvailableReplicas = int32(availableReplicasCount)

	// Copy the newly calculated status into the machineset
	if ms.Status.Replicas != newStatus.Replicas ||
		ms.Status.FullyLabeledReplicas != newStatus.FullyLabeledReplicas ||
		ms.Status.ReadyReplicas != newStatus.ReadyReplicas ||
		ms.Status.AvailableReplicas != newStatus.AvailableReplicas ||
		ms.Generation != ms.Status.ObservedGeneration {
		log.V(4).Info("Updating status: " +
			fmt.Sprintf("replicas %d->%d (need %d), ", ms.Status.Replicas, newStatus.Replicas, desiredReplicas) +
			fmt.Sprintf("fullyLabeledReplicas %d->%d, ", ms.Status.FullyLabeledReplicas, newStatus.FullyLabeledReplicas) +
			fmt.Sprintf("readyReplicas %d->%d, ", ms.Status.ReadyReplicas, newStatus.ReadyReplicas) +
			fmt.Sprintf("availableReplicas %d->%d, ", ms.Status.AvailableReplicas, newStatus.AvailableReplicas) +
			fmt.Sprintf("observedGeneration %v->%v", ms.Status.ObservedGeneration, ms.Generation))

		// Save the generation number we acted on, otherwise we might wrongfully indicate
		// that we've seen a spec update when we retry.
		newStatus.ObservedGeneration = ms.Generation
		newStatus.DeepCopyInto(&ms.Status)
	}
	switch {
	// We are scaling up
	case newStatus.Replicas < desiredReplicas:
		conditions.MarkFalse(ms, vmopv1.ResizedCondition, vmopv1.ScalingUpReason, vmopv1.ConditionSeverityWarning, "Scaling up MachineSet to %d replicas (actual %d)", desiredReplicas, newStatus.Replicas)
	// We are scaling down
	case newStatus.Replicas > desiredReplicas:
		conditions.MarkFalse(ms, vmopv1.ResizedCondition, vmopv1.ScalingDownReason, vmopv1.ConditionSeverityWarning, "Scaling down MachineSet to %d replicas (actual %d)", desiredReplicas, newStatus.Replicas)
		// This means that there was no error in generating the desired number of machine objects
		conditions.MarkTrue(ms, vmopv1.MachinesCreatedCondition)
	default:
		// Make sure last resize operation is marked as completed.
		// NOTE: we are checking the number of machines ready so we report resize completed only when the machines
		// are actually provisioned (vs reporting completed immediately after the last machine object is created). This convention is also used by KCP.
		if newStatus.ReadyReplicas == newStatus.Replicas {
			if conditions.IsFalse(ms, vmopv1.ResizedCondition) {
				log.Info("All the replicas are ready", "replicas", newStatus.ReadyReplicas)
			}
			conditions.MarkTrue(ms, vmopv1.ResizedCondition)
		}
		// This means that there was no error in generating the desired number of machine objects
		conditions.MarkTrue(ms, vmopv1.MachinesCreatedCondition)
	}

	// TODO(muchhals): Set aggregate condition based on the condition of the individual Virtual Machines
	// Aggregate the operational state of all the machines; while aggregating we are adding the
	// source ref (reason@machine/name) so the problem can be easily tracked down to its source machine.
	//conditions.SetAggregate(ms, vmopv1.MachinesReadyCondition, collections.FromMachines(filteredMachines...).ConditionGetters(), conditions.AddSourceRef(), conditions.WithStepCounterIf(false))

	return nil
}

// MustFormatValue returns the passed inputLabelValue if it meets the standards for a Kubernetes label value.
// If the name is not a valid label value this function returns a hash which meets the requirements.
func MustFormatValue(str string) string {
	// a valid Kubernetes label value must:
	// - be less than 64 characters long.
	// - be an empty string OR consist of alphanumeric characters, '-', '_' or '.'.
	// - start and end with an alphanumeric character
	if len(validation.IsValidLabelValue(str)) == 0 {
		return str
	}
	hasher := fnv.New32a()
	_, err := hasher.Write([]byte(str))
	if err != nil {
		// At time of writing the implementation of fnv's Write function can never return an error.
		// If this changes in a future go version this function will panic.
		panic(err)
	}
	return fmt.Sprintf("hash_%s_z", base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hasher.Sum(nil)))
}

// MustEqualValue returns true if the actualLabelValue equals either the inputLabelValue or the hashed
// value of the inputLabelValue.
func MustEqualValue(str, labelValue string) bool {
	return labelValue == MustFormatValue(str)
}

func HasMatchingLabels(matchSelector metav1.LabelSelector, matchLabels map[string]string) bool {
	// This should never fail, validating webhook should catch this first
	selector, err := metav1.LabelSelectorAsSelector(&matchSelector)
	if err != nil {
		return false
	}
	// If a nil or empty selector creeps in, it should match nothing, not everything.
	if selector.Empty() {
		return false
	}
	if !selector.Matches(labels.Set(matchLabels)) {
		return false
	}
	return true
}

/*func (r *Reconciler) getMachineNode(ctx goctx.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) (*corev1.Node, error) {
	remoteClient, err := r.Tracker.GetClient(ctx, util.ObjectKey(cluster))
	if err != nil {
		return nil, err
	}
	node := &corev1.Node{}
	if err := remoteClient.Get(ctx, client.ObjectKey{Name: machine.Status.NodeRef.Name}, node); err != nil {
		return nil, errors.Wrapf(err, "error retrieving node %s for machine %s/%s", machine.Status.NodeRef.Name, machine.Namespace, machine.Name)
	}
	return node, nil
}
*/
/*func reconcileExternalTemplateReference(ctx goctx.Context, c client.Client, apiReader client.Reader, cluster *clusterv1.Cluster, ref *corev1.ObjectReference) error {
	if !strings.HasSuffix(ref.Kind, clusterv1.TemplateSuffix) {
		return nil
	}

	if err := utilconversion.UpdateReferenceAPIContract(ctx, c, apiReader, ref); err != nil {
		return err
	}

	obj, err := external.Get(ctx, c, ref, cluster.Namespace)
	if err != nil {
		return err
	}

	patchHelper, err := patch.NewHelper(obj, c)
	if err != nil {
		return err
	}

	obj.SetOwnerReferences(util.EnsureOwnerRef(obj.GetOwnerReferences(), metav1.OwnerReference{
		APIVersion: clusterv1.GroupVersion.String(),
		Kind:       "Cluster",
		Name:       cluster.Name,
		UID:        cluster.UID,
	}))

	return patchHelper.Patch(ctx, obj)
}*/
