package captcha

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSolveAWSWAFFailureKeepsDiagnosticsAndStops(t *testing.T) {
	for _, tc := range []struct {
		name      string
		operation string
		code      string
		errorID   int
		taskID    int64
	}{
		{"creation", "createTask", "ERROR_BAD_PARAMETERS", 110, 0},
		{"solving", "getTaskResult", "ERROR_CAPTCHA_UNSOLVABLE", 12, 42},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var creates, polls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/createTask":
					creates.Add(1)
					if tc.operation != "createTask" {
						json.NewEncoder(w).Encode(map[string]interface{}{"errorId": 0, "taskId": 42})
						return
					}
				case "/getTaskResult":
					polls.Add(1)
				default:
					http.NotFound(w, r)
					return
				}
				json.NewEncoder(w).Encode(map[string]interface{}{
					"errorId":          tc.errorID,
					"errorCode":        tc.code,
					"errorDescription": "Solver rejected the task",
					"captchaToken":     "response-secret",
				})
			}))
			defer server.Close()
			client := NewClient("api-key-secret")
			client.APIBase = server.URL
			client.PollInterval = time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, err := client.SolveAWSWAFResult(ctx, AWSWAFOptions{
				WebsiteURL:  "https://example.com/protected",
				WebsiteKey:  "challenge-secret",
				JSAPIScript: "https://example.com/jsapi.js",
			})
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected APIError, got %v", err)
			}
			if apiErr.ErrorID != tc.errorID || apiErr.Code != tc.code || apiErr.Operation != tc.operation || apiErr.TaskID != tc.taskID || apiErr.TaskType != "AmazonTaskProxyless" {
				t.Fatalf("missing failure diagnostics: %+v", apiErr)
			}
			for _, field := range []string{tc.code, "operation=" + tc.operation, "type=AmazonTaskProxyless"} {
				if !strings.Contains(err.Error(), field) {
					t.Fatalf("missing %q from error: %v", field, err)
				}
			}
			if strings.Contains(err.Error(), "taskId=42") != (tc.taskID == 42) {
				t.Fatalf("incorrect accepted task ID in error: %v", err)
			}
			for _, secret := range []string{"api-key-secret", "challenge-secret", "response-secret"} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("error leaked a request or response value: %v", err)
				}
			}
			wantPolls := int32(0)
			if tc.taskID != 0 {
				wantPolls = 1
			}
			if creates.Load() != 1 || polls.Load() != wantPolls {
				t.Fatalf("solver resubmitted or kept polling a failed task: creates=%d, polls=%d", creates.Load(), polls.Load())
			}
		})
	}
}

func TestSolveAWSWAFReturnsExistingTokenAndUsesTaskProxy(t *testing.T) {
	var createTask map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/createTask":
			if err := json.NewDecoder(r.Body).Decode(&createTask); err != nil {
				t.Fatal(err)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"errorId": 0, "taskId": 42})
		case "/getTaskResult":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"errorId": 0,
				"status":  "ready",
				"solution": map[string]string{
					"captcha_voucher": "voucher",
					"existing_token":  "waf-token",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient("api-key")
	client.APIBase = server.URL
	client.PollInterval = time.Millisecond
	token, err := client.SolveAWSWAF(context.Background(), AWSWAFOptions{
		WebsiteURL: "https://example.com/protected",
		WebsiteKey: "site-key",
		IV:         "iv",
		Context:    "context",
		Proxy:      "socks5://user:password@127.0.0.1:1080",
	})
	if err != nil {
		t.Fatal(err)
	}
	if token != "waf-token" {
		t.Fatalf("token = %q", token)
	}
	task, _ := createTask["task"].(map[string]interface{})
	if task["type"] != "AmazonTask" || task["proxyType"] != "socks5" || task["proxyAddress"] != "127.0.0.1" {
		t.Fatalf("unexpected task: %#v", task)
	}
	if task["proxyLogin"] != "user" || task["proxyPassword"] != "password" {
		t.Fatalf("proxy credentials missing: %#v", task)
	}
}

func TestSolveAWSWAFDoesNotAcceptVoucherAsToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/createTask" {
			json.NewEncoder(w).Encode(map[string]interface{}{"errorId": 0, "taskId": 42})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errorId":  0,
			"status":   "ready",
			"solution": map[string]string{"captcha_voucher": "voucher"},
		})
	}))
	defer server.Close()

	client := NewClient("api-key")
	client.APIBase = server.URL
	client.PollInterval = time.Millisecond
	_, err := client.SolveAWSWAF(context.Background(), AWSWAFOptions{
		WebsiteURL:  "https://example.com/protected",
		WebsiteKey:  "site-key",
		JSAPIScript: "https://example.com/jsapi.js",
	})
	if err == nil {
		t.Fatal("expected missing existing_token error")
	}
}

func TestSolveAWSWAFResultReturnsVoucherAndToken(t *testing.T) {
	var createTask map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/createTask" {
			if err := json.NewDecoder(r.Body).Decode(&createTask); err != nil {
				t.Fatal(err)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"errorId": 0, "taskId": 42})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errorId": 0,
			"status":  "ready",
			"solution": map[string]string{
				"captcha_voucher": "voucher",
				"existing_token":  "waf-token",
			},
		})
	}))
	defer server.Close()

	client := NewClient("api-key")
	client.APIBase = server.URL
	client.PollInterval = time.Millisecond
	solution, err := client.SolveAWSWAFResult(context.Background(), AWSWAFOptions{
		WebsiteURL:  "https://example.com/protected",
		JSAPIScript: "https://example.com/jsapi.js",
		Proxy:       "http://203.0.113.10:8080",
	})
	if err != nil {
		t.Fatal(err)
	}
	if solution.ExistingToken != "waf-token" || solution.CaptchaVoucher != "voucher" {
		t.Fatalf("unexpected solution: %#v", solution)
	}
	task, _ := createTask["task"].(map[string]interface{})
	if task["jsapiScript"] != "https://example.com/jsapi.js" {
		t.Fatalf("jsapiScript missing from task: %#v", task)
	}
	if _, exists := task["websiteKey"]; exists {
		t.Fatalf("jsapiScript task must not invent a websiteKey: %#v", task)
	}
	if task["type"] != "AmazonTask" {
		t.Fatalf("jsapiScript without websiteKey must use AmazonTask: %#v", task)
	}
}

func TestSolveAWSWAFRejectsProxylessJSAPIWithoutWebsiteKey(t *testing.T) {
	client := NewClient("api-key")
	_, err := client.SolveAWSWAFResult(context.Background(), AWSWAFOptions{
		WebsiteURL:  "https://example.com/protected",
		JSAPIScript: "https://example.com/jsapi.js",
	})
	if err == nil {
		t.Fatal("expected proxyless jsapiScript without websiteKey to fail")
	}
}

func TestSolveAWSWAFPrefersCompleteChallengeParameters(t *testing.T) {
	var createTask map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/createTask" {
			if err := json.NewDecoder(r.Body).Decode(&createTask); err != nil {
				t.Fatal(err)
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"errorId": 0, "taskId": 42})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errorId":  0,
			"status":   "ready",
			"solution": map[string]string{"existing_token": "waf-token"},
		})
	}))
	defer server.Close()

	client := NewClient("api-key")
	client.APIBase = server.URL
	client.PollInterval = time.Millisecond
	_, err := client.SolveAWSWAF(context.Background(), AWSWAFOptions{
		WebsiteURL:  "https://example.com/protected",
		WebsiteKey:  "site-key",
		IV:          "iv",
		Context:     "context",
		JSAPIScript: "https://example.com/jsapi.js",
	})
	if err != nil {
		t.Fatal(err)
	}
	task, _ := createTask["task"].(map[string]interface{})
	if task["iv"] != "iv" || task["context"] != "context" {
		t.Fatalf("challenge parameters missing: %#v", task)
	}
	if _, exists := task["jsapiScript"]; exists {
		t.Fatalf("jsapiScript must not be mixed with challenge parameters: %#v", task)
	}
}

func TestParseProxyRejectsUnsupportedScheme(t *testing.T) {
	if _, err := parseProxy("ftp://127.0.0.1:21"); err == nil {
		t.Fatal("expected unsupported proxy scheme error")
	}
}

func TestRemoteWorkerProxyRejectsLocallyScopedAddresses(t *testing.T) {
	for _, proxy := range []string{
		"http://127.0.0.1:7890",
		"socks5://localhost:1080",
		"http://192.168.1.10:8080",
		"http://[::1]:8080",
	} {
		if got := RemoteWorkerProxy(proxy); got != "" {
			t.Fatalf("RemoteWorkerProxy(%q) = %q", proxy, got)
		}
	}
	const publicProxy = "http://user:pass@203.0.113.10:8080"
	if got := RemoteWorkerProxy(publicProxy); got != publicProxy {
		t.Fatalf("RemoteWorkerProxy(%q) = %q", publicProxy, got)
	}
}
