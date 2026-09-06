package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"sort"
	"strings"

	http "github.com/bogdanfinn/fhttp"

	"reg_go/internal/captcha"
	httputil "reg_go/internal/http"
)

type amsTokenPayload struct {
	APIRoot  string `json:"apiRoot"`
	EnvStage string `json:"envStage"`
	Region   string `json:"region"`
}

type amsChallengeResponse struct {
	ChallengeType     string `json:"challengeType"`
	ChallengeMetadata string `json:"challengeMetadata"`
	RedemptionToken   string `json:"redemptionToken"`
	SessionID         string `json:"sessionId"`
}

type amsWAFGridMetadata struct {
	ScriptURL        string `json:"scriptUrl"`
	APIKey           string `json:"apiKey"`
	ClientID         string `json:"clientId"`
	ChallengeVariant string `json:"challengeVariant"`
}

type amsImageResponse struct {
	ImageCDNURL string `json:"imageCdnUrl"`
}

type amsVerifyResponse struct {
	AMCSAccessCode string `json:"amcsAccessCode"`
}

type amsSubmitResponse struct {
	AccessCode string `json:"accessCode"`
}

func decodeBase64JSON(value string, target interface{}) error {
	value = strings.TrimSpace(value)
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(value)
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(decoded, target)
}

func decodeAMSToken(token string) (amsTokenPayload, error) {
	var payload amsTokenPayload
	if err := decodeBase64JSON(token, &payload); err != nil {
		return payload, fmt.Errorf("AMS redemptionToken 格式无效")
	}
	payload.APIRoot = strings.TrimSpace(payload.APIRoot)
	if payload.APIRoot == "" || strings.ContainsAny(payload.APIRoot, "/?#@") {
		return payload, fmt.Errorf("AMS apiRoot 格式无效")
	}
	parsed, err := url.Parse("https://" + payload.APIRoot)
	if err != nil || parsed.Hostname() == "" || !strings.HasSuffix(strings.ToLower(parsed.Hostname()), ".threat-mitigation.aws.amazon.com") {
		return payload, fmt.Errorf("AMS apiRoot 不受信任")
	}
	return payload, nil
}

func (r *Registrar) loadAMSChallenge(redemptionToken, websiteURL string) (amsTokenPayload, amsChallengeResponse, error) {
	var challenge amsChallengeResponse
	payload, err := decodeAMSToken(redemptionToken)
	if err != nil {
		return payload, challenge, err
	}
	status, err := r.postAMS(payload, websiteURL, "/challenge", map[string]string{"redemptionToken": redemptionToken}, &challenge)
	if err != nil {
		return payload, challenge, fmt.Errorf("请求 AMS challenge 失败: %w", err)
	}
	if status < 200 || status >= 300 {
		return payload, challenge, fmt.Errorf("请求 AMS challenge 失败: HTTP %d", status)
	}
	challenge.ChallengeType = strings.TrimSpace(challenge.ChallengeType)
	if challenge.ChallengeType == "" {
		return payload, challenge, fmt.Errorf("AMS challenge 未返回 challengeType")
	}
	return payload, challenge, nil
}

func (r *Registrar) amsHeaders(payload amsTokenPayload, websiteURL string) map[string]string {
	cdnOrigin := "https://cdn." + payload.APIRoot
	websiteOrigin := pageOrigin(websiteURL)
	return map[string]string{
		"Accept":             "*/*",
		"Accept-Language":    "zh-CN,zh;q=0.9,en;q=0.8",
		"Content-Type":       "application/x-www-form-urlencoded",
		"Origin":             cdnOrigin,
		"Referer":            cdnOrigin + "/?origin=" + url.QueryEscape(websiteOrigin) + "&stage=" + url.QueryEscape(payload.EnvStage) + "&use_vr=false",
		"User-Agent":         r.Identity.UA,
		"sec-ch-ua":          r.Identity.SecUA,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": `"Windows"`,
		"sec-fetch-dest":     "empty",
		"sec-fetch-mode":     "cors",
		"sec-fetch-site":     "same-site",
	}
}

func pageOrigin(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.ToLower(strings.TrimSpace(rawURL))
	}
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host)
}

func (r *Registrar) postAMS(payload amsTokenPayload, websiteURL, path string, requestBody, responseTarget interface{}) (int, error) {
	body, err := json.Marshal(requestBody)
	if err != nil {
		return 0, err
	}
	responseBody, status, responseHeaders, err := r.DoPostBodyRaw("https://"+payload.APIRoot+path, string(body), r.amsHeaders(payload, websiteURL))
	if err != nil {
		return 0, err
	}
	if amsTraceEnabled() {
		log.Printf("[WAF TRACE] AMS %s status=%d responseKeys=%v headers=%v", path, status, safeJSONResponseKeys(responseBody), sortedHeaderNames(responseHeaders))
	}
	if len(responseBody) > 0 && responseTarget != nil {
		if err := json.Unmarshal(responseBody, responseTarget); err != nil {
			return status, fmt.Errorf("AMS %s 响应格式无效", strings.TrimPrefix(path, "/"))
		}
	}
	return status, nil
}

func sortedHeaderNames(headers map[string][]string) []string {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, strings.ToLower(name))
	}
	sort.Strings(names)
	return names
}

func (r *Registrar) downloadAMSImage(rawURL, referer string) ([]byte, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return nil, fmt.Errorf("AMS 图片地址无效")
	}
	req, err := http.NewRequestWithContext(r.context(), http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	headers := map[string]string{
		"Accept":           "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8",
		"Accept-Language":  "zh-CN,zh;q=0.9,en;q=0.8",
		"Referer":          referer,
		"User-Agent":       r.Identity.UA,
		"sec-fetch-dest":   "image",
		"sec-fetch-mode":   "no-cors",
		"sec-fetch-site":   "cross-site",
		"sec-ch-ua":        r.Identity.SecUA,
		"sec-ch-ua-mobile": "?0",
	}
	httputil.SetHeaders(req, headers)
	resp, err := r.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("下载 AMS 图片失败: HTTP %d", resp.StatusCode)
	}
	const maxImageBytes = 4 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > maxImageBytes {
		return nil, fmt.Errorf("AMS 图片大小无效")
	}
	return data, nil
}

func (r *Registrar) solveAMCSImage(ctx context.Context, tokenPayload amsTokenPayload, challenge amsChallengeResponse, originalToken, websiteURL string) (string, error) {
	sessionID := strings.TrimSpace(challenge.SessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(challenge.ChallengeMetadata)
	}
	if sessionID == "" {
		return "", fmt.Errorf("AMCS challenge 未返回 sessionId")
	}
	solveImage := r.solveImage
	if solveImage == nil {
		solveImage = captcha.NewClient(r.Cfg.TwoCaptchaAPIKey).SolveImageToText
	}
	referer := "https://cdn." + tokenPayload.APIRoot + "/?origin=" + url.QueryEscape(pageOrigin(websiteURL)) + "&stage=" + url.QueryEscape(tokenPayload.EnvStage) + "&use_vr=false"
	for attempt := 1; attempt <= 3; attempt++ {
		var imageResponse amsImageResponse
		status, err := r.postAMS(tokenPayload, websiteURL, "/amcs/image", map[string]string{"sessionId": sessionID}, &imageResponse)
		if err != nil {
			return "", fmt.Errorf("获取 AMCS 图片失败: %w", err)
		}
		if status == 410 {
			return "", fmt.Errorf("AMCS challenge 已过期")
		}
		if status < 200 || status >= 300 || strings.TrimSpace(imageResponse.ImageCDNURL) == "" {
			return "", fmt.Errorf("获取 AMCS 图片失败: HTTP %d", status)
		}
		image, err := r.downloadAMSImage(imageResponse.ImageCDNURL, referer)
		if err != nil {
			return "", err
		}
		answer, err := solveImage(ctx, image)
		if err != nil {
			return "", err
		}
		var verifyResponse amsVerifyResponse
		status, err = r.postAMS(tokenPayload, websiteURL, "/amcs/verify", map[string]string{
			"guess":     answer,
			"sessionId": sessionID,
		}, &verifyResponse)
		if err != nil {
			return "", fmt.Errorf("验证 AMCS 图片答案失败: %w", err)
		}
		if status >= 500 {
			return "", fmt.Errorf("验证 AMCS 图片答案失败: HTTP %d", status)
		}
		if strings.TrimSpace(verifyResponse.AMCSAccessCode) == "" {
			log.Printf("[WAF] AMCS 图片答案未通过，正在刷新挑战 (%d/3)", attempt)
			continue
		}
		return r.redeemAMSAccessCode(tokenPayload, challenge, originalToken, verifyResponse.AMCSAccessCode, websiteURL)
	}
	return "", fmt.Errorf("AMCS 图片连续 3 次识别失败")
}

func (r *Registrar) solveAMSWAFGrid(ctx context.Context, tokenPayload amsTokenPayload, challenge amsChallengeResponse, originalToken, websiteURL, solverProxy string) (string, error) {
	var metadata amsWAFGridMetadata
	if err := decodeBase64JSON(challenge.ChallengeMetadata, &metadata); err != nil {
		return "", fmt.Errorf("WAF_GRID challengeMetadata 格式无效")
	}
	if strings.TrimSpace(metadata.APIKey) == "" || strings.TrimSpace(metadata.ScriptURL) == "" {
		return "", fmt.Errorf("WAF_GRID 缺少 apiKey 或 scriptUrl")
	}
	scriptURL, err := url.Parse(metadata.ScriptURL)
	if err != nil || scriptURL.Scheme != "https" || scriptURL.Hostname() == "" {
		return "", fmt.Errorf("WAF_GRID scriptUrl 无效")
	}
	solve := r.solveAWSWAF
	if solve == nil {
		solve = captcha.NewClient(r.Cfg.TwoCaptchaAPIKey).SolveAWSWAFResult
	}
	solution, err := solve(ctx, captcha.AWSWAFOptions{
		WebsiteURL:  websiteURL,
		WebsiteKey:  metadata.APIKey,
		JSAPIScript: metadata.ScriptURL,
		Proxy:       solverProxy,
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(solution.CaptchaVoucher) == "" {
		return "", fmt.Errorf("2Captcha 结果未返回 captcha_voucher")
	}
	if strings.TrimSpace(solution.ExistingToken) != "" {
		r.Cookies["aws-waf-token"] = solution.ExistingToken
	}
	return r.redeemAMSAccessCode(tokenPayload, challenge, originalToken, solution.CaptchaVoucher, websiteURL)
}

func (r *Registrar) redeemAMSAccessCode(tokenPayload amsTokenPayload, challenge amsChallengeResponse, originalToken, challengeAccessCode, websiteURL string) (string, error) {
	redemptionToken := strings.TrimSpace(challenge.RedemptionToken)
	if redemptionToken == "" {
		redemptionToken = strings.TrimSpace(originalToken)
	}
	if redemptionToken == "" || strings.TrimSpace(challengeAccessCode) == "" {
		return "", fmt.Errorf("AMS submit 缺少验证数据")
	}
	var submitResponse amsSubmitResponse
	status, err := r.postAMS(tokenPayload, websiteURL, "/submit", map[string]string{
		"redemptionToken":     redemptionToken,
		"challengeAccessCode": challengeAccessCode,
	}, &submitResponse)
	if err != nil {
		return "", fmt.Errorf("提交 AMS challenge 失败: %w", err)
	}
	if status < 200 || status >= 300 {
		return "", fmt.Errorf("提交 AMS challenge 失败: HTTP %d", status)
	}
	accessCode := strings.TrimSpace(submitResponse.AccessCode)
	if accessCode == "" {
		return "", fmt.Errorf("AMS submit 未返回 accessCode")
	}
	return accessCode, nil
}
