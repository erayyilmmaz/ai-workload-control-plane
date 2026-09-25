// SPDX-License-Identifier: Apache-2.0
package controller

import (
	"context"
	"errors"
	"reflect"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	platformv1alpha1 "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
	"github.com/erayyilmmaz/ai-workload-control-plane/internal/resource"
)

var (
	ErrOwnershipConflict = errors.New("ResourceOwnershipConflict")
	ErrInvalidPlan       = resource.ErrInvalidConfiguration
	ErrChildDeleting     = errors.New("owned child is still deleting")
)

// Outcome makes create/patch/delete/no-op visible without forcing a requeue.
type Outcome string

const (
	Created   Outcome = "created"
	Patched   Outcome = "patched"
	Deleted   Outcome = "deleted"
	Unchanged Outcome = "unchanged"
)

type Engine struct {
	Client client.Client
	Scheme *runtime.Scheme
}

func childOrder(o client.Object) int {
	switch object := o.(type) {
	case *corev1.ServiceAccount:
		return 0
	case *appsv1.Deployment:
		return 1
	case *corev1.Service:
		return 2
	case *networkingv1.NetworkPolicy:
		return 3
	case *unstructured.Unstructured:
		if object.GroupVersionKind() == resource.HTTPRouteGVK {
			return 4
		}
		return -1
	default:
		return -1
	}
}

// ValidatePlan rejects unsafe targets before the first API write and orders dependencies.
func ValidatePlan(p *platformv1alpha1.AIWorkload, plan []resource.Intent) ([]resource.Intent, error) {
	if p.UID == "" || p.Namespace == "" || p.Name == "" {
		return nil, ErrInvalidPlan
	}
	seen := map[int]bool{}
	ordered := append([]resource.Intent(nil), plan...)
	for _, i := range ordered {
		if childOrder(i.Object) < 0 || reflect.ValueOf(i.Object).IsNil() {
			return nil, ErrInvalidPlan
		}
		kind := childOrder(i.Object)
		if kind < 0 || seen[kind] || i.Object.GetNamespace() != p.Namespace || i.Object.GetName() != resource.ChildName(p.Name) || (i.Absent && kind < 2) || (!i.Absent && i.Mutate == nil) {
			return nil, ErrInvalidPlan
		}
		if i.Object.GetUID() != "" || i.Object.GetResourceVersion() != "" || len(i.Object.GetOwnerReferences()) != 0 || len(i.Object.GetFinalizers()) != 0 || i.Object.GetDeletionTimestamp() != nil {
			return nil, ErrInvalidPlan
		}
		seen[kind] = true
	}
	sort.SliceStable(ordered, func(i, j int) bool { return childOrder(ordered[i].Object) < childOrder(ordered[j].Object) })
	return ordered, nil
}

func owns(p *platformv1alpha1.AIWorkload, o client.Object) bool {
	owner := metav1.GetControllerOf(o)
	return o.GetNamespace() == p.Namespace && owner != nil && owner.UID == p.UID && owner.Name == p.Name && owner.Kind == "AIWorkload" && owner.APIVersion == platformv1alpha1.GroupVersion.String()
}

// Apply uses optimistic patches and UID+resourceVersion delete preconditions.
// There is intentionally no adoption, force update, or destructive recreation.
func (e Engine) Apply(ctx context.Context, p *platformv1alpha1.AIWorkload, i resource.Intent) (Outcome, error) {
	if !p.DeletionTimestamp.IsZero() {
		return Unchanged, nil
	}
	if _, err := ValidatePlan(p, []resource.Intent{i}); err != nil {
		return "", err
	}
	current := i.Object.DeepCopyObject().(client.Object)
	err := e.Client.Get(ctx, client.ObjectKeyFromObject(current), current)
	if apierrors.IsNotFound(err) {
		if i.Absent {
			return Unchanged, nil
		}
		if err = i.Mutate(current); err != nil {
			return "", err
		}
		if current.GetName() != i.Object.GetName() || current.GetNamespace() != p.Namespace || current.GetUID() != "" || current.GetResourceVersion() != "" || len(current.GetOwnerReferences()) != 0 || len(current.GetFinalizers()) != 0 || current.GetDeletionTimestamp() != nil {
			return "", ErrInvalidPlan
		}
		if err = controllerutil.SetControllerReference(p, current, e.Scheme, controllerutil.WithBlockOwnerDeletion(false)); err != nil {
			return "", ErrInvalidPlan
		}
		if err = e.Client.Create(ctx, current); err != nil {
			return "", err
		}
		return Created, nil
	}
	if err != nil {
		return "", err
	}
	if !owns(p, current) {
		return "", ErrOwnershipConflict
	}
	if !current.GetDeletionTimestamp().IsZero() {
		return "", ErrChildDeleting
	}
	if i.Absent {
		uid, rv := current.GetUID(), current.GetResourceVersion()
		err = e.Client.Delete(ctx, current, &client.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid, ResourceVersion: &rv}})
		if apierrors.IsNotFound(err) {
			return Unchanged, nil
		}
		if err != nil {
			return "", err
		}
		return Deleted, nil
	}
	before := current.DeepCopyObject().(client.Object)
	if err = i.Mutate(current); err != nil {
		return "", err
	}
	if current.GetName() != before.GetName() || current.GetNamespace() != before.GetNamespace() || current.GetUID() != before.GetUID() || current.GetResourceVersion() != before.GetResourceVersion() || !reflect.DeepEqual(current.GetOwnerReferences(), before.GetOwnerReferences()) || !reflect.DeepEqual(current.GetFinalizers(), before.GetFinalizers()) || !reflect.DeepEqual(current.GetDeletionTimestamp(), before.GetDeletionTimestamp()) {
		return "", ErrInvalidPlan
	}
	if err = controllerutil.SetControllerReference(p, current, e.Scheme, controllerutil.WithBlockOwnerDeletion(false)); err != nil {
		return "", ErrInvalidPlan
	}
	if apiequality.Semantic.DeepEqual(before, current) {
		return Unchanged, nil
	}
	if err = e.Client.Patch(ctx, current, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})); err != nil {
		return "", err
	}
	return Patched, nil
}
