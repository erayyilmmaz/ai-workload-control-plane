// SPDX-License-Identifier: Apache-2.0
package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	platformv1alpha1 "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/capability"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/resource"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/telemetry"
)

// AIWorkloadReconciler executes pure plans; a nil Builder deliberately creates no children.
type AIWorkloadReconciler struct {
	client.Client
	// TenantReader uses direct API reads for the one GitOps-owned tenant profile
	// ConfigMap. It avoids widening the cache to arbitrary ConfigMaps.
	TenantReader client.Reader
	// ExternalSecretsReader performs narrow direct reads of optional ESO objects;
	// it is not a provider client and never reads Secret data.
	ExternalSecretsReader client.Reader
	// GatewayReader performs narrow direct reads of optional Gateway API objects.
	// It never creates Gateways or discovers cross-namespace routing authority.
	GatewayReader    client.Reader
	CapabilityLookup capability.ResourceLookup
	WatchNamespace   string
	WatchNamespaces  []string
	ControllerName   string
	Scheme           *runtime.Scheme
	Builder          resource.Builder
	Recorder         events.EventRecorder
	Telemetry        telemetry.Recorder
}

// +kubebuilder:rbac:groups=platform.example.io,namespace=awcp-workloads,resources=aiworkloads,verbs=get;list;watch
// +kubebuilder:rbac:groups=platform.example.io,namespace=awcp-workloads,resources=aiworkloads/status,verbs=patch;update
// +kubebuilder:rbac:groups=apps,namespace=awcp-workloads,resources=deployments,verbs=get;list;watch;create;patch;update
// +kubebuilder:rbac:groups="",namespace=awcp-workloads,resources=serviceaccounts,verbs=get;list;watch;create;patch;update
// +kubebuilder:rbac:groups="",namespace=awcp-workloads,resources=services,verbs=get;list;watch;create;patch;update;delete
// +kubebuilder:rbac:groups=networking.k8s.io,namespace=awcp-workloads,resources=networkpolicies,verbs=get;list;watch;create;patch;update;delete
// +kubebuilder:rbac:groups=autoscaling,namespace=awcp-workloads,resources=horizontalpodautoscalers,verbs=get;list;watch;create;patch;update;delete
// +kubebuilder:rbac:groups=policy,namespace=awcp-workloads,resources=poddisruptionbudgets,verbs=get;list;watch;create;patch;update;delete
// +kubebuilder:rbac:groups="",namespace=awcp-workloads,resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",namespace=awcp-workloads,resources=configmaps,resourceNames=awcp-tenant-profile,verbs=get
// +kubebuilder:rbac:groups=external-secrets.io,namespace=awcp-workloads,resources=externalsecrets;secretstores,verbs=get
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,namespace=awcp-workloads,resources=gateways,verbs=get
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,namespace=awcp-workloads,resources=httproutes,verbs=get;list;watch;create;patch;delete
// +kubebuilder:rbac:groups=events.k8s.io,namespace=awcp-workloads,resources=events,verbs=create;patch;update

// Reconcile guards parent lifecycle, builds/validates a plan, then applies in dependency order.
func (r *AIWorkloadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if len(r.effectiveWatchNamespaces()) == 0 {
		return ctrl.Result{}, errors.New("watch namespaces are required")
	}
	if !r.watchesNamespace(req.Namespace) {
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
	tenantBefore := workload.DeepCopy()
	if err := r.validateTenant(ctx, &workload); err != nil {
		return r.reportFailure(ctx, &workload, err)
	}
	if _, err := r.patchStatus(ctx, tenantBefore, &workload); err != nil {
		return r.reportFailure(ctx, &workload, err)
	}
	externalBefore := workload.DeepCopy()
	external, err := r.validateExternalSecrets(ctx, &workload)
	if err != nil {
		return r.reportFailure(ctx, &workload, err)
	}
	if _, err := r.patchStatus(ctx, externalBefore, &workload); err != nil {
		return r.reportFailure(ctx, &workload, err)
	}
	exposureBefore := workload.DeepCopy()
	exposure, err := r.validateExposure(ctx, &workload)
	if err != nil {
		return r.reportFailure(ctx, &workload, err)
	}
	if _, err := r.patchStatus(ctx, exposureBefore, &workload); err != nil {
		return r.reportFailure(ctx, &workload, err)
	}
	desired := workload.DeepCopy()
	for _, reference := range external {
		desired.Spec.SecretRefs = append(desired.Spec.SecretRefs, reference.targetSecret)
	}
	plan, err := r.Builder.Build(desired)
	if err == nil && exposure.manageRoute {
		var route resource.Intent
		route, err = resource.HTTPRouteIntent(desired)
		if err == nil {
			plan = append(plan, route)
		}
	}
	if err == nil {
		plan, err = ValidatePlan(&workload, plan)
	}
	if err == nil {
		plan = decorateExternalSecretRevision(plan, externalSecretRevision(external))
	}
	if err == nil {
		engine := Engine{Client: r.Client, Scheme: r.Scheme}
		for _, intent := range plan {
			var outcome Outcome
			outcome, err = engine.Apply(ctx, &workload, intent)
			if err != nil {
				if r.Telemetry != nil {
					r.Telemetry.RecordResource(childKind(intent.Object), "error")
				}
				break
			}
			if r.Telemetry != nil {
				r.Telemetry.RecordResource(childKind(intent.Object), string(outcome))
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
		return r.reportFailure(ctx, &workload, err)
	}
	exposureBefore = workload.DeepCopy()
	exposureRequeue, err := r.observeExposure(ctx, &workload, exposure)
	if err != nil {
		return r.reportFailure(ctx, &workload, err)
	}
	if _, err := r.patchStatus(ctx, exposureBefore, &workload); err != nil {
		return r.reportFailure(ctx, &workload, err)
	}
	if err = r.observeAndReportStatus(ctx, &workload); err != nil {
		return r.reportFailure(ctx, &workload, err)
	}
	return ctrl.Result{RequeueAfter: exposureRequeue}, nil
}

func externalSecretRevision(references []resolvedExternalSecret) string {
	if len(references) == 0 {
		return ""
	}
	hash := sha256.New()
	for _, reference := range references {
		_, _ = hash.Write([]byte(reference.targetSecret))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(reference.resourceVersion))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (r *AIWorkloadReconciler) effectiveWatchNamespaces() []string {
	if len(r.WatchNamespaces) != 0 {
		return r.WatchNamespaces
	}
	if r.WatchNamespace == "" {
		return nil
	}
	return []string{r.WatchNamespace}
}

func (r *AIWorkloadReconciler) watchesNamespace(namespace string) bool {
	for _, watched := range r.effectiveWatchNamespaces() {
		if namespace == watched {
			return true
		}
	}
	return false
}

const (
	tenantProfileConfigMap = "awcp-tenant-profile"
	conditionTenantReady   = "TenantReady"
)

type tenantConfigurationError struct {
	reason  string
	message string
}

func (e tenantConfigurationError) Error() string { return e.reason }

// validateTenant treats a namespace-local, GitOps-owned ConfigMap as the
// policy boundary. No AIWorkload field is ever allowed to redirect this lookup
// into another namespace.
func (r *AIWorkloadReconciler) validateTenant(ctx context.Context, workload *platformv1alpha1.AIWorkload) error {
	if workload.Spec.Tenant == "" {
		meta.RemoveStatusCondition(&workload.Status.Conditions, conditionTenantReady)
		return nil
	}
	reader := r.TenantReader
	if reader == nil {
		reader = r.Client
	}
	profile := &corev1.ConfigMap{}
	if err := reader.Get(ctx, client.ObjectKey{Namespace: workload.Namespace, Name: tenantProfileConfigMap}, profile); err != nil {
		if apierrors.IsNotFound(err) {
			return tenantConfigurationError{reason: "TenantNotConfigured", message: "This namespace has no GitOps-managed AWCP tenant profile."}
		}
		return fmt.Errorf("read tenant profile: %w", err)
	}
	if profile.Data["tenant"] != workload.Spec.Tenant {
		return tenantConfigurationError{reason: "TenantMismatch", message: "spec.tenant must match the tenant profile bound to this namespace."}
	}
	if profile.Data["profile"] != "small" && profile.Data["profile"] != "medium" && profile.Data["profile"] != "large" {
		return tenantConfigurationError{reason: "TenantProfileInvalid", message: "The namespace tenant profile must select small, medium, or large."}
	}
	meta.SetStatusCondition(&workload.Status.Conditions, metav1.Condition{
		Type: conditionTenantReady, Status: metav1.ConditionTrue, Reason: "TenantConfigured",
		Message: "The workload tenant matches the namespace-local GitOps profile.", ObservedGeneration: workload.Generation,
	})
	return nil
}

func childKind(object client.Object) string {
	switch childOrder(object) {
	case 0:
		return "ServiceAccount"
	case 1:
		return "Deployment"
	case 2:
		return "Service"
	case 3:
		return "NetworkPolicy"
	case 4:
		return "HorizontalPodAutoscaler"
	case 5:
		return "PodDisruptionBudget"
	case 6:
		return "HTTPRoute"
	default:
		return "Unknown"
	}
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
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		WatchesMetadata(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.RequestsForSecret)).
		Named(r.ControllerName).
		Complete(r)
}
