package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/url"

	fhttp "github.com/bogdanfinn/fhttp"

	httputil "reg_go/internal/http"
)

// Step14KiroAuthorize Kiro IDE OAuth 授权（authorization_code + PKCE）
func (r *Registrar) Step14KiroAuthorize() (string, error) {
	log.Println("[14] Kiro OIDC 授权")

	// 1. 注册 Kiro IDE 客户端
	regBody, _, err := r.DoPost(r.Cfg.OIDCBase+"/client/register", map[string]interface{}{
		"clientName":   "Kiro IDE",
		"clientType":   "public",
		"scopes":       []string{"codewhisperer:completions", "codewhisperer:analysis", "codewhisperer:conversations", "codewhisperer:transformations", "codewhisperer:taskassist"},
		"redirectUris": []string{"http://127.0.0.1/oauth/callback"},
		"grantTypes":   []string{"authorization_code", "refresh_token"},
		"issuerUrl":    "https://view.awsapps.com/start",
	}, map[string]string{"Content-Type": "application/json"})
	if err != nil {
		return "", err
	}
	var regData map[string]interface{}
	json.Unmarshal(regBody, &regData)
	r.KiroClientID, _ = regData["clientId"].(string)
	r.KiroClientSecret, _ = regData["clientSecret"].(string)
	if r.KiroClientID == "" {
		return "", fmt.Errorf("Kiro client/register 失败: %s", string(regBody))
	}

	// 2. 生成 PKCE + state + 随机端口
	verifier, challenge := httputil.PKCE()
	r.KiroCodeVerifier = verifier
	r.KiroState = NewUUID()
	r.KiroRedirectPort = 49152 + rand.Intn(16384)
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/oauth/callback", r.KiroRedirectPort)

	// 3. /authorize -> 302 (Location 含 orchestrator_id)
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", r.KiroClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scopes", "codewhisperer:completions,codewhisperer:analysis,codewhisperer:conversations,codewhisperer:transformations,codewhisperer:taskassist")
	params.Set("state", r.KiroState)
	params.Set("code_challenge", challenge)
	params.Set("code_challenge_method", "S256")
	authURL := r.Cfg.OIDCBase + "/authorize?" + params.Encode()

	noRedirect := httputil.NewNoRedirectTLSClient(r.Cfg.Proxy, r.Identity.ChromeVer)
	defer noRedirect.CloseIdleConnections()
	navHeaders := map[string]string{
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"User-Agent":                r.Identity.UA,
		"sec-ch-ua":                 r.Identity.SecUA,
		"sec-ch-ua-mobile":          "?0",
		"sec-ch-ua-platform":        `"Windows"`,
		"sec-fetch-dest":            "document",
		"sec-fetch-mode":            "navigate",
		"sec-fetch-site":            "none",
		"sec-fetch-user":            "?1",
		"upgrade-insecure-requests": "1",
	}
	req1, err := fhttp.NewRequestWithContext(r.context(), "GET", authURL, nil)
	if err != nil {
		return "", err
	}
	httputil.SetHeaders(req1, navHeaders)
	resp1, err := noRedirect.Do(req1)
	if err != nil {
		return "", err
	}
	resp1.Body.Close()
	if resp1.StatusCode != 302 {
		return "", fmt.Errorf("authorize 非 302: %d", resp1.StatusCode)
	}
	loc1 := resp1.Header.Get("Location")
	orchID := httputil.ExtractParam(loc1, "orchestrator_id")
	if orchID == "" {
		snippet := loc1
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		return "", fmt.Errorf("无 orchestrator_id: %s", snippet)
	}

	// 4. POST /authentication_result -> 拿到第一段 resume URL
	authResultBody, _ := json.Marshal(map[string]string{"orchestrator_id": orchID})
	req2, err := fhttp.NewRequestWithContext(r.context(), "POST", r.Cfg.OIDCBase+"/authentication_result", bytes.NewReader(authResultBody))
	if err != nil {
		return "", err
	}
	httputil.SetHeaders(req2, map[string]string{
		"Accept":                 "application/json, text/plain, */*",
		"Content-Type":           "application/json",
		"User-Agent":             r.Identity.UA,
		"Origin":                 "https://view.awsapps.com",
		"Referer":                "https://view.awsapps.com/",
		"x-amz-sso_bearer_token": r.SSOToken,
		"x-amz-sso-bearer-token": r.SSOToken,
		"sec-ch-ua":              r.Identity.SecUA,
		"sec-ch-ua-mobile":       "?0",
		"sec-ch-ua-platform":     `"Windows"`,
		"sec-fetch-dest":         "empty",
		"sec-fetch-mode":         "cors",
		"sec-fetch-site":         "cross-site",
	})
	client := httputil.NewTLSClient(r.Cfg.Proxy, true, r.Identity.ChromeVer)
	defer client.CloseIdleConnections()
	resp2, err := client.Do(req2)
	if err != nil {
		return "", err
	}
	body2, err := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if err != nil {
		return "", err
	}
	if resp2.StatusCode != 200 {
		return "", fmt.Errorf("authentication_result 失败 %d: %s", resp2.StatusCode, string(body2))
	}
	var rd2 map[string]interface{}
	json.Unmarshal(body2, &rd2)
	resumeURL1, _ := rd2["location"].(string)
	if resumeURL1 == "" {
		return "", fmt.Errorf("authentication_result 无 location")
	}

	// 5. GET resumeURL1（不跟随）-> Location 含 authorizationResumptionContext
	req3, err := fhttp.NewRequestWithContext(r.context(), "GET", resumeURL1, nil)
	if err != nil {
		return "", err
	}
	httputil.SetHeaders(req3, navHeaders)
	resp3, err := noRedirect.Do(req3)
	if err != nil {
		return "", err
	}
	resp3.Body.Close()
	if resp3.StatusCode != 302 {
		return "", fmt.Errorf("resume1 非 302: %d", resp3.StatusCode)
	}
	loc3 := resp3.Header.Get("Location")
	ctx := httputil.ExtractParam(loc3, "authorizationResumptionContext")
	if ctx == "" {
		return "", fmt.Errorf("无 authorizationResumptionContext: %s", loc3)
	}

	// 6. POST /device_authorization/associate_token -> 消费同意，拿第二段 resume URL
	consentBody, _ := json.Marshal(map[string]string{
		"authorizationResumptionContext": ctx,
		"userSessionId":                  r.SSOToken,
	})
	req4, err := fhttp.NewRequestWithContext(r.context(), "POST", r.Cfg.OIDCBase+"/device_authorization/associate_token", bytes.NewReader(consentBody))
	if err != nil {
		return "", err
	}
	httputil.SetHeaders(req4, map[string]string{
		"Accept":             "application/json, text/plain, */*",
		"Content-Type":       "application/json",
		"User-Agent":         r.Identity.UA,
		"Origin":             "https://view.awsapps.com",
		"Referer":            "https://view.awsapps.com/",
		"sec-ch-ua":          r.Identity.SecUA,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": `"Windows"`,
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "cross-site",
	})
	resp4, err := client.Do(req4)
	if err != nil {
		return "", err
	}
	body4, err := io.ReadAll(resp4.Body)
	resp4.Body.Close()
	if err != nil {
		return "", err
	}
	if resp4.StatusCode != 200 {
		return "", fmt.Errorf("associate_token 失败 %d: %s", resp4.StatusCode, string(body4))
	}
	var rd4 map[string]interface{}
	json.Unmarshal(body4, &rd4)
	resumeURL2, _ := rd4["location"].(string)
	if resumeURL2 == "" {
		return "", fmt.Errorf("associate_token 无 location: %s", string(body4))
	}

	// 7. GET resumeURL2（不跟随）-> Location 含 code
	req5, err := fhttp.NewRequestWithContext(r.context(), "GET", resumeURL2, nil)
	if err != nil {
		return "", err
	}
	httputil.SetHeaders(req5, navHeaders)
	resp5, err := noRedirect.Do(req5)
	if err != nil {
		return "", err
	}
	resp5.Body.Close()
	if resp5.StatusCode != 302 {
		return "", fmt.Errorf("resume2 非 302: %d", resp5.StatusCode)
	}
	loc5 := resp5.Header.Get("Location")
	code := httputil.ExtractParam(loc5, "code")
	if code == "" {
		return "", fmt.Errorf("Kiro callback 无 code: %s", loc5)
	}
	if st := httputil.ExtractParam(loc5, "state"); st != "" {
		r.KiroState = st
	}
	if len(code) > 30 {
		log.Printf("code=%s...", code[:30])
	}
	return code, nil
}
