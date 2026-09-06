package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"

	"reg_go/internal/browser"
	"reg_go/internal/captcha"
)

const testAMSRoot = "unit-test.threat-mitigation.aws.amazon.com"

type amsTestClient struct {
	tls_client.HttpClient
	do func(*fhttp.Request) (*fhttp.Response, error)
}

func (c *amsTestClient) Do(req *fhttp.Request) (*fhttp.Response, error) { return c.do(req) }

func amsTestResponse(status int, body string) *fhttp.Response {
	return &fhttp.Response{StatusCode: status, Header: make(fhttp.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func testAMSToken(t *testing.T) string {
	t.Helper()
	payload, err := json.Marshal(amsTokenPayload{APIRoot: testAMSRoot, EnvStage: "prod", Region: "us-east-1"})
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(payload)
}

func testWAFGridMetadata(t *testing.T) string {
	t.Helper()
	payload, err := json.Marshal(amsWAFGridMetadata{ScriptURL: "https://captcha.example.com/jsapi.js", APIKey: "website-key", ClientID: "client-id"})
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(payload)
}

func challengeBody(token string) []byte {
	return []byte(`{"captchaResponse":{"captchaToken":"` + token + `","captchaCDN":"https://cdn.example.com/consumer.js"}}`)
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
	client := &amsTestClient{}
	client.do = func(req *fhttp.Request) (*fhttp.Response, error) {
		if req.URL.Path != "/platform/source-directory/cookieread" {
			t.Fatalf("path = %q", req.URL.Path)
		}
		if req.Header.Get("sec-fetch-dest") != "iframe" {
			t.Fatalf("sec-fetch-dest = %q", req.Header.Get("sec-fetch-dest"))
		}
		return amsTestResponse(fhttp.StatusOK, `{"cookieValue":"builder-session"}`), nil
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

func TestSolvePasswordWAFChallengeCompletesAMCSProtocol(t *testing.T) {
	token := testAMSToken(t)
	requests := make([]map[string]string, 0, 4)
	client := &amsTestClient{}
	client.do = func(req *fhttp.Request) (*fhttp.Response, error) {
		if req.URL.Host == "images.example.com" {
			return amsTestResponse(fhttp.StatusOK, "image-bytes"), nil
		}
		var payload map[string]string
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			return nil, err
		}
		requests = append(requests, payload)
		switch req.URL.Path {
		case "/challenge":
			return amsTestResponse(fhttp.StatusOK, `{"challengeType":"AMCS","sessionId":"session-id","redemptionToken":"rotated-token"}`), nil
		case "/amcs/image":
			return amsTestResponse(fhttp.StatusOK, `{"imageCdnUrl":"https://images.example.com/captcha.png"}`), nil
		case "/amcs/verify":
			return amsTestResponse(fhttp.StatusOK, `{"amcsAccessCode":"amcs-access-code"}`), nil
		case "/submit":
			return amsTestResponse(fhttp.StatusOK, `{"accessCode":"final-access-code"}`), nil
		default:
			return amsTestResponse(fhttp.StatusNotFound, `{}`), nil
		}
	}
	r := &Registrar{
		Cfg:      &Config{},
		Client:   client,
		Cookies:  map[string]string{},
		Identity: &browser.BrowserIdentity{},
		solveImage: func(_ context.Context, image []byte) (string, error) {
			if string(image) != "image-bytes" {
				t.Fatalf("image = %q", image)
			}
			return "typed-answer", nil
		},
	}
	payload := map[string]interface{}{}
	handled, err := r.solvePasswordWAFChallenge(challengeBody(token), "https://us-east-1.signin.aws/platform/test/signup", payload)
	if err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	captchaRequest, _ := payload["captchaRequest"].(map[string]string)
	if captchaRequest["captchaAccessCode"] != "final-access-code" {
		t.Fatalf("captcha request = %#v", captchaRequest)
	}
	if len(requests) != 4 || requests[0]["redemptionToken"] != token || requests[1]["sessionId"] != "session-id" ||
		requests[2]["guess"] != "typed-answer" || requests[3]["redemptionToken"] != "rotated-token" ||
		requests[3]["challengeAccessCode"] != "amcs-access-code" {
		t.Fatalf("unexpected AMS requests: %#v", requests)
	}
}

func TestSolvePasswordWAFChallengeMapsWAFGridResult(t *testing.T) {
	token := testAMSToken(t)
	metadata := testWAFGridMetadata(t)
	client := &amsTestClient{}
	client.do = func(req *fhttp.Request) (*fhttp.Response, error) {
		switch req.URL.Path {
		case "/challenge":
			return amsTestResponse(fhttp.StatusOK, `{"challengeType":"WAF_GRID","challengeMetadata":"`+metadata+`"}`), nil
		case "/submit":
			return amsTestResponse(fhttp.StatusOK, `{"accessCode":"final-access-code"}`), nil
		default:
			return amsTestResponse(fhttp.StatusNotFound, `{}`), nil
		}
	}
	var received captcha.AWSWAFOptions
	r := &Registrar{
		Cfg:      &Config{Proxy: "http://127.0.0.1:7890"},
		Client:   client,
		Cookies:  map[string]string{},
		Identity: &browser.BrowserIdentity{},
		solveAWSWAF: func(_ context.Context, options captcha.AWSWAFOptions) (captcha.AWSWAFSolution, error) {
			received = options
			return captcha.AWSWAFSolution{ExistingToken: "waf-token", CaptchaVoucher: "voucher"}, nil
		},
	}
	payload := map[string]interface{}{}
	handled, err := r.solvePasswordWAFChallenge(challengeBody(token), "https://us-east-1.signin.aws/platform/test/signup", payload)
	if err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	if received.WebsiteKey != "website-key" || received.JSAPIScript != "https://captcha.example.com/jsapi.js" || received.Proxy != "" {
		t.Fatalf("2Captcha options = %#v", received)
	}
	if r.Cookies["aws-waf-token"] != "waf-token" {
		t.Fatalf("cookies = %#v", r.Cookies)
	}
	captchaRequest, _ := payload["captchaRequest"].(map[string]string)
	if captchaRequest["captchaAccessCode"] != "final-access-code" {
		t.Fatalf("captcha request = %#v", captchaRequest)
	}
}

func TestSolvePasswordWAFFailurePreservesDiagnostics(t *testing.T) {
	token := testAMSToken(t)
	metadata := testWAFGridMetadata(t)
	client := &amsTestClient{}
	client.do = func(req *fhttp.Request) (*fhttp.Response, error) {
		return amsTestResponse(fhttp.StatusOK, `{"challengeType":"WAF_GRID","challengeMetadata":"`+metadata+`"}`), nil
	}
	upstream := &captcha.APIError{ErrorID: 12, Code: "ERROR_CAPTCHA_UNSOLVABLE", Description: "Workers could not solve the Captcha", Operation: "getTaskResult", TaskType: "AmazonTaskProxyless", TaskID: 42}
	r := &Registrar{
		Cfg:      &Config{},
		Client:   client,
		Cookies:  map[string]string{"session": "session-value"},
		Identity: &browser.BrowserIdentity{},
		solveAWSWAF: func(context.Context, captcha.AWSWAFOptions) (captcha.AWSWAFSolution, error) {
			return captcha.AWSWAFSolution{}, upstream
		},
	}
	payload := map[string]interface{}{"requestId": "request-id"}
	handled, err := r.solvePasswordWAFChallenge(challengeBody(token), "https://us-east-1.signin.aws/signup", payload)
	if handled {
		t.Fatal("failed challenge was accepted")
	}
	var apiErr *captcha.APIError
	if !errors.As(err, &apiErr) || apiErr != upstream {
		t.Fatalf("lost provider diagnostics: %v", err)
	}
	if payload["requestId"] != "request-id" || payload["captchaRequest"] != nil || r.Cookies["session"] != "session-value" {
		t.Fatalf("failure changed request state: payload=%#v cookies=%#v", payload, r.Cookies)
	}
	message := r.formatError("SetPassword", err)
	for _, detail := range []string{"AWS WAF 动态验证失败", "ERROR_CAPTCHA_UNSOLVABLE", "taskId=42"} {
		if !strings.Contains(message, detail) {
			t.Fatalf("task log lost diagnostic %q: %s", detail, message)
		}
	}
	if strings.Contains(message, token) || strings.Contains(message, "session-value") {
		t.Fatalf("task log leaked session data: %s", message)
	}
}

func TestSolvePasswordWAFChallengeIgnoresIncompleteResponse(t *testing.T) {
	r := &Registrar{Cfg: &Config{}, Cookies: map[string]string{}}
	handled, err := r.solvePasswordWAFChallenge(
		[]byte(`{"captchaResponse":{"captchaToken":"redemption-token"}}`),
		"https://signin.example.com/signup",
		map[string]interface{}{},
	)
	if err != nil || handled {
		t.Fatalf("got handled=%v, err=%v", handled, err)
	}
}
