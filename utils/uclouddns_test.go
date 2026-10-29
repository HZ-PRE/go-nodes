package utils

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUCallResponse(t *testing.T) {
	for _, tt := range []struct {
		name, body, wantError string
		status                int
	}{
		{"success", `{"RetCode":0,"ID":9007199254740993}`, "", 200},
		{"api failure", `{"RetCode":42,"Message":"denied"}`, "ret=42 msg=denied", 200},
		{"http failure", `unavailable`, "body=unavailable", 503},
		{"invalid JSON", `{"RetCode":`, "response parse failed", 200},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				if r.Form.Get("Action") != "TestAction" || r.Form.Get("Signature") == "" {
					t.Error("missing action or signature")
				}
				w.WriteHeader(tt.status)
				io.WriteString(w, tt.body)
			}))
			old := ucloudHTTPClient
			ucloudHTTPClient = server.Client()
			t.Cleanup(func() { ucloudHTTPClient = old })
			params := map[string]string{"Action": "TestAction"}
			out, err := uCall("public", "private", "", "", params)
			if len(params) != 1 {
				t.Fatal("caller parameters were mutated")
			}
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("error = %v, want %q", err, tt.wantError)
				}
				if tt.name == "api failure" && out["RetCode"] != json.Number("42") {
					t.Fatalf("lost API error response: %v", out)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if out["ID"] != json.Number("9007199254740993") {
				t.Fatalf("large integer lost precision: %v", out["ID"])
			}
		})
	}
}
