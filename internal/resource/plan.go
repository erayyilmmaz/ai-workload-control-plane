// SPDX-License-Identifier: Apache-2.0
package resource

import (
	"crypto/sha256"
	"fmt"
	"strings"

	platformv1alpha1 "github.com/erayyilmmaz/ai-workload-control-plane/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Builder computes a deterministic plan without API calls or external state.
// Production mappings are introduced incrementally in AWCP-6..9.
type Builder interface {
	Build(*platformv1alpha1.AIWorkload) ([]Intent, error)
}

// BuilderFunc adapts a pure function to Builder.
type BuilderFunc func(*platformv1alpha1.AIWorkload) ([]Intent, error)

func (f BuilderFunc) Build(p *platformv1alpha1.AIWorkload) ([]Intent, error) { return f(p) }

// Intent targets one child. Object is a fresh typed identity, not a cached object.
// Mutate sets ONLY owned fields, preserving API defaults, unrelated metadata and
// injected fields. It runs on both fresh and existing objects, and must be pure.
// Absent is allowed only for optional Service, NetworkPolicy, HPA, or PDB children.
type Intent struct {
	Object client.Object
	Mutate func(client.Object) error
	Absent bool
}

// ChildName implements the accepted bounded naming rule, including short names.
func ChildName(name string) string {
	prefix := name
	if len(prefix) > 40 {
		prefix = prefix[:40]
	}
	prefix = strings.TrimRight(strings.ReplaceAll(prefix, ".", "-"), "-")
	sum := sha256.Sum256([]byte(name))
	return fmt.Sprintf("awcp-%s-%x", prefix, sum[:8])
}
