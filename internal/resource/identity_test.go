// SPDX-License-Identifier: Apache-2.0
package resource

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apimachinery "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/ptr"

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
)

func identityFor(t *testing.T, p *platform.AIWorkload) Intent {
	t.Helper()
	plan, err := (WorkloadBuilder{}).Build(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, intent := range plan {
		if _, ok := intent.Object.(*corev1.ServiceAccount); ok {
			return intent
		}
	}
	t.Fatal("ServiceAccount intent missing")
	return Intent{}
}

func TestDedicatedServiceAccountMapping(t *testing.T) {
	p := source()
	before := p.DeepCopy()
	intent := identityFor(t, p)
	account := intent.Object.(*corev1.ServiceAccount)
	if err := intent.Mutate(account); err != nil {
		t.Fatal(err)
	}
	if !apimachinery.Semantic.DeepEqual(p, before) || intent.Absent || account.Name != ChildName(p.Name) || account.Namespace != p.Namespace || len(account.OwnerReferences) != 0 {
		t.Fatal("identity intent changed primary or violates engine ownership boundary")
	}
	if ptr.Deref(account.AutomountServiceAccountToken, true) || account.Annotations[WorkloadNameAnnotation] != p.Name {
		t.Fatal("dedicated identity token policy or metadata is wrong")
	}
	for k, v := range SelectorLabels(p) {
		if account.Labels[k] != v {
			t.Fatalf("identity label %q missing", k)
		}
	}
}

func TestDedicatedServiceAccountPreservesUnmanagedFields(t *testing.T) {
	p := source()
	account := &corev1.ServiceAccount{}
	intent := identityFor(t, p)
	if err := intent.Mutate(account); err != nil {
		t.Fatal(err)
	}
	account.ResourceVersion = "1"
	account.Labels["user.example/keep"] = "yes"
	account.Annotations["user.example/keep"] = "yes"
	account.ImagePullSecrets = []corev1.LocalObjectReference{{Name: "injected"}}
	account.Secrets = []corev1.ObjectReference{{Name: "legacy-token"}}
	before := account.DeepCopy()
	if err := identityFor(t, p).Mutate(account); err != nil {
		t.Fatal(err)
	}
	if !apimachinery.Semantic.DeepEqual(before, account) || !reflect.DeepEqual(account.ImagePullSecrets, []corev1.LocalObjectReference{{Name: "injected"}}) || !reflect.DeepEqual(account.Secrets, []corev1.ObjectReference{{Name: "legacy-token"}}) {
		t.Fatal("identity reconciliation overwrote unmanaged ServiceAccount fields")
	}
}
