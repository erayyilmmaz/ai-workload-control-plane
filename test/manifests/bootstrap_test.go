// SPDX-License-Identifier: Apache-2.0
package manifests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

func readYAML(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", path))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.UnmarshalStrict(data, value); err != nil {
		t.Fatal(err)
	}
}

func TestManagerSecurity(t *testing.T) {
	var deployment appsv1.Deployment
	readYAML(t, "config/manager/manager.yaml", &deployment)
	if deployment.Namespace != "awcp-system" || deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 1 {
		t.Fatal("bootstrap supports one manager in awcp-system")
	}
	pod := deployment.Spec.Template.Spec
	security := pod.SecurityContext
	if security == nil || security.RunAsNonRoot == nil || !*security.RunAsNonRoot || security.RunAsUser == nil || *security.RunAsUser != 65532 {
		t.Fatal("manager must run nonroot as UID 65532")
	}
	if security.SeccompProfile == nil || security.SeccompProfile.Type != "RuntimeDefault" {
		t.Fatal("seccomp is required")
	}
	if pod.AutomountServiceAccountToken == nil || !*pod.AutomountServiceAccountToken {
		t.Fatal("manager requires its Kubernetes API identity")
	}
	if len(pod.Containers) != 1 {
		t.Fatal("expected exactly one manager container")
	}
	container := pod.Containers[0]
	sc := container.SecurityContext
	if sc == nil || sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation || sc.ReadOnlyRootFilesystem == nil || !*sc.ReadOnlyRootFilesystem {
		t.Fatal("restricted container security is required")
	}
	if sc.Capabilities == nil || len(sc.Capabilities.Drop) != 1 || sc.Capabilities.Drop[0] != "ALL" {
		t.Fatal("all capabilities must be dropped")
	}
	if container.LivenessProbe == nil || container.LivenessProbe.HTTPGet.Path != "/healthz" || container.ReadinessProbe == nil || container.ReadinessProbe.HTTPGet.Path != "/readyz" {
		t.Fatal("both probes must be configured")
	}
	if !strings.Contains(strings.Join(container.Args, " "), "--leader-elect=true") || !strings.Contains(strings.Join(container.Args, " "), "--metrics-bind-address=:8443") {
		t.Fatal("leader election must be enabled")
	}
	if len(container.Ports) != 2 || container.Ports[1].Name != "metrics" || container.Ports[1].ContainerPort != 8443 || len(container.VolumeMounts) != 1 || container.VolumeMounts[0].MountPath != "/tmp/k8s-metrics-server/serving-certs" || len(pod.Volumes) != 1 || pod.Volumes[0].EmptyDir == nil {
		t.Fatal("secure metrics serving volume or port missing")
	}
	if len(container.Env) != 2 || container.Env[0].Name != "WATCH_NAMESPACE" || container.Env[0].Value != "awcp-workloads" || container.Env[1].Name != "MANAGER_NAMESPACE" || container.Env[1].ValueFrom.FieldRef.FieldPath != "metadata.namespace" {
		t.Fatal("namespace configuration drift")
	}
}

func TestMetricsAuthenticationRBACAndService(t *testing.T) {
	var role rbacv1.ClusterRole
	readYAML(t, "config/rbac/metrics_auth_clusterrole.yaml", &role)
	if role.Name != "awcp-metrics-auth" || len(role.Rules) != 2 {
		t.Fatalf("unexpected metrics auth role: %+v", role)
	}
	want := map[string]string{"authentication.k8s.io/tokenreviews": "create", "authorization.k8s.io/subjectaccessreviews": "create"}
	for _, rule := range role.Rules {
		key := strings.Join(rule.APIGroups, ",") + "/" + strings.Join(rule.Resources, ",")
		if want[key] != strings.Join(rule.Verbs, ",") {
			t.Fatalf("unexpected metrics auth permission: %+v", rule)
		}
		delete(want, key)
	}
	if len(want) != 0 {
		t.Fatalf("missing metrics auth permission: %v", want)
	}
	var service corev1.Service
	readYAML(t, "config/telemetry/metrics-service.yaml", &service)
	if service.Namespace != "awcp-system" || len(service.Spec.Ports) != 1 || service.Spec.Ports[0].Port != 8443 || service.Spec.Ports[0].TargetPort.String() != "metrics" {
		t.Fatalf("unexpected metrics service: %+v", service)
	}
}

func TestGeneratedRBACAndCRD(t *testing.T) {
	var role rbacv1.Role
	readYAML(t, "config/rbac/role.yaml", &role)
	if role.Kind != "Role" || role.Namespace != "awcp-workloads" || len(role.Rules) != 8 {
		t.Fatalf("unexpected runtime role: %+v", role)
	}
	want := map[string]string{
		"/secrets":                               "get,list,watch",
		"/serviceaccounts":                       "create,get,list,patch,update,watch",
		"/services":                              "create,delete,get,list,patch,update,watch",
		"apps/deployments":                       "create,get,list,patch,update,watch",
		"networking.k8s.io/networkpolicies":      "create,delete,get,list,patch,update,watch",
		"events.k8s.io/events":                   "create,patch,update",
		"platform.example.io/aiworkloads":        "get,list,watch",
		"platform.example.io/aiworkloads/status": "patch,update",
	}
	for _, rule := range role.Rules {
		key := strings.Join(rule.APIGroups, ",") + "/" + strings.Join(rule.Resources, ",")
		verbs, ok := want[key]
		if !ok || strings.Join(rule.Verbs, ",") != verbs || len(rule.ResourceNames) != 0 || len(rule.NonResourceURLs) != 0 {
			t.Fatalf("unexpected permissions: %+v", rule)
		}
		delete(want, key)
	}
	if len(want) != 0 {
		t.Fatalf("missing permissions: %v", want)
	}
	var crd apiextensionsv1.CustomResourceDefinition
	readYAML(t, "config/crd/bases/platform.example.io_aiworkloads.yaml", &crd)
	if crd.Spec.Scope != apiextensionsv1.NamespaceScoped || crd.Spec.Group != "platform.example.io" || crd.Spec.Names.Kind != "AIWorkload" {
		t.Fatal("CRD identity/scope drift")
	}
	if len(crd.Spec.Versions) != 1 || crd.Spec.Versions[0].Name != "v1alpha1" || !crd.Spec.Versions[0].Storage || !crd.Spec.Versions[0].Served || crd.Spec.Versions[0].Subresources.Status == nil {
		t.Fatal("CRD version/status subresource drift")
	}
}

func TestDockerfileMatchesLock(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "toolchain.lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	var lock struct{ ContainerImages map[string]string }
	if err = json.Unmarshal(data, &lock); err != nil {
		t.Fatal(err)
	}
	dockerfile, err := os.ReadFile(filepath.Join("..", "..", "Dockerfile"))
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.ContainerImages) != 2 {
		t.Fatal("both base images must be locked")
	}
	for _, image := range lock.ContainerImages {
		if !strings.Contains(image, "@sha256:") || !strings.Contains(string(dockerfile), "FROM "+image) {
			t.Fatalf("base image lock drift: %s", image)
		}
	}
}
