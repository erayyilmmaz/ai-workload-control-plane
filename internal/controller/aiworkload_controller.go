// SPDX-License-Identifier: Apache-2.0
package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	platformv1alpha1 "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/resource"
)

// AIWorkloadReconciler executes pure plans; a nil Builder deliberately creates no children.
type AIWorkloadReconciler struct {
	client.Client
	WatchNamespace string
	ControllerName string
	Scheme         *runtime.Scheme
	Builder        resource.Builder
	Recorder       events.EventRecorder
}

// +kubebuilder:rbac:groups=platform.example.io,namespace=awcp-workloads,resources=aiworkloads,verbs=get;list;watch
// +kubebuilder:rbac:groups=platform.example.io,namespace=awcp-workloads,resources=aiworkloads/status,verbs=patch;update
// +kubebuilder:rbac:groups=apps,namespace=awcp-workloads,resources=deployments,verbs=get;list;watch;create;patch;update
// +kubebuilder:rbac:groups="",namespace=awcp-workloads,resources=serviceaccounts,verbs=get;list;watch;create;patch;update
// +kubebuilder:rbac:groups="",namespace=awcp-workloads,resources=services,verbs=get;list;watch;create;patch;update;delete
// +kubebuilder:rbac:groups=networking.k8s.io,namespace=awcp-workloads,resources=networkpolicies,verbs=get;list;watch;create;patch;update;delete
// +kubebuilder:rbac:groups="",namespace=awcp-workloads,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups=events.k8s.io,namespace=awcp-workloads,resources=events,verbs=create;patch;update

// Reconcile guards parent lifecycle, builds/validates a plan, then applies in dependency order.
func (r *AIWorkloadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if r.WatchNamespace == "" {
		return ctrl.Result{}, errors.New("watch namespace is required")
	}
	if req.Namespace != r.WatchNamespace {
		return ctrl.Result{}, nil
	}
	var workload platformv1alpha1.AIWorkload
	if err := r.Get(ctx, req.NamespacedName, &workload); err != nil {
		return ctrl.Result{}, retryError(client.IgnoreNotFound(err))
	}
	logger := log.FromContext(ctx).WithValues("namespace", req.Namespace, "name", req.Name, "generation", workload.Generation)
	ctx = log.IntoContext(ctx, logger)
	if !workload.DeletionTimestamp.IsZero() || r.Builder == nil {
		return ctrl.Result{}, nil
	}
	plan, err := r.Builder.Build(workload.DeepCopy())
	if err == nil {
		plan, err = ValidatePlan(&workload, plan)
	}
	if err == nil {
		engine := Engine{Client: r.Client, Scheme: r.Scheme}
		for _, intent := range plan {
			var outcome Outcome
			outcome, err = engine.Apply(ctx, &workload, intent)
			if err != nil {
				break
			}
			logger.V(1).Info("Reconciled child", "kind", childOrder(intent.Object), "outcome", outcome)
		}
	}
	if err == nil {
		err = r.validateSecretReferences(ctx, &workload)
	}
	if errors.Is(err, ErrChildDeleting) {
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
	}
	if err != nil {
		// A disabled Service must not leave stale discovery data even when its
		// deletion is blocked by a foreign owner or another apply failure.
		if !resource.ServiceEnabled(&workload) {
			if statusErr := r.syncServiceEndpoint(ctx, &workload, ""); statusErr != nil {
				return ctrl.Result{}, retryError(statusErr)
			}
		}
		return r.reportFailure(ctx, &workload, err)
	}
	if err = r.syncServiceEndpoint(ctx, &workload, resource.ServiceEndpoint(&workload)); err != nil {
		return ctrl.Result{}, retryError(err)
	}
	if err = r.clearFailure(ctx, &workload); err != nil {
		return ctrl.Result{}, retryError(err)
	}
	return ctrl.Result{}, nil
}

func (r *AIWorkloadReconciler) syncServiceEndpoint(ctx context.Context, workload *platformv1alpha1.AIWorkload, endpoint string) error {
	if workload.Status.Endpoint == endpoint {
		return nil
	}
	before := workload.DeepCopy()
	workload.Status.Endpoint = endpoint
	if _, err := r.patchStatus(ctx, before, workload); err != nil {
		return fmt.Errorf("patch Service endpoint: %w", err)
	}
	return nil
}

// SetupWithManager registers the primary watch with the namespace-scoped cache.
func (r *AIWorkloadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.ControllerName == "" {
		r.ControllerName = "aiworkload"
	}
	if r.Scheme == nil {
		r.Scheme = mgr.GetScheme()
	}
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorder("awcp-controller")
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &platformv1alpha1.AIWorkload{}, SecretReferenceIndex, secretIndex); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.AIWorkload{}, builder.WithPredicates(primaryPredicate())).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&networkingv1.NetworkPolicy{}).
		WatchesMetadata(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.RequestsForSecret)).
		Named(r.ControllerName).
		Complete(r)
}
