package virtualmachinedeployment

import (
	goctx "context"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	vmopv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	"github.com/vmware-tanzu/vm-operator/pkg/context"
	"github.com/vmware-tanzu/vm-operator/pkg/patch"
	"github.com/vmware-tanzu/vm-operator/pkg/record"
)

const finalizerName = "virtualmachinedeployment.vmoperator.vmware.com"

// AddToManager adds this package's controller to the provided manager.
func AddToManager(ctx *context.ControllerManagerContext, mgr manager.Manager) error {
	var (
		controlledType     = &vmopv1.VirtualMachineDeployment{}
		controlledTypeName = reflect.TypeOf(controlledType).Elem().Name()

		controllerNameShort = fmt.Sprintf("%s-controller", strings.ToLower(controlledTypeName))
		controllerNameLong  = fmt.Sprintf("%s/%s/%s", ctx.Namespace, ctx.Name, controllerNameShort)
	)

	r := NewReconciler(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName(controlledTypeName),
		record.New(mgr.GetEventRecorderFor(controllerNameLong)),
	)

	builder := ctrl.NewControllerManagedBy(mgr).
		For(controlledType).
		Owns(&vmopv1.VirtualMachineReplicaSet{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: ctx.MaxConcurrentReconciles})

	return builder.Complete(r)
}

func NewReconciler(
	client client.Client,
	logger logr.Logger,
	recorder record.Recorder) *Reconciler {
	return &Reconciler{
		Client:   client,
		logger:   logger,
		recorder: recorder,
	}
}

var _ reconcile.Reconciler = &Reconciler{}

type Reconciler struct {
	client.Client
	logger   logr.Logger
	recorder record.Recorder
}

// +kubebuilder:rbac:groups=vmoperator.vmware.com,resources=virtualmachinedeployments,verbs=get;list;watch;patch;
// +kubebuilder:rbac:groups=vmoperator.vmware.com,resources=virtualmachinedeployments/status,verbs=get;update;patch;
// +kubebuilder:rbac:groups=vmoperator.vmware.com,resources=virtualmachinereplicasets,verbs=create;delete;get;list;watch;patch;update;
// +kubebuilder:rbac:groups=vmoperator.vmware.com,resources=virtualmachinereplicasets/status,verbs=get;patch;update;

func (r *Reconciler) Reconcile(ctx goctx.Context, request ctrl.Request) (_ ctrl.Result, reterr error) {
	vmDeployment := &vmopv1.VirtualMachineDeployment{}
	if err := r.Get(ctx, request.NamespacedName, vmDeployment); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	vmDeploymentCtx := &context.VirtualMachineDeploymentContext{
		Context:                  goctx.WithValue(ctx, "vmop-object", vmDeployment),
		Logger:                   ctrl.Log.WithName("VirtualMachineDeployment").WithValues("name", vmDeployment.NamespacedName()),
		VirtualMachineDeployment: vmDeployment,
	}

	patchHelper, err := patch.NewHelper(vmDeployment, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to init patch helper for %s", vmDeploymentCtx.String())
	}

	defer func() {
		if err := patchHelper.Patch(ctx, vmDeployment); err != nil {
			if reterr == nil {
				reterr = err
			}
			vmDeploymentCtx.Logger.Error(err, "patch failed")
		}
	}()

	// Ignore deleted MachineDeployments, this can happen when foregroundDeletion
	// is enabled
	if !vmDeployment.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, r.ReconcileNormal(vmDeploymentCtx)
}

func (r *Reconciler) ReconcileNormal(ctx *context.VirtualMachineDeploymentContext) (reterr error) {
	/*if !controllerutil.ContainsFinalizer(ctx.VirtualMachineDeployment, finalizerName) {
		// The finalizer must be present before proceeding in order to ensure that the VM will
		// be cleaned up. Return immediately after here to let the patcher helper update the
		// object, and then we'll proceed on the next reconciliation.
		controllerutil.AddFinalizer(ctx.VirtualMachineDeployment, finalizerName)
		return nil
	}*/

	ctx.Logger.Info("Reconciling VirtualMachineDeployment")
	vmDeploymentObj := ctx.Value("vmop-object")
	vmDeployment, _ := vmDeploymentObj.(*vmopv1.VirtualMachineDeployment)
	return r.reconcile(ctx, vmDeployment)
}

/*
	func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
		log := ctrl.LoggerFrom(ctx)

		// Fetch the MachineDeployment instance.
		deployment := &clusterv1.MachineDeployment{}
		if err := r.Client.Get(ctx, req.NamespacedName, deployment); err != nil {
			if apierrors.IsNotFound(err) {
				// Object not found, return.  Created objects are automatically garbage collected.
				// For additional cleanup logic use finalizers.
				return ctrl.Result{}, nil
			}
			// Error reading the object - requeue the request.
			return ctrl.Result{}, err
		}

		log = log.WithValues("Cluster", klog.KRef(deployment.Namespace, deployment.Spec.ClusterName))
		ctx = ctrl.LoggerInto(ctx, log)

		cluster, err := util.GetClusterByName(ctx, r.Client, deployment.Namespace, deployment.Spec.ClusterName)
		if err != nil {
			return ctrl.Result{}, err
		}

		// Return early if the object or Cluster is paused.
		if annotations.IsPaused(cluster, deployment) {
			log.Info("Reconciliation is paused for this object")
			return ctrl.Result{}, nil
		}

		// Initialize the patch helper
		patchHelper, err := patch.NewHelper(deployment, r.Client)
		if err != nil {
			return ctrl.Result{}, err
		}

		defer func() {
			// Always attempt to patch the object and status after each reconciliation.
			// Patch ObservedGeneration only if the reconciliation completed successfully
			patchOpts := []patch.Option{}
			if reterr == nil {
				patchOpts = append(patchOpts, patch.WithStatusObservedGeneration{})
			}
			if err := patchMachineDeployment(ctx, patchHelper, deployment, patchOpts...); err != nil {
				reterr = kerrors.NewAggregate([]error{reterr, err})
			}
		}()

		// Ignore deleted MachineDeployments, this can happen when foregroundDeletion
		// is enabled
		if !deployment.DeletionTimestamp.IsZero() {
			return ctrl.Result{}, nil
		}

		result, err := r.reconcile(ctx, cluster, deployment)
		if err != nil {
			log.Error(err, "Failed to reconcile MachineDeployment")
			r.recorder.Eventf(deployment, corev1.EventTypeWarning, "ReconcileError", "%v", err)
		}
		return result, err
	}
*/
func (r *Reconciler) reconcile(ctx goctx.Context, md *vmopv1.VirtualMachineDeployment) error {
	log := ctrl.LoggerFrom(ctx)
	log.V(4).Info("Reconcile MachineDeployment")

	msList, err := r.getVirtualMachineReplicaSetsForDeployment(ctx, md)
	if err != nil {
		return err
	}

	// If not already present, add a label specifying the MachineDeployment name to MachineSets.
	// Ensure all required labels exist on the controlled MachineSets.
	for idx := range msList {
		machineSet := msList[idx]
		if name, ok := machineSet.Labels[vmopv1.VirtualMachineDeploymentNameLabel]; ok && name == md.Name {
			continue
		}

		helper, err := patch.NewHelper(machineSet, r.Client)
		if err != nil {
			return errors.Wrapf(err, "failed to apply %s label to MachineSet %q", vmopv1.VirtualMachineDeploymentNameLabel, machineSet.Name)
		}
		machineSet.Labels[vmopv1.VirtualMachineDeploymentNameLabel] = md.Name
		if err := helper.Patch(ctx, machineSet); err != nil {
			return errors.Wrapf(err, "failed to apply %s label to MachineSet %q", vmopv1.VirtualMachineDeploymentNameLabel, machineSet.Name)
		}
	}

	// Loop over all MachineSets and cleanup managed fields.
	// We do this so that MachineSets that were created/patched before (< v1.4.0) the controller adopted
	// Server-Side-Apply (SSA) can also work with SSA. Otherwise, fields would be co-owned by our "old" "manager" and
	// "capi-machinedeployment" and then we would not be able to e.g. drop labels and annotations.
	// Note: We are cleaning up managed fields for all MachineSets, so we're able to remove this code in a few
	// Cluster API releases. If we do this only for selected MachineSets, we would have to keep this code forever.
	/*for idx := range msList {
		machineSet := msList[idx]
		if err := ssa.CleanUpManagedFieldsForSSAAdoption(ctx, r.Client, machineSet, machineDeploymentManagerName); err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to clean up managedFields of MachineSet %s", klog.KObj(machineSet))
		}
	}*/

	/*if md.Spec.Paused {
		return ctrl.Result{}, r.sync(ctx, md, msList)
	}*/

	/*if md.Spec.Strategy == nil {
		return ctrl.Result{}, errors.Errorf("missing MachineDeployment strategy")
	}*/

	if md.Spec.Strategy.Type == vmopv1.VirtualMachineDeploymentStrategyTypeRollingUpdate {
		if md.Spec.Strategy.RollingUpdate == nil {
			return errors.Errorf("missing MachineDeployment settings for strategy type: %s", md.Spec.Strategy.Type)
		}
		return r.rolloutRolling(ctx, md, msList)
	}

	// TODO(muchhals): Implement OnDelete
	/*if md.Spec.Strategy.Type == vmopv1.VirtualMachineDeploymentStrategyTypeRecreate {
		return ctrl.Result{}, r.rolloutOnDelete(ctx, md, msList)
	}*/

	return errors.Errorf("unexpected deployment strategy type: %s", md.Spec.Strategy.Type)
}

// getVirtualMachineReplicaSetsForDeployment returns a list of MachineSets associated with a MachineDeployment.
func (r *Reconciler) getVirtualMachineReplicaSetsForDeployment(ctx goctx.Context, md *vmopv1.VirtualMachineDeployment) ([]*vmopv1.VirtualMachineReplicaSet, error) {
	log := ctrl.LoggerFrom(ctx)

	// List all VirtualMachineReplicaSets to find those we own but that no longer match our selector.
	replicaSetsList := &vmopv1.VirtualMachineReplicaSetList{}
	if err := r.Client.List(ctx, replicaSetsList, client.InNamespace(md.Namespace)); err != nil {
		return nil, err
	}

	filtered := make([]*vmopv1.VirtualMachineReplicaSet, 0, len(replicaSetsList.Items))
	for idx := range replicaSetsList.Items {
		rs := &replicaSetsList.Items[idx]
		log.WithValues("VirtualMachineReplicaSet", klog.KObj(rs))
		selector, err := metav1.LabelSelectorAsSelector(md.Spec.Selector)
		if err != nil {
			log.Error(err, "Skipping VirtualMachineReplicaSet, failed to get label selector from spec selector")
			continue
		}

		// If a MachineDeployment with a nil or empty selector creeps in, it should match nothing, not everything.
		// TODO(muchhals): Is this check necessary?
		if selector.Empty() {
			log.Info("Skipping VirtualMachineReplicaSet as the selector is empty")
			continue
		}

		// Skip this VirtualMachineReplicaSet unless either selector matches or it has a controller ref pointing to this VirtualMachineDeployment
		if !selector.Matches(labels.Set(rs.Labels)) && !metav1.IsControlledBy(rs, md) {
			log.V(4).Info("Skipping VirtualMachineReplicaSet, label mismatch")
			continue
		}

		// Attempt to adopt VirtualMachineReplicaSet if it meets previous conditions and it has no controller references.
		if metav1.GetControllerOf(rs) == nil {
			if err := r.adoptOrphan(ctx, md, rs); err != nil {
				log.Error(err, "Failed to adopt VirtualMachineReplicaSet into MachineDeployment")
				r.recorder.Eventf(md, corev1.EventTypeWarning, "FailedAdopt", "Failed to adopt VirtualMachineReplicaSet %q: %v", rs.Name, err)
				continue
			}
			log.Info("Adopted VirtualMachineReplicaSet into MachineDeployment")
			r.recorder.Eventf(md, corev1.EventTypeNormal, "SuccessfulAdopt", "Adopted VirtualMachineReplicaSet %q", rs.Name)
		}

		if !metav1.IsControlledBy(rs, md) {
			continue
		}

		filtered = append(filtered, rs)
	}

	return filtered, nil
}

// adoptOrphan sets the MachineDeployment as a controller OwnerReference to the MachineSet.
func (r *Reconciler) adoptOrphan(ctx goctx.Context, deployment *vmopv1.VirtualMachineDeployment, replicaSet *vmopv1.VirtualMachineReplicaSet) error {
	patch := client.MergeFrom(replicaSet.DeepCopy())
	// TODO(muchhals): Does Re-setting the same owner reference work?
	if err := controllerutil.SetControllerReference(deployment, replicaSet, r.Client.Scheme()); err != nil {
		return err
	}
	return r.Client.Patch(ctx, replicaSet, patch)
}

/*
// getMachineDeploymentsForMachineSet returns a list of MachineDeployments that could potentially match a MachineSet.
func (r *Reconciler) getMachineDeploymentsForMachineSet(ctx context.Context, ms *clusterv1.MachineSet) []*clusterv1.MachineDeployment {
	log := ctrl.LoggerFrom(ctx)

	if len(ms.Labels) == 0 {
		log.V(2).Info("No MachineDeployments found for MachineSet because it has no labels", "MachineSet", klog.KObj(ms))
		return nil
	}

	dList := &clusterv1.MachineDeploymentList{}
	if err := r.Client.List(ctx, dList, client.InNamespace(ms.Namespace)); err != nil {
		log.Error(err, "Failed to list MachineDeployments")
		return nil
	}

	deployments := make([]*clusterv1.MachineDeployment, 0, len(dList.Items))
	for idx, d := range dList.Items {
		selector, err := metav1.LabelSelectorAsSelector(&d.Spec.Selector)
		if err != nil {
			continue
		}

		// If a deployment with a nil or empty selector creeps in, it should match nothing, not everything.
		if selector.Empty() || !selector.Matches(labels.Set(ms.Labels)) {
			continue
		}

		deployments = append(deployments, &dList.Items[idx])
	}

	return deployments
}

// MachineSetToDeployments is a handler.ToRequestsFunc to be used to enqueue requests for reconciliation
// for MachineDeployments that might adopt an orphaned MachineSet.
func (r *Reconciler) MachineSetToDeployments(ctx context.Context, o client.Object) []ctrl.Request {
	result := []ctrl.Request{}

	ms, ok := o.(*clusterv1.MachineSet)
	if !ok {
		panic(fmt.Sprintf("Expected a MachineSet but got a %T", o))
	}

	// Check if the controller reference is already set and
	// return an empty result when one is found.
	for _, ref := range ms.ObjectMeta.GetOwnerReferences() {
		if ref.Controller != nil && *ref.Controller {
			return result
		}
	}

	mds := r.getMachineDeploymentsForMachineSet(ctx, ms)
	if len(mds) == 0 {
		return nil
	}

	for _, md := range mds {
		name := client.ObjectKey{Namespace: md.Namespace, Name: md.Name}
		result = append(result, ctrl.Request{NamespacedName: name})
	}

	return result
}*/
