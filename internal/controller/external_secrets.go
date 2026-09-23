// SPDX-License-Identifier: Apache-2.0
package controller

import (
	"context"
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platformv1alpha1 "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/capability"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/resource"
)

const (
	conditionExternalSecretsReady = "ExternalSecretsReady"
	externalSecretRevisionKey     = "platform.example.io/external-secret-revision"
)

var (
	externalSecretGVK = schema.GroupVersionKind{Group: "external-secrets.io", Version: "v1", Kind: "ExternalSecret"}
	secretStoreGVK    = schema.GroupVersionKind{Group: "external-secrets.io", Version: "v1", Kind: "SecretStore"}
)

// externalSecretConfigurationError is a permanent, payload-free dependency
// error. Its reason/message are deliberately curated before status or Event use.
type externalSecretConfigurationError struct {
	reason  string
	message string
}

func (e externalSecretConfigurationError) Error() string { return e.reason }

type resolvedExternalSecret struct {
	targetSecret    platformv1alpha1.SecretReference
	resourceVersion string
}

// validateExternalSecrets validates only ESO metadata/status and the target
// Secret metadata. It never reads Secret data, writes ESO objects, or contacts a
// provider. Each referenced ESO resource is required to remain in the workload
// namespace and to use a namespaced SecretStore.
func (r *AIWorkloadReconciler) validateExternalSecrets(ctx context.Context, workload *platformv1alpha1.AIWorkload) ([]resolvedExternalSecret, error) {
	if len(workload.Spec.ExternalSecrets) == 0 {
		meta.RemoveStatusCondition(&workload.Status.Conditions, conditionExternalSecretsReady)
		return nil, nil
	}
	if r.CapabilityLookup == nil {
		return nil, externalSecretConfigurationError{reason: "ExternalSecretsUnavailable", message: "External Secrets Operator v1 APIs are unavailable; install ESO or remove spec.externalSecrets."}
	}
	requirement, ok := capability.RequirementFor(capability.ExternalSecrets)
	if !ok {
		return nil, fmt.Errorf("external secrets capability contract is missing")
	}
	observation := capability.Detect(ctx, r.CapabilityLookup, requirement)
	if observation.State == capability.Unavailable {
		return nil, externalSecretConfigurationError{reason: "ExternalSecretsUnavailable", message: "External Secrets Operator v1 APIs are unavailable; install ESO or remove spec.externalSecrets."}
	}
	if observation.State == capability.Unknown {
		return nil, fmt.Errorf("discover External Secrets Operator API: %w", observation.Err)
	}
	reader := r.ExternalSecretsReader
	if reader == nil {
		reader = r.Client
	}
	direct := map[string]bool{}
	for _, name := range workload.Spec.SecretRefs {
		direct[string(name)] = true
	}
	resolved := make([]resolvedExternalSecret, 0, len(workload.Spec.ExternalSecrets))
	for _, reference := range workload.Spec.ExternalSecrets {
		if direct[string(reference.TargetSecret)] {
			return nil, externalSecretConfigurationError{reason: "ExternalSecretTargetDuplicate", message: "An external Secret target must not duplicate a direct spec.secretRefs entry."}
		}
		externalSecret := &unstructured.Unstructured{}
		externalSecret.SetGroupVersionKind(externalSecretGVK)
		if err := reader.Get(ctx, client.ObjectKey{Namespace: workload.Namespace, Name: string(reference.ExternalSecret)}, externalSecret); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, externalSecretConfigurationError{reason: "ExternalSecretNotFound", message: "A referenced ExternalSecret is unavailable in the workload namespace."}
			}
			return nil, fmt.Errorf("get ExternalSecret metadata: %w", err)
		}
		storeName, found, err := unstructured.NestedString(externalSecret.Object, "spec", "secretStoreRef", "name")
		if err != nil || !found || storeName == "" {
			return nil, externalSecretConfigurationError{reason: "ExternalSecretInvalid", message: "The referenced ExternalSecret must select a namespace-local SecretStore."}
		}
		storeKind, found, err := unstructured.NestedString(externalSecret.Object, "spec", "secretStoreRef", "kind")
		if err != nil || !found || storeKind != "SecretStore" {
			return nil, externalSecretConfigurationError{reason: "ExternalSecretStoreScopeInvalid", message: "The referenced ExternalSecret must use kind: SecretStore; ClusterSecretStore is outside this workload boundary."}
		}
		store := &unstructured.Unstructured{}
		store.SetGroupVersionKind(secretStoreGVK)
		if err := reader.Get(ctx, client.ObjectKey{Namespace: workload.Namespace, Name: storeName}, store); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, externalSecretConfigurationError{reason: "SecretStoreNotFound", message: "The referenced namespace-local SecretStore is unavailable."}
			}
			return nil, fmt.Errorf("get SecretStore metadata: %w", err)
		}
		if !readyCondition(store) {
			return nil, externalSecretConfigurationError{reason: "SecretStoreNotReady", message: "The referenced SecretStore is not Ready; inspect External Secrets Operator status."}
		}
		targetName, found, err := unstructured.NestedString(externalSecret.Object, "spec", "target", "name")
		if err != nil {
			return nil, externalSecretConfigurationError{reason: "ExternalSecretInvalid", message: "The referenced ExternalSecret target configuration is invalid."}
		}
		if !found || targetName == "" {
			targetName = externalSecret.GetName()
		}
		if targetName != string(reference.TargetSecret) {
			return nil, externalSecretConfigurationError{reason: "ExternalSecretTargetMismatch", message: "spec.externalSecrets targetSecret must match the ExternalSecret target in the same namespace."}
		}
		if !readyCondition(externalSecret) {
			return nil, externalSecretConfigurationError{reason: "ExternalSecretNotReady", message: "The referenced ExternalSecret is not Ready; inspect External Secrets Operator status."}
		}
		target := &metav1.PartialObjectMetadata{}
		target.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "Secret"})
		if err := r.Get(ctx, client.ObjectKey{Namespace: workload.Namespace, Name: targetName}, target); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, externalSecretConfigurationError{reason: "ExternalSecretTargetNotFound", message: "The ready ExternalSecret target Secret is unavailable in the workload namespace."}
			}
			return nil, fmt.Errorf("get ExternalSecret target metadata: %w", err)
		}
		resolved = append(resolved, resolvedExternalSecret{targetSecret: reference.TargetSecret, resourceVersion: target.GetResourceVersion()})
	}
	meta.SetStatusCondition(&workload.Status.Conditions, metav1.Condition{Type: conditionExternalSecretsReady, Status: metav1.ConditionTrue, Reason: "ExternalSecretsReady", Message: "Referenced namespace-local ExternalSecrets and target Secrets are Ready.", ObservedGeneration: workload.Generation})
	return resolved, nil
}

func readyCondition(object *unstructured.Unstructured) bool {
	conditions, found, err := unstructured.NestedSlice(object.Object, "status", "conditions")
	if err != nil || !found {
		return false
	}
	for _, raw := range conditions {
		condition, ok := raw.(map[string]any)
		if ok && condition["type"] == "Ready" && condition["status"] == "True" {
			return true
		}
	}
	return false
}

func decorateExternalSecretRevision(plan []resource.Intent, revision string) []resource.Intent {
	if revision == "" {
		return plan
	}
	for index := range plan {
		if _, ok := plan[index].Object.(*appsv1.Deployment); !ok {
			continue
		}
		mutate := plan[index].Mutate
		plan[index].Mutate = func(object client.Object) error {
			if err := mutate(object); err != nil {
				return err
			}
			deployment, ok := object.(*appsv1.Deployment)
			if !ok {
				return errors.New("external Secret revision was applied to a non-Deployment")
			}
			if deployment.Spec.Template.Annotations == nil {
				deployment.Spec.Template.Annotations = map[string]string{}
			}
			deployment.Spec.Template.Annotations[externalSecretRevisionKey] = revision
			return nil
		}
		break
	}
	return plan
}
