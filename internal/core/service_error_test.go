package core

import (
	"strings"
	"testing"
)

func TestParseServiceErrorRedactsCaptchaAndRepairsMessage(t *testing.T) {
	body := []byte(`{
		"requestId":"outer-id",
		"message":{
			"text":"è¯·å°è¯éæ°ç»å½",
			"heading":"åçæå¤éè¯¯",
			"type":"ERROR",
			"requestId":"message-id",
			"errorCode":"AUTHENTICATION_FAILED"
		},
		"captchaResponse":{"captchaToken":"secret-token","captchaCDN":"https://example.com"}
	}`)

	err := parseServiceError(body)
	if err == nil {
		t.Fatal("expected a parsed service error")
	}
	if err.Code != "AUTHENTICATION_FAILED" || err.RequestID != "message-id" || !err.Captcha {
		t.Fatalf("unexpected parsed error: %#v", err)
	}
	if err.Message != "请尝试重新登录" {
		t.Fatalf("message = %q", err.Message)
	}
	if strings.Contains(err.Error(), "secret-token") || strings.Contains(err.Error(), "example.com") {
		t.Fatalf("sensitive CAPTCHA response leaked through error: %s", err)
	}
}

func TestRepairMojibakeLeavesUTF8TextAlone(t *testing.T) {
	for _, value := range []string{"请稍后重试", "café", "plain text"} {
		if got := repairMojibake(value); got != value {
			t.Fatalf("repairMojibake(%q) = %q", value, got)
		}
	}
}

func TestUnexpectedServiceResponseNeverIncludesRawBody(t *testing.T) {
	err := unexpectedServiceResponse("密码设置未返回 redirect", []byte(`not-json captchaToken=secret-token`))
	if strings.Contains(err.Error(), "secret-token") || strings.Contains(err.Error(), "captchaToken") {
		t.Fatalf("raw response leaked through error: %s", err)
	}
}

func TestParseAWSWAFChallengeRequiresTokenAndScript(t *testing.T) {
	challenge, ok := parseAWSWAFChallenge([]byte(`{
		"captchaResponse": {
			"captchaToken": " redemption-token ",
			"captchaCDN": " https://example.com/jsapi.js "
		}
	}`))
	if !ok || challenge.RedemptionToken != "redemption-token" || challenge.JSAPIScript != "https://example.com/jsapi.js" {
		t.Fatalf("unexpected challenge: %#v, ok=%v", challenge, ok)
	}

	if _, ok := parseAWSWAFChallenge([]byte(`{"captchaResponse":{"captchaToken":"token"}}`)); ok {
		t.Fatal("incomplete challenge was accepted")
	}
}

func TestFormatAuthenticationFailure(t *testing.T) {
	r := &Registrar{}
	got := r.formatError("SetPassword", &ServiceError{
		Code:      "AUTHENTICATION_FAILED",
		Message:   "请尝试重新登录",
		RequestID: "request-id",
		Captcha:   true,
	})
	want := "设置密码失败: AWS 身份验证失败，验证令牌可能无效或已过期，响应包含 CAPTCHA 验证信息 (AUTHENTICATION_FAILED, requestId=request-id)"
	if got != want {
		t.Fatalf("formatError() = %q, want %q", got, want)
	}
}
