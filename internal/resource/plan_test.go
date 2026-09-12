// SPDX-License-Identifier: Apache-2.0
package resource

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/validation"
)

func TestChildNameIsBoundedStableAndUsesFullName(t *testing.T) {
	for _, name := range []string{"a", "a.b", strings.Repeat("a", 40) + ".tail", strings.Repeat("a", 39) + "-tail", strings.Repeat("a", 253)} {
		got := ChildName(name)
		if got != ChildName(name) || len(got) > 62 || len(validation.IsDNS1035Label(got)) != 0 {
			t.Fatalf("invalid or unstable child name: %s", got)
		}
	}
	prefix := strings.Repeat("a", 40)
	if ChildName(prefix+"-first") == ChildName(prefix+"-second") || ChildName("a.b") == ChildName("a-b") {
		t.Fatal("full-name hash identity lost")
	}
	if ChildName("sample") != "awcp-sample-af2bdbe1aa9b6ec1" {
		t.Fatal("naming compatibility changed")
	}
}
