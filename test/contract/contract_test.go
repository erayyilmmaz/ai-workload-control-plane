// SPDX-License-Identifier: Apache-2.0
// Package contract tests admission without starting any AWCP controller.
package contract

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/yaml"
)

func fixture(t *testing.T, path string) *unstructured.Unstructured {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", path))
	if err != nil {
		t.Fatal(err)
	}
	data, err = yaml.YAMLToJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	var object unstructured.Unstructured
	if err = json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	object.SetNamespace("contract-tests")
	return &object
}

func set(t *testing.T, object *unstructured.Unstructured, value any, path ...string) {
	t.Helper()
	if err := unstructured.SetNestedField(object.Object, value, path...); err != nil {
		t.Fatal(err)
	}
}

func expect(t *testing.T, object *unstructured.Unstructured, want any, path ...string) {
	t.Helper()
	got, found, err := unstructured.NestedFieldNoCopy(object.Object, path...)
	if err != nil || !found || !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %#v (found %v, err %v), want %#v", strings.Join(path, "."), got, found, err, want)
	}
}

func TestAPIContract(t *testing.T) {
	if testing.Short() {
		t.Skip("real API contract tests require envtest; run make test")
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
	if err = api.Create(t.Context(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "contract-tests"}}); err != nil {
		t.Fatal(err)
	}
	minimal := func(name string) *unstructured.Unstructured {
		o := fixture(t, "config/samples/platform_v1alpha1_aiworkload.yaml")
		o.SetName(name)
		return o
	}
	create := func(t *testing.T, o *unstructured.Unstructured) {
		t.Helper()
		if err := api.Create(t.Context(), o, &client.CreateOptions{FieldValidation: "Strict"}); err != nil {
			t.Fatal(err)
		}
	}
	get := func(t *testing.T, o *unstructured.Unstructured) {
		t.Helper()
		if err := api.Get(t.Context(), client.ObjectKeyFromObject(o), o); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("minimal-defaults", func(t *testing.T) {
		o := minimal("defaults")
		create(t, o)
		get(t, o)
		expect(t, o, int64(1), "spec", "replicas")
		expect(t, o, true, "spec", "service", "enabled")
		expect(t, o, int64(80), "spec", "service", "port")
		expect(t, o, true, "spec", "network", "enabled")
		expect(t, o, []any{}, "spec", "secretRefs")
		for _, key := range []string{"health", "resources"} {
			if _, ok, _ := unstructured.NestedFieldNoCopy(o.Object, "spec", key); ok {
				t.Fatalf("unexpected default %s", key)
			}
		}
		if _, ok := o.Object["status"]; ok {
			t.Fatal("admission must not manufacture status")
		}
	})
	t.Run("explicit-zero-false-and-empty-blocks", func(t *testing.T) {
		for i, enabled := range []bool{false, true} {
			o := minimal([]string{"explicit", "empty-blocks"}[i])
			set(t, o, int64(0), "spec", "replicas")
			set(t, o, map[string]any{}, "spec", "service")
			set(t, o, map[string]any{}, "spec", "network")
			if !enabled {
				set(t, o, false, "spec", "service", "enabled")
				set(t, o, false, "spec", "network", "enabled")
			}
			create(t, o)
			get(t, o)
			expect(t, o, int64(0), "spec", "replicas")
			expect(t, o, enabled, "spec", "service", "enabled")
			expect(t, o, enabled, "spec", "network", "enabled")
			expect(t, o, int64(80), "spec", "service", "port")
		}
	})
	t.Run("valid-boundaries-and-update-validation", func(t *testing.T) {
		for i, port := range []int64{1, 65535} {
			o := minimal([]string{"lower-bound", "upper-bound"}[i])
			set(t, o, int64(20), "spec", "replicas")
			set(t, o, port, "spec", "container", "port")
			set(t, o, map[string]any{"port": port}, "spec", "service")
			set(t, o, []any{"a", "secret.with-dots", strings.Repeat("a", 253)}, "spec", "secretRefs")
			create(t, o)
			get(t, o)
			expect(t, o, port, "spec", "service", "port")
			expect(t, o, true, "spec", "service", "enabled")
			set(t, o, int64(21), "spec", "replicas")
			if err := api.Update(t.Context(), o); !apierrors.IsInvalid(err) {
				t.Fatalf("invalid update accepted: %v", err)
			}
			get(t, o)
			expect(t, o, int64(20), "spec", "replicas")
		}
	})
	t.Run("full-sample-and-quantity-semantics", func(t *testing.T) {
		o := fixture(t, "config/samples/aiworkload_full.yaml")
		create(t, o)
		get(t, o)
		expect(t, o, []any{"agent-api-secrets", "agent-overrides"}, "spec", "secretRefs")
		expect(t, o, "/ready", "spec", "health", "readiness", "path")
		expect(t, o, "/health", "spec", "health", "liveness", "path")
		// These would fail if compared lexicographically rather than as quantities.
		for i, pair := range [][2]string{{"900m", "1"}, {"1024Mi", "1Gi"}, {"0", "0"}, {"1e3", "1k"}} {
			x := minimal([]string{"cpu-units", "memory-units", "zero-quantity", "exponent-quantity"}[i])
			resource := "memory"
			if i == 0 {
				resource = "cpu"
			}
			set(t, x, map[string]any{"requests": map[string]any{resource: pair[0]}, "limits": map[string]any{resource: pair[1]}}, "spec", "resources")
			create(t, x)
		}
	})
	t.Run("v0-compatible-manifest-remains-valid", func(t *testing.T) {
		o := fixture(t, "test/fixtures/v1/v0-compatible.yaml")
		create(t, o)
		get(t, o)
		expect(t, o, "example.invalid/v0-compatible:v1", "spec", "image")
		expect(t, o, int64(2), "spec", "replicas")
		expect(t, o, int64(8080), "spec", "service", "port")
		for _, futureField := range []string{"environment", "tenant", "autoscaling", "exposure", "delivery", "externalSecrets", "availability", "policy"} {
			if _, found, err := unstructured.NestedFieldNoCopy(o.Object, "spec", futureField); err != nil || found {
				t.Fatalf("V0 fixture unexpectedly has V1 field %q: found=%v err=%v", futureField, found, err)
			}
		}
	})
	t.Run("environment-is-optional-logical-identity", func(t *testing.T) {
		o := minimal("environment")
		set(t, o, "staging", "spec", "environment")
		create(t, o)
		get(t, o)
		expect(t, o, "staging", "spec", "environment")

		invalid := minimal("invalid-environment")
		set(t, invalid, "Staging", "spec", "environment")
		if err := api.Create(t.Context(), invalid, &client.CreateOptions{FieldValidation: "Strict"}); !apierrors.IsInvalid(err) || !strings.Contains(err.Error(), "spec.environment") {
			t.Fatalf("invalid environment accepted: %v", err)
		}
	})
	t.Run("tenant-is-optional-namespace-bound-identity", func(t *testing.T) {
		o := minimal("tenant")
		set(t, o, "alpha", "spec", "tenant")
		create(t, o)
		get(t, o)
		expect(t, o, "alpha", "spec", "tenant")

		invalid := minimal("invalid-tenant")
		set(t, invalid, "Tenant_Alpha", "spec", "tenant")
		if err := api.Create(t.Context(), invalid, &client.CreateOptions{FieldValidation: "Strict"}); !apierrors.IsInvalid(err) || !strings.Contains(err.Error(), "spec.tenant") {
			t.Fatalf("invalid tenant accepted: %v", err)
		}
	})
	t.Run("policy-is-an-optional-platform-profile", func(t *testing.T) {
		for _, profile := range []string{"baseline", "restricted"} {
			o := minimal("policy-" + profile)
			set(t, o, profile, "spec", "policy")
			create(t, o)
			get(t, o)
			expect(t, o, profile, "spec", "policy")
		}
		invalid := minimal("invalid-policy")
		set(t, invalid, "user-supplied-cel", "spec", "policy")
		if err := api.Create(t.Context(), invalid, &client.CreateOptions{FieldValidation: "Strict"}); !apierrors.IsInvalid(err) || !strings.Contains(err.Error(), "spec.policy") {
			t.Fatalf("invalid policy profile accepted: %v", err)
		}
	})
	t.Run("invalid-fixtures", func(t *testing.T) {
		data, err := os.ReadFile("../fixtures/invalid/index.json")
		if err != nil {
			t.Fatal(err)
		}
		var cases []struct{ File, Field string }
		if err = json.Unmarshal(data, &cases); err != nil {
			t.Fatal(err)
		}
		for _, tc := range cases {
			t.Run(tc.File, func(t *testing.T) {
				o := fixture(t, "test/fixtures/invalid/"+tc.File)
				err := api.Create(t.Context(), o, &client.CreateOptions{FieldValidation: "Strict"})
				if !apierrors.IsInvalid(err) || !strings.Contains(err.Error(), tc.Field) {
					t.Fatalf("expected Invalid at %s, got %v", tc.Field, err)
				}
				if err = api.Get(t.Context(), client.ObjectKeyFromObject(o), o); !apierrors.IsNotFound(err) {
					t.Fatalf("invalid object was stored: %v", err)
				}
			})
		}
	})
	t.Run("unknown-fields-strict-versus-pruning", func(t *testing.T) {
		o := fixture(t, "test/fixtures/unknown-field.json")
		err := api.Create(t.Context(), o, &client.CreateOptions{FieldValidation: "Strict"})
		if !apierrors.IsBadRequest(err) || !strings.Contains(err.Error(), "spec.contaner") {
			t.Fatalf("strict must reject typo: %v", err)
		}
		if err = api.Create(t.Context(), o, &client.CreateOptions{FieldValidation: "Ignore"}); err != nil {
			t.Fatal(err)
		}
		get(t, o)
		if _, ok, _ := unstructured.NestedFieldNoCopy(o.Object, "spec", "contaner"); ok {
			t.Fatal("unknown field was not pruned")
		}
	})
	t.Run("status-and-spec-responsibilities", func(t *testing.T) {
		o := minimal("status-contract")
		data, err := os.ReadFile("../fixtures/status.yaml")
		if err != nil {
			t.Fatal(err)
		}
		jsonData, err := yaml.YAMLToJSON(data)
		if err != nil {
			t.Fatal(err)
		}
		var holder unstructured.Unstructured
		if err = json.Unmarshal(append(append([]byte(`{"apiVersion":"platform.example.io/v1alpha1","kind":"AIWorkload","status":`), jsonData...), '}'), &holder); err != nil {
			t.Fatal(err)
		}
		status := holder.Object["status"]
		set(t, o, status, "status")
		create(t, o)
		get(t, o)
		if _, ok := o.Object["status"]; ok {
			t.Fatal("create wrote user-supplied status")
		}
		generation := o.GetGeneration()
		originalImage, _, _ := unstructured.NestedString(o.Object, "spec", "image")
		set(t, o, status, "status")
		set(t, o, "ignored.invalid/image:v9", "spec", "image")
		if err = api.Status().Update(t.Context(), o); err != nil {
			t.Fatal(err)
		}
		get(t, o)
		expect(t, o, originalImage, "spec", "image")
		expect(t, o, status, "status")
		if o.GetGeneration() != generation {
			t.Fatal("status write advanced generation")
		}
		set(t, o, map[string]any{}, "status")
		set(t, o, "example.invalid/app:v2", "spec", "image")
		if err = api.Update(t.Context(), o); err != nil {
			t.Fatal(err)
		}
		get(t, o)
		expect(t, o, status, "status")
		if o.GetGeneration() != generation+1 {
			t.Fatal("spec update did not increment generation")
		}
		// Invalid status is rejected on /status; ordinary spec updates cannot clear it.
		set(t, o, int64(-1), "status", "readyReplicas")
		if err = api.Status().Update(t.Context(), o); !apierrors.IsInvalid(err) {
			t.Fatalf("negative status accepted: %v", err)
		}
		get(t, o)
		conditions, _, _ := unstructured.NestedSlice(o.Object, "status", "conditions")
		delete(conditions[0].(map[string]any), "observedGeneration")
		set(t, o, conditions, "status", "conditions")
		if err = api.Status().Update(t.Context(), o); !apierrors.IsInvalid(err) {
			t.Fatalf("missing condition generation accepted: %v", err)
		}
		get(t, o)
		conditions, _, _ = unstructured.NestedSlice(o.Object, "status", "conditions")
		conditions = append(conditions, conditions[0])
		set(t, o, conditions, "status", "conditions")
		if err = api.Status().Update(t.Context(), o); !apierrors.IsInvalid(err) {
			t.Fatalf("duplicate condition type accepted: %v", err)
		}
	})
	t.Run("kubectl-explain-and-server-printer-columns", func(t *testing.T) {
		o := minimal("printer-table")
		create(t, o)
		set(t, o, []any{map[string]any{"type": "Ready", "status": "True", "reason": "TestFixture", "message": "Synthetic table test", "observedGeneration": o.GetGeneration(), "lastTransitionTime": "2026-09-12T09:00:00Z"}}, "status", "conditions")
		if err := api.Status().Update(t.Context(), o); err != nil {
			t.Fatal(err)
		}
		config := clientcmdapi.Config{Clusters: map[string]*clientcmdapi.Cluster{"local": {Server: cfg.Host, CertificateAuthorityData: cfg.CAData}}, AuthInfos: map[string]*clientcmdapi.AuthInfo{"local": {ClientCertificateData: cfg.CertData, ClientKeyData: cfg.KeyData}}, Contexts: map[string]*clientcmdapi.Context{"local": {Cluster: "local", AuthInfo: "local", Namespace: "contract-tests"}}, CurrentContext: "local"}
		path := filepath.Join(t.TempDir(), "kubeconfig")
		if err := clientcmd.WriteToFile(config, path); err != nil {
			t.Fatal(err)
		}
		run := func(args ...string) string {
			t.Helper()
			cmd := exec.CommandContext(t.Context(), filepath.Join(assets, "kubectl"), append([]string{"--kubeconfig", path}, args...)...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("kubectl %v: %v\n%s", args, err, out)
			}
			return string(out)
		}
		out := run("explain", "aiworkload.spec", "--api-version=platform.example.io/v1alpha1")
		for _, field := range []string{"image", "replicas", "container", "resources", "health", "service", "secretRefs", "network"} {
			if !strings.Contains(out, field) {
				t.Fatalf("missing explain field %s: %s", field, out)
			}
		}
		out = run("explain", "aiworkload.spec.image", "--api-version=platform.example.io/v1alpha1")
		if !strings.Contains(out, "container image reference") {
			t.Fatalf("missing field documentation: %s", out)
		}
		out = run("get", "aiworkloads", "printer-table")
		for _, word := range []string{"NAME", "READY", "REPLICAS", "IMAGE", "AGE", "True", "ghcr.io/example/demo-agent:v1"} {
			if !strings.Contains(out, word) {
				t.Fatalf("missing table value %s: %s", word, out)
			}
		}
	})
}
