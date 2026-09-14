// SPDX-License-Identifier: Apache-2.0
package capability

import (
	"context"
	"errors"
	"testing"
)

type lookupResult struct {
	found bool
	err   error
}

type fakeLookup map[APIResource]lookupResult

func (f fakeLookup) HasResource(_ context.Context, resource APIResource) (bool, error) {
	result, ok := f[resource]
	if !ok {
		return false, nil
	}
	return result.found, result.err
}

func TestRequirementsAreIsolatedCopies(t *testing.T) {
	first := Requirements()
	first[0].Resources[0].Resource = "mutated"
	second := Requirements()
	if second[0].Resources[0].Resource == "mutated" {
		t.Fatal("callers can mutate the capability catalog")
	}
	if _, ok := RequirementFor("unknown"); ok {
		t.Fatal("unknown feature unexpectedly has a requirement")
	}
}

func TestDetectDistinguishesAvailableUnavailableAndUnknown(t *testing.T) {
	requirement, ok := RequirementFor(GatewayAPI)
	if !ok {
		t.Fatal("Gateway API requirement is missing")
	}
	allAvailable := fakeLookup{}
	for _, resource := range requirement.Resources {
		allAvailable[resource] = lookupResult{found: true}
	}
	if observation := Detect(t.Context(), allAvailable, requirement); observation.State != Available || observation.Err != nil {
		t.Fatalf("available observation = %#v", observation)
	}
	missing := fakeLookup{requirement.Resources[0]: {found: true}}
	if observation := Detect(t.Context(), missing, requirement); observation.State != Unavailable || observation.Err != nil {
		t.Fatalf("missing optional API observation = %#v", observation)
	}
	discoveryFailure := fakeLookup{requirement.Resources[0]: {err: errors.New("synthetic discovery failure")}}
	if observation := Detect(t.Context(), discoveryFailure, requirement); observation.State != Unknown || observation.Err == nil {
		t.Fatalf("discovery failure observation = %#v", observation)
	}
}
