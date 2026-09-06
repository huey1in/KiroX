package browser

import (
	"encoding/json"
	"testing"
)

func TestResetPerfTimingStartsANewPageContext(t *testing.T) {
	ctx := &FingerprintContext{}
	if got := ctx.GetStartTime(100); got != 100 {
		t.Fatalf("initial start time = %d", got)
	}
	ctx.GetPerfTiming(1000)

	ctx.ResetPerfTiming()

	if got := ctx.GetStartTime(200); got != 200 {
		t.Fatalf("reset start time = %d, want 200", got)
	}
	if ctx.perfTiming != nil {
		t.Fatal("performance timing was not cleared")
	}
}

func TestFingerprintReportsCaptchaAfterChallengeAppears(t *testing.T) {
	identity := fingerprintIdentityFixture("captcha", 1)
	ctx := NewFPContext(identity)
	ctx.PageHasCaptcha = true
	encoded := GenerateFingerprintJSON(identity, "https://signin.example.com/signup", "", ctx, "signup", "PageSubmit", 0, 0, "")
	var fingerprint map[string]interface{}
	if err := json.Unmarshal([]byte(encoded), &fingerprint); err != nil {
		t.Fatal(err)
	}
	token, _ := fingerprint["token"].(map[string]interface{})
	if token["pageHasCaptcha"] != float64(1) {
		t.Fatalf("pageHasCaptcha = %#v", token["pageHasCaptcha"])
	}
}
