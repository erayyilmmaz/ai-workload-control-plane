// SPDX-License-Identifier: Apache-2.0
package resource

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platform "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
)

// serviceAccountIntent creates a workload identity without granting it RBAC.
func serviceAccountIntent(p *platform.AIWorkload) Intent {
	return Intent{Object: &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: ChildName(p.Name), Namespace: p.Namespace}}, Mutate: func(o client.Object) error {
		account, ok := o.(*corev1.ServiceAccount)
		if !ok {
			return ErrInvalidConfiguration
		}
		managedMetadata(&account.ObjectMeta, p)
		account.AutomountServiceAccountToken = ptr.To(false)
		return nil
	}}
}
