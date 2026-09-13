// SPDX-License-Identifier: Apache-2.0
package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerEndpoints(t *testing.T) {
	for _, test := range []struct{ path, want string }{
		{"/health", "ok\n"}, {"/ready", "ok\n"}, {"/version", "test-version\n"},
	} {
		t.Run(test.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			newHandler("test-version").ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
			if recorder.Code != http.StatusOK || recorder.Body.String() != test.want {
				t.Fatalf("status/body = %d/%q, want 200/%q", recorder.Code, recorder.Body.String(), test.want)
			}
		})
	}
}
