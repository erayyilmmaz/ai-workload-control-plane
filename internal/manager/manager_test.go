// SPDX-License-Identifier: Apache-2.0
package manager

import (
	"testing"
	"time"
)

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

func TestLeaderElectionTimingIsBounded(t *testing.T) {
	valid := Options{WatchNamespace: "awcp-workloads", ManagerNamespace: "awcp-system", LeaseDuration: 15 * time.Second, RenewDeadline: 10 * time.Second, RetryPeriod: 2 * time.Second}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Options){
		func(o *Options) { o.LeaseDuration = 10 * time.Second; o.RenewDeadline = 10 * time.Second },
		func(o *Options) { o.RetryPeriod = 10 * time.Second },
		func(o *Options) { o.RetryPeriod = -time.Second },
	} {
		o := valid
		mutate(&o)
		if err := o.Validate(); err == nil {
			t.Fatal("unsafe leader election timing accepted")
		}
	}
}

func TestMultipleWatchNamespacesAreExplicitAndBounded(t *testing.T) {
	opts := Options{WatchNamespaces: []string{"awcp-workloads", "awcp-tenant-alpha"}, ManagerNamespace: "awcp-system"}
	if err := opts.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := opts.EffectiveWatchNamespaces(); len(got) != 2 || got[1] != "awcp-tenant-alpha" {
		t.Fatalf("unexpected effective namespaces: %v", got)
	}
	if err := (Options{WatchNamespace: "awcp-workloads", WatchNamespaces: []string{"awcp-tenant-alpha"}, ManagerNamespace: "awcp-system"}).Validate(); err == nil {
		t.Fatal("legacy and list scopes must not be combined")
	}
	if err := (Options{WatchNamespaces: []string{"awcp-tenant-alpha", "awcp-tenant-alpha"}, ManagerNamespace: "awcp-system"}).Validate(); err == nil {
		t.Fatal("duplicate tenant scopes must be rejected")
	}
	if got := ParseWatchNamespaces("awcp-tenant-alpha,awcp-tenant-bravo"); len(got) != 2 || got[0] != "awcp-tenant-alpha" || got[1] != "awcp-tenant-bravo" {
		t.Fatalf("unexpected parsed namespaces: %v", got)
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
