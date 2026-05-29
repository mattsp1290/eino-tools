package urlfetch_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mattsp1290/eino-tools/result"
	"github.com/mattsp1290/eino-tools/urlfetch"
)

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("nil opts succeeds default client populated", func(t *testing.T) {
		t.Parallel()
		tool, err := urlfetch.New()
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if tool == nil {
			t.Fatal("New() returned nil tool")
		}
	})

	t.Run("explicit http client preserved", func(t *testing.T) {
		t.Parallel()
		client := &http.Client{}
		tool, err := urlfetch.New(urlfetch.Options{HTTPClient: client})
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if tool == nil {
			t.Fatal("New() returned nil tool")
		}
	})

	t.Run("multiple options returns error", func(t *testing.T) {
		t.Parallel()
		_, err := urlfetch.New(urlfetch.Options{}, urlfetch.Options{})
		if err == nil {
			t.Fatal("expected error for multiple options, got nil")
		}
	})
}

func TestTool_Run_FileScheme(t *testing.T) {
	t.Parallel()

	tool, err := urlfetch.New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	t.Run("existing file returns succeeded and content", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "hello.txt")
		if err := os.WriteFile(path, []byte("hello world"), 0o600); err != nil {
			t.Fatal(err)
		}
		res := tool.Run(context.Background(), urlfetch.Args{URL: "file://" + path})
		if res.Outcome != result.OutcomeSucceeded {
			t.Fatalf("outcome = %s, want succeeded; error = %+v", res.Outcome, res.Error)
		}
		if res.Content != "hello world" {
			t.Fatalf("content = %q, want %q", res.Content, "hello world")
		}
	})

	t.Run("non-existent path returns not_found", func(t *testing.T) {
		t.Parallel()
		res := tool.Run(context.Background(), urlfetch.Args{URL: "file:///no/such/file.txt"})
		if res.Outcome != result.OutcomeFailed {
			t.Fatalf("outcome = %s, want failed", res.Outcome)
		}
		if res.Error == nil || res.Error.Category != urlfetch.ErrCategoryNotFound {
			t.Fatalf("error category = %v, want %s", res.Error, urlfetch.ErrCategoryNotFound)
		}
	})

	t.Run("directory path returns io error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		res := tool.Run(context.Background(), urlfetch.Args{URL: "file://" + dir})
		if res.Outcome != result.OutcomeFailed {
			t.Fatalf("outcome = %s, want failed", res.Outcome)
		}
		if res.Error == nil {
			t.Fatal("expected error, got nil")
		}
		// directory reads as io or not_found depending on OS behavior
		if res.Error.Category != urlfetch.ErrCategoryIO && res.Error.Category != urlfetch.ErrCategoryNotFound {
			t.Fatalf("error category = %s, want io or not_found", res.Error.Category)
		}
	})

	t.Run("file with percent-encoded path decodes correctly", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "hello world.txt")
		if err := os.WriteFile(path, []byte("encoded"), 0o600); err != nil {
			t.Fatal(err)
		}
		// url.Parse will decode %20 in u.Path
		res := tool.Run(context.Background(), urlfetch.Args{URL: "file://" + filepath.Join(dir, "hello%20world.txt")})
		if res.Outcome != result.OutcomeSucceeded {
			t.Fatalf("outcome = %s, want succeeded; error = %+v", res.Outcome, res.Error)
		}
		if res.Content != "encoded" {
			t.Fatalf("content = %q, want %q", res.Content, "encoded")
		}
	})

	t.Run("file URL with non-empty host rejected", func(t *testing.T) {
		t.Parallel()
		res := tool.Run(context.Background(), urlfetch.Args{URL: "file://host/path"})
		if res.Outcome != result.OutcomeFailed {
			t.Fatalf("outcome = %s, want failed", res.Outcome)
		}
		if res.Error == nil || res.Error.Category != urlfetch.ErrCategoryValidation {
			t.Fatalf("error category = %v, want %s", res.Error, urlfetch.ErrCategoryValidation)
		}
	})
}

func TestTool_Run_HTTPSScheme(t *testing.T) {
	t.Parallel()

	t.Run("200 response returns succeeded and body", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("response body"))
		}))
		defer srv.Close()

		tool, err := urlfetch.New(urlfetch.Options{HTTPClient: srv.Client()})
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		res := tool.Run(context.Background(), urlfetch.Args{URL: srv.URL})
		if res.Outcome != result.OutcomeSucceeded {
			t.Fatalf("outcome = %s, want succeeded; error = %+v", res.Outcome, res.Error)
		}
		if res.Content != "response body" {
			t.Fatalf("content = %q, want %q", res.Content, "response body")
		}
	})

	t.Run("404 response returns not_found", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		tool, err := urlfetch.New(urlfetch.Options{HTTPClient: srv.Client()})
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		res := tool.Run(context.Background(), urlfetch.Args{URL: srv.URL})
		if res.Outcome != result.OutcomeFailed {
			t.Fatalf("outcome = %s, want failed", res.Outcome)
		}
		if res.Error == nil || res.Error.Category != urlfetch.ErrCategoryNotFound {
			t.Fatalf("error category = %v, want %s", res.Error, urlfetch.ErrCategoryNotFound)
		}
	})

	t.Run("500 response returns network error", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		tool, err := urlfetch.New(urlfetch.Options{HTTPClient: srv.Client()})
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		res := tool.Run(context.Background(), urlfetch.Args{URL: srv.URL})
		if res.Outcome != result.OutcomeFailed {
			t.Fatalf("outcome = %s, want failed", res.Outcome)
		}
		if res.Error == nil || res.Error.Category != urlfetch.ErrCategoryNetwork {
			t.Fatalf("error category = %v, want %s", res.Error, urlfetch.ErrCategoryNetwork)
		}
	})

	t.Run("connection refused returns network error", func(t *testing.T) {
		t.Parallel()
		// Use a server we immediately close to get connection refused
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
		client := srv.Client()
		serverURL := srv.URL
		srv.Close()

		tool, err := urlfetch.New(urlfetch.Options{HTTPClient: client})
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		res := tool.Run(context.Background(), urlfetch.Args{URL: serverURL})
		if res.Outcome != result.OutcomeFailed {
			t.Fatalf("outcome = %s, want failed", res.Outcome)
		}
		if res.Error == nil || res.Error.Category != urlfetch.ErrCategoryNetwork {
			t.Fatalf("error category = %v, want %s", res.Error, urlfetch.ErrCategoryNetwork)
		}
	})
}

func TestTool_Run_Validation(t *testing.T) {
	t.Parallel()

	tool, err := urlfetch.New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		name    string
		url     string
		wantCat string
	}{
		{
			name:    "empty URL",
			url:     "",
			wantCat: urlfetch.ErrCategoryValidation,
		},
		{
			name:    "http scheme rejected",
			url:     "http://example.com",
			wantCat: urlfetch.ErrCategoryValidation,
		},
		{
			name:    "ftp scheme rejected",
			url:     "ftp://example.com",
			wantCat: urlfetch.ErrCategoryValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			res := tool.Run(context.Background(), urlfetch.Args{URL: tt.url})
			if res.Outcome != result.OutcomeFailed {
				t.Fatalf("outcome = %s, want failed", res.Outcome)
			}
			if res.Error == nil || res.Error.Category != tt.wantCat {
				t.Fatalf("error category = %v, want %s", res.Error, tt.wantCat)
			}
		})
	}

	t.Run("http scheme error message names scheme and suggests https", func(t *testing.T) {
		t.Parallel()
		res := tool.Run(context.Background(), urlfetch.Args{URL: "http://example.com"})
		if res.Error == nil {
			t.Fatal("expected error")
		}
		msg := res.Error.Message
		if len(msg) == 0 {
			t.Fatal("expected non-empty error message")
		}
		// message must mention the rejected scheme and suggest https
		if !contains(msg, "http") {
			t.Errorf("message %q does not name rejected scheme 'http'", msg)
		}
		if !contains(msg, "https") {
			t.Errorf("message %q does not suggest https", msg)
		}
	})

	t.Run("canceled ctx returns canceled error before any IO", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		res := tool.Run(ctx, urlfetch.Args{URL: "https://example.com"})
		if res.Outcome != result.OutcomeFailed {
			t.Fatalf("outcome = %s, want failed", res.Outcome)
		}
		if res.Error == nil || res.Error.Category != urlfetch.ErrCategoryCanceled {
			t.Fatalf("error category = %v, want %s", res.Error, urlfetch.ErrCategoryCanceled)
		}
	})
}

func TestTool_InvokableRun(t *testing.T) {
	t.Parallel()

	tool, err := urlfetch.New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	t.Run("empty JSON returns error", func(t *testing.T) {
		t.Parallel()
		_, err := tool.InvokableRun(context.Background(), "")
		if err == nil {
			t.Fatal("expected error for empty JSON, got nil")
		}
	})

	t.Run("duplicate keys rejected", func(t *testing.T) {
		t.Parallel()
		_, err := tool.InvokableRun(context.Background(), `{"url":"a","url":"b"}`)
		if err == nil {
			t.Fatal("expected error for duplicate keys, got nil")
		}
	})

	t.Run("valid JSON round-trips", func(t *testing.T) {
		t.Parallel()
		out, err := tool.InvokableRun(context.Background(), `{"url":"ftp://bad"}`)
		if err != nil {
			t.Fatalf("InvokableRun error = %v", err)
		}
		if out == "" {
			t.Fatal("expected non-empty output")
		}
	})
}

func TestResult_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	raw := `{"outcome":"succeeded","content":"hello"}`
	var res urlfetch.Result
	if err := (&res).UnmarshalJSON([]byte(raw)); err != nil {
		t.Fatalf("UnmarshalJSON error = %v", err)
	}
	if string(res.RawJSON) != raw {
		t.Fatalf("RawJSON = %s, want %s", res.RawJSON, raw)
	}
	if res.Outcome != result.OutcomeSucceeded {
		t.Fatalf("Outcome = %s, want succeeded", res.Outcome)
	}
	if res.Content != "hello" {
		t.Fatalf("Content = %q, want hello", res.Content)
	}
}

func TestSchema(t *testing.T) {
	t.Parallel()

	s := urlfetch.Schema()
	if len(s) == 0 {
		t.Fatal("Schema() returned empty")
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(s, &obj); err != nil {
		t.Fatalf("Schema() is not valid JSON: %v", err)
	}

	t.Run("returns valid JSON", func(t *testing.T) {
		t.Parallel()
		s2 := urlfetch.Schema()
		if string(s2) != string(s) {
			t.Fatal("Schema() returned different bytes on second call (not a fresh clone?)")
		}
	})

	t.Run("additionalProperties is false", func(t *testing.T) {
		t.Parallel()
		ap, ok := obj["additionalProperties"]
		if !ok {
			t.Fatal("additionalProperties not found in schema")
		}
		if ap != false {
			t.Fatalf("additionalProperties = %v, want false", ap)
		}
	})

	t.Run("url is required", func(t *testing.T) {
		t.Parallel()
		req, ok := obj["required"].([]interface{})
		if !ok {
			t.Fatal("required field not found or wrong type")
		}
		found := false
		for _, f := range req {
			if f == "url" {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("url not in required fields")
		}
	})
}

func TestIsRetryable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		res  urlfetch.Result
		want bool
	}{
		{
			name: "succeeded",
			res:  urlfetch.Result{Outcome: result.OutcomeSucceeded},
			want: false,
		},
		{
			name: "not_found",
			res: urlfetch.Result{
				Outcome: result.OutcomeFailed,
				Error:   &urlfetch.ResultError{Category: urlfetch.ErrCategoryNotFound},
			},
			want: false,
		},
		{
			name: "validation",
			res: urlfetch.Result{
				Outcome: result.OutcomeFailed,
				Error:   &urlfetch.ResultError{Category: urlfetch.ErrCategoryValidation},
			},
			want: false,
		},
		{
			name: "canceled",
			res: urlfetch.Result{
				Outcome: result.OutcomeFailed,
				Error:   &urlfetch.ResultError{Category: urlfetch.ErrCategoryCanceled},
			},
			want: false,
		},
		{
			name: "io",
			res: urlfetch.Result{
				Outcome: result.OutcomeFailed,
				Error:   &urlfetch.ResultError{Category: urlfetch.ErrCategoryIO},
			},
			want: false,
		},
		{
			name: "network",
			res: urlfetch.Result{
				Outcome: result.OutcomeFailed,
				Error:   &urlfetch.ResultError{Category: urlfetch.ErrCategoryNetwork},
			},
			want: true,
		},
		{
			name: "unknown",
			res: urlfetch.Result{
				Outcome: result.OutcomeFailed,
				Error:   &urlfetch.ResultError{Category: urlfetch.ErrCategoryUnknown},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.res.IsRetryable(); got != tt.want {
				t.Fatalf("IsRetryable() = %t, want %t", got, tt.want)
			}
		})
	}
}

// contains is a case-sensitive substring check.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
