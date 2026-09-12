// SPDX-License-Identifier: Apache-2.0
package manager

import "testing"

func TestNamespaceValidation(t *testing.T) {
	for _, value := range []string{"", " ", "default,other", "UPPER", "a/b", "default ", "a.b"} {
		for _, field := range []string{"watch", "manager"} {
			t.Run(field+"/"+value, func(t *testing.T) {
				opts := Options{WatchNamespace: "awcp-workloads", ManagerNamespace: "awcp-system"}
				if field == "watch" {
					opts.WatchNamespace = value
				} else {
					opts.ManagerNamespace = value
				}
				if opts.Validate() == nil {
					t.Fatalf("accepted invalid namespace %q", value)
				}
			})
		}
	}
	if err := (Options{WatchNamespace: "awcp-workloads", ManagerNamespace: "awcp-system"}).Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestReadinessRequiresStartup(t *testing.T) {
	r := &startupReadiness{}
	if !r.NeedLeaderElection() {
		t.Fatal("readiness must wait for leadership")
	}
	if r.check(nil) == nil {
		t.Fatal("ready before startup")
	}
	r.ready.Store(true)
	if err := r.check(nil); err != nil {
		t.Fatal(err)
	}
	r.ready.Store(false)
	if r.check(nil) == nil {
		t.Fatal("ready after shutdown")
	}
}
