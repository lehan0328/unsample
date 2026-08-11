package cli

import (
	"net/http"
	"testing"
)

func TestParseCurlSimpleGET(t *testing.T) {
	req, err := ParseCurl("curl https://api.example.com/users")
	if err != nil {
		t.Fatalf("ParseCurl: %v", err)
	}

	if req.Method != "GET" {
		t.Errorf("Method = %q, want GET", req.Method)
	}
	if req.URL.String() != "https://api.example.com/users" {
		t.Errorf("URL = %q, want %q", req.URL, "https://api.example.com/users")
	}
}

func TestParseCurlPOSTWithBody(t *testing.T) {
	req, err := ParseCurl(`curl -X POST -H 'Content-Type: application/json' -d '{"user":"test"}' https://api.example.com/users`)
	if err != nil {
		t.Fatalf("ParseCurl: %v", err)
	}

	if req.Method != "POST" {
		t.Errorf("Method = %q, want POST", req.Method)
	}
	if req.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want %q", req.Header.Get("Content-Type"), "application/json")
	}
	if req.Body == nil {
		t.Error("expected non-nil body")
	}
}

func TestParseCurlImplicitPOST(t *testing.T) {
	// When -d is used without -X, curl defaults to POST.
	req, err := ParseCurl(`curl -d 'data' https://api.example.com/submit`)
	if err != nil {
		t.Fatalf("ParseCurl: %v", err)
	}

	if req.Method != "POST" {
		t.Errorf("Method = %q, want POST (implicit from -d)", req.Method)
	}
}

func TestParseCurlMultipleHeaders(t *testing.T) {
	req, err := ParseCurl(`curl -H "Authorization: Bearer tok123" -H "Accept: application/json" https://api.example.com`)
	if err != nil {
		t.Fatalf("ParseCurl: %v", err)
	}

	if req.Header.Get("Authorization") != "Bearer tok123" {
		t.Errorf("Authorization = %q, want %q", req.Header.Get("Authorization"), "Bearer tok123")
	}
	if req.Header.Get("Accept") != "application/json" {
		t.Errorf("Accept = %q, want %q", req.Header.Get("Accept"), "application/json")
	}
}

func TestParseCurlMalformed(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
	}{
		{"no URL", "curl -X GET"},
		{"empty string", ""},
		{"unterminated quote", "curl 'https://example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseCurl(tt.cmd)
			if err == nil {
				t.Errorf("ParseCurl(%q) should return error", tt.cmd)
			}
		})
	}
}

func TestParseCurlSkipsUnknownFlags(t *testing.T) {
	// -s (silent), -k (insecure) should be silently skipped.
	req, err := ParseCurl("curl -s -k https://api.example.com/health")
	if err != nil {
		t.Fatalf("ParseCurl: %v", err)
	}

	if req.URL.String() != "https://api.example.com/health" {
		t.Errorf("URL = %q, want %q", req.URL, "https://api.example.com/health")
	}
}

func TestTokenizeQuotedStrings(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   []string
	}{
		{
			"single quotes",
			"curl -H 'Content-Type: application/json' https://example.com",
			[]string{"curl", "-H", "Content-Type: application/json", "https://example.com"},
		},
		{
			"double quotes",
			`curl -H "Authorization: Bearer token" https://example.com`,
			[]string{"curl", "-H", "Authorization: Bearer token", "https://example.com"},
		},
		{
			"mixed quotes",
			`curl -H 'Accept: */*' -d "body data" https://example.com`,
			[]string{"curl", "-H", "Accept: */*", "-d", "body data", "https://example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tokenize(tt.input)
			if err != nil {
				t.Fatalf("tokenize: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("tokenize returned %d tokens, want %d: %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("token[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// Verify that ParseCurl produces a request compatible with http.Client.
func TestParseCurlRequestIsUsable(t *testing.T) {
	req, err := ParseCurl("curl -X PUT -H 'X-Custom: val' https://httpbin.org/put")
	if err != nil {
		t.Fatalf("ParseCurl: %v", err)
	}

	// Verify it's a valid *http.Request that could be sent.
	if req.Method != http.MethodPut {
		t.Errorf("Method = %q, want PUT", req.Method)
	}
	if req.URL.Scheme != "https" {
		t.Errorf("Scheme = %q, want https", req.URL.Scheme)
	}
	if req.Header.Get("X-Custom") != "val" {
		t.Errorf("X-Custom header = %q, want %q", req.Header.Get("X-Custom"), "val")
	}
}
