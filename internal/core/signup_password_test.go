package core

import (
	"io"
	"strings"
	"testing"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"

	"reg_go/internal/browser"
)

type passwordTestClient struct {
	tls_client.HttpClient
	do func(*fhttp.Request) (*fhttp.Response, error)
}

func (c *passwordTestClient) Do(req *fhttp.Request) (*fhttp.Response, error) { return c.do(req) }

func passwordTestResponse(status int, body string) *fhttp.Response {
	return &fhttp.Response{StatusCode: status, Header: make(fhttp.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestPasswordCreationStepIDUsesWorkflowResponse(t *testing.T) {
	if got := passwordCreationStepID(map[string]interface{}{"stepId": "current-password-step"}); got != "current-password-step" {
		t.Fatalf("step id = %q", got)
	}
	if got := passwordCreationStepID(map[string]interface{}{}); got != "get-new-password-for-password-creation" {
		t.Fatalf("fallback step id = %q", got)
	}
}

func TestReadBuilderIDSessionMatchesFrontendCookieRead(t *testing.T) {
	client := &passwordTestClient{}
	client.do = func(req *fhttp.Request) (*fhttp.Response, error) {
		if req.URL.Path != "/platform/source-directory/cookieread" {
			t.Fatalf("path = %q", req.URL.Path)
		}
		if req.Header.Get("sec-fetch-dest") != "iframe" {
			t.Fatalf("sec-fetch-dest = %q", req.Header.Get("sec-fetch-dest"))
		}
		return passwordTestResponse(fhttp.StatusOK, `{"cookieValue":"builder-session"}`), nil
	}
	r := &Registrar{
		Cfg:      &Config{SigninBase: "https://signin.example.com"},
		Client:   client,
		Identity: &browser.BrowserIdentity{},
	}
	value, err := r.readBuilderIDSession("source-directory", "https://signin.example.com/signup")
	if err != nil || value != "builder-session" {
		t.Fatalf("value=%q err=%v", value, err)
	}
}
