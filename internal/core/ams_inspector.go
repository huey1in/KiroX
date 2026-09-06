package core

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"

	httputil "reg_go/internal/http"
)

const amsInspectDirEnv = "KIROX_AMS_INSPECT_DIR"
const amsTraceEnv = "KIROX_AMS_TRACE"

type amsScriptReport struct {
	CapturedAt string            `json:"capturedAt"`
	Host       string            `json:"host"`
	Path       string            `json:"path"`
	SHA256     string            `json:"sha256"`
	Bytes      int               `json:"bytes"`
	TokenMeta  map[string]string `json:"tokenMeta,omitempty"`
	Challenge  amsChallengeInfo  `json:"challenge"`
	Markers    map[string]bool   `json:"markers"`
}

type amsChallengeInfo struct {
	Type                string `json:"type"`
	SessionIDPresent    bool   `json:"sessionIdPresent"`
	MetadataPresent     bool   `json:"metadataPresent"`
	WAFScriptHost       string `json:"wafScriptHost,omitempty"`
	WAFScriptPath       string `json:"wafScriptPath,omitempty"`
	WAFAPIKeyPresent    bool   `json:"wafApiKeyPresent,omitempty"`
	WAFClientIDPresent  bool   `json:"wafClientIdPresent,omitempty"`
	WAFChallengeVariant string `json:"wafChallengeVariant,omitempty"`
}

func amsInspectionEnabled() bool {
	return strings.TrimSpace(os.Getenv(amsInspectDirEnv)) != ""
}

func amsTraceEnabled() bool {
	return strings.TrimSpace(os.Getenv(amsTraceEnv)) == "1"
}

func safeCookieNames(cookies map[string]string) []string {
	names := make([]string, 0, len(cookies))
	for name := range cookies {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *Registrar) safeClientCookieNames(rawURL string) []string {
	parsed, err := url.Parse(rawURL)
	if err != nil || r.Client == nil {
		return nil
	}
	cookies := r.Client.GetCookies(parsed)
	names := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		sum := sha256.Sum256([]byte(cookie.Value))
		domain := cookie.Domain
		if domain == "" {
			domain = "host-only"
		}
		path := cookie.Path
		if path == "" {
			path = "/"
		}
		names = append(names, fmt.Sprintf("%s(domain=%s,path=%s,len=%d,hash=%x)", cookie.Name, domain, path, len(cookie.Value), sum[:4]))
	}
	return names
}

func safeJSONShape(value interface{}) map[string]interface{} {
	result := map[string]interface{}{}
	payload, ok := value.(map[string]interface{})
	if !ok {
		return result
	}
	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result["keys"] = keys
	if inputs, ok := payload["inputs"].([]interface{}); ok {
		inputShapes := make([]map[string]interface{}, 0, len(inputs))
		for _, rawInput := range inputs {
			input, _ := rawInput.(map[string]interface{})
			if input == nil {
				if stringInput, ok := rawInput.(map[string]string); ok {
					input = make(map[string]interface{}, len(stringInput))
					for key, value := range stringInput {
						input[key] = value
					}
				}
			}
			shape := map[string]interface{}{}
			fieldNames := make([]string, 0, len(input))
			for key := range input {
				fieldNames = append(fieldNames, key)
			}
			sort.Strings(fieldNames)
			shape["fields"] = fieldNames
			shape["type"], _ = input["input_type"].(string)
			if password, ok := input["password"].(string); ok {
				shape["passwordLength"] = len(password)
			}
			inputShapes = append(inputShapes, shape)
		}
		result["inputs"] = inputShapes
	}
	return result
}

func safeJSONResponseKeys(body []byte) []string {
	var payload map[string]interface{}
	if json.Unmarshal(body, &payload) != nil {
		return nil
	}
	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func decodedModulusBits(value string) int {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return 0
	}
	return len(decoded) * 8
}

func safeResponseRoute(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "invalid-url"
	}
	return parsed.Hostname() + parsed.EscapedPath()
}

func safeSetCookieScopes(headers map[string][]string) []string {
	var result []string
	for headerName, values := range headers {
		if !strings.EqualFold(headerName, "Set-Cookie") {
			continue
		}
		for _, raw := range values {
			parts := strings.Split(raw, ";")
			nameValue := strings.SplitN(strings.TrimSpace(parts[0]), "=", 2)
			if len(nameValue) != 2 || nameValue[0] == "" {
				continue
			}
			domain := "host-only"
			path := "/"
			for _, attribute := range parts[1:] {
				attributeParts := strings.SplitN(strings.TrimSpace(attribute), "=", 2)
				if len(attributeParts) != 2 {
					continue
				}
				switch strings.ToLower(attributeParts[0]) {
				case "domain":
					domain = strings.ToLower(attributeParts[1])
				case "path":
					path = attributeParts[1]
				}
			}
			result = append(result, nameValue[0]+"(domain="+domain+",path="+path+")")
		}
	}
	sort.Strings(result)
	return result
}

func (r *Registrar) captureAMSScript(rawURL, redemptionToken, referer string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return "", fmt.Errorf("AMS captchaCDN 不是有效的 HTTPS URL")
	}

	headers := map[string]string{
		"Accept":             "*/*",
		"Accept-Language":    "zh-CN,zh;q=0.9,en;q=0.8",
		"User-Agent":         r.Identity.UA,
		"Referer":            referer,
		"sec-ch-ua":          r.Identity.SecUA,
		"sec-ch-ua-mobile":   "?0",
		"sec-ch-ua-platform": `"Windows"`,
		"sec-fetch-dest":     "script",
		"sec-fetch-mode":     "no-cors",
		"sec-fetch-site":     "cross-site",
	}
	req, err := http.NewRequestWithContext(r.context(), http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", fmt.Errorf("创建 AMS SDK 请求失败: %w", err)
	}
	httputil.SetHeaders(req, headers)
	resp, err := r.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载 AMS SDK 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("下载 AMS SDK 失败: HTTP %d", resp.StatusCode)
	}
	const maxScriptBytes = 8 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxScriptBytes+1))
	if err != nil {
		return "", fmt.Errorf("读取 AMS SDK 失败: %w", err)
	}
	if len(data) > maxScriptBytes {
		return "", fmt.Errorf("AMS SDK 超过 %d MiB 限制", maxScriptBytes>>20)
	}

	dir := filepath.Clean(strings.TrimSpace(os.Getenv(amsInspectDirEnv)))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("创建 AMS 诊断目录失败: %w", err)
	}
	sum := sha256.Sum256(data)
	markers := make(map[string]bool)
	for _, marker := range []string{
		"initAMS", "AwsWafCaptcha", "redemptionToken", "onChallengeComplete",
		"accessCode", "apiKey", "renderCaptcha", "captcha_voucher", "websiteKey",
		"challengeScript", "captchaScript",
	} {
		markers[marker] = strings.Contains(string(data), marker)
	}
	_, challenge, err := r.loadAMSChallenge(redemptionToken, referer)
	if err != nil {
		return "", fmt.Errorf("读取 AMS challenge 失败: %w", err)
	}
	challengeInfo := amsChallengeInfo{
		Type:             challenge.ChallengeType,
		SessionIDPresent: strings.TrimSpace(challenge.SessionID) != "",
		MetadataPresent:  strings.TrimSpace(challenge.ChallengeMetadata) != "",
	}
	if strings.EqualFold(challenge.ChallengeType, "WAF_GRID") {
		var metadata amsWAFGridMetadata
		if err := decodeBase64JSON(challenge.ChallengeMetadata, &metadata); err == nil {
			if scriptURL, err := url.Parse(metadata.ScriptURL); err == nil {
				challengeInfo.WAFScriptHost = scriptURL.Hostname()
				challengeInfo.WAFScriptPath = scriptURL.EscapedPath()
			}
			challengeInfo.WAFAPIKeyPresent = metadata.APIKey != ""
			challengeInfo.WAFClientIDPresent = metadata.ClientID != ""
			challengeInfo.WAFChallengeVariant = metadata.ChallengeVariant
		}
	}
	report := amsScriptReport{
		CapturedAt: time.Now().UTC().Format(time.RFC3339),
		Host:       parsed.Hostname(),
		Path:       parsed.EscapedPath(),
		SHA256:     hex.EncodeToString(sum[:]),
		Bytes:      len(data),
		TokenMeta:  safeAMSTokenMetadata(redemptionToken),
		Challenge:  challengeInfo,
		Markers:    markers,
	}
	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("生成 AMS 诊断报告失败: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ams-sdk.js"), data, 0o600); err != nil {
		return "", fmt.Errorf("保存 AMS SDK 失败: %w", err)
	}
	reportPath := filepath.Join(dir, "report.json")
	if err := os.WriteFile(reportPath, reportJSON, 0o600); err != nil {
		return "", fmt.Errorf("保存 AMS 诊断报告失败: %w", err)
	}
	return reportPath, nil
}

func safeAMSTokenMetadata(token string) map[string]string {
	payload, err := decodeAMSToken(token)
	if err != nil {
		return nil
	}
	metadata := make(map[string]string)
	if payload.EnvStage != "" {
		metadata["envStage"] = payload.EnvStage
	}
	if payload.Region != "" {
		metadata["region"] = payload.Region
	}
	metadata["apiRootHost"] = payload.APIRoot
	return metadata
}
