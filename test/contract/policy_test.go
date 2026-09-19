// SPDX-License-Identifier: Apache-2.0
package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/yaml"
)

func policyObjects(t *testing.T) []*unstructured.Unstructured {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "config", "policy", "restricted-aiworkload-policy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(data), "\n---\n")
	objects := make([]*unstructured.Unstructured, 0, len(parts))
	for _, part := range parts {
		jsonData, err := yaml.YAMLToJSON([]byte(part))
		if err != nil {
			t.Fatal(err)
		}
		object := &unstructured.Unstructured{}
		if err = json.Unmarshal(jsonData, &object.Object); err != nil {
			t.Fatal(err)
		}
		objects = append(objects, object)
	}
	return objects
}

func TestRestrictedAdmissionPolicy(t *testing.T) {
	if testing.Short() {
		t.Skip("real admission policy tests require envtest; run make test")
	}
	assets := os.Getenv("KUBEBUILDER_ASSETS")
	if assets == "" {
		t.Fatal("KUBEBUILDER_ASSETS is required; run make test")
	}
	environment := &envtest.Environment{CRDDirectoryPaths: []string{"../../config/crd/bases"}, ErrorIfCRDPathMissing: true}
	cfg, err := environment.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := environment.Stop(); err != nil {
			t.Errorf("envtest Stop: %v", err)
		}
	})
	api, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		t.Fatal(err)
	}
	if err = api.Create(t.Context(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "policy-restricted", Labels: map[string]string{"platform.example.io/policy-profile": "restricted"}}}); err != nil {
		t.Fatal(err)
	}
	if err = api.Create(t.Context(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "policy-shared"}}); err != nil {
		t.Fatal(err)
	}
	for _, object := range policyObjects(t) {
		if err = api.Create(t.Context(), object); err != nil {
			t.Fatalf("create %s/%s: %v", object.GetKind(), object.GetName(), err)
		}
	}

	bad := fixture(t, "config/samples/aiworkload_full.yaml")
	bad.SetName("restricted-missing-policy")
	bad.SetNamespace("policy-restricted")
	set(t, bad, "alpha", "spec", "tenant")
	denied := false
	for attempt := 0; attempt < 30; attempt++ {
		err = api.Create(t.Context(), bad)
		if err != nil {
			if !apierrors.IsInvalid(err) || !strings.Contains(err.Error(), "restricted tenant namespaces require spec.policy: restricted") {
				t.Fatalf("unexpected restricted policy response: %v", err)
			}
			denied = true
			break
		}
		if err = api.Delete(t.Context(), bad); err != nil {
			t.Fatal(err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !denied {
		t.Fatal("restricted policy never rejected an AIWorkload without spec.policy")
	}

	good := fixture(t, "config/samples/aiworkload_full.yaml")
	good.SetName("restricted-accepted")
	good.SetNamespace("policy-restricted")
	set(t, good, "alpha", "spec", "tenant")
	set(t, good, "restricted", "spec", "policy")
	if err = api.Create(t.Context(), good); err != nil {
		t.Fatalf("restricted profile rejected a compliant workload: %v", err)
	}

	v0 := fixture(t, "config/samples/platform_v1alpha1_aiworkload.yaml")
	v0.SetName("shared-v0")
	v0.SetNamespace("policy-shared")
	if err = api.Create(t.Context(), v0); err != nil {
		t.Fatalf("unselected namespace changed V0 admission behavior: %v", err)
	}
}
