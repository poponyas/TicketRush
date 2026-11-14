package web

import (
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbeddedFrontend(t *testing.T) {
	for _, tc := range []struct {
		path, contentType string
		code              int
	}{
		{"/", "text/html", 200}, {"/app.js", "text/javascript", 200},
		{"/views/account.js", "text/javascript", 200}, {"/assets/poster.svg", "image/svg+xml", 200},
		{"/missing.js", "", 404}, {"/views/", "", 404}, {"/.env", "", 404}, {"/api/v1/missing", "", 404},
	} {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
			require.Equal(t, tc.code, w.Code)
			if tc.code == 200 {
				require.Contains(t, w.Header().Get("Content-Type"), tc.contentType)
				require.Contains(t, w.Header().Get("Content-Security-Policy"), "script-src 'self'")
				require.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
			}
		})
	}
}
