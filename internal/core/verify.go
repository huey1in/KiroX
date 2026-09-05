package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	fhttp "github.com/bogdanfinn/fhttp"
	httputil "reg_go/internal/http"
)

// VerifyAlive 验活: 刷新 Token + 查用量 + 查模型
func (r *Registrar) VerifyAlive(awsToken map[string]interface{}) map[string]interface{} {
	log.Println("[验活] 刷新 Token + 查用量 + 查模型")
	client := httputil.NewTLSClient(r.Cfg.Proxy, true, r.Identity.ChromeVer)
	defer client.CloseIdleConnections()

	refreshToken, _ := awsToken["refreshToken"].(string)

	tokenBody, _ := json.Marshal(map[string]string{
		"clientId":     r.ClientID,
		"clientSecret": r.ClientSecret,
		"refreshToken": refreshToken,
		"grantType":    "refresh_token",
	})
	req, err := fhttp.NewRequestWithContext(r.context(), "POST", "https://oidc.us-east-1.amazonaws.com/token",
		bytes.NewReader(tokenBody))
	if err != nil {
		return map[string]interface{}{"alive": false, "error": err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("验活异常: %v", err)
		return map[string]interface{}{"alive": false, "error": err.Error()}
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return map[string]interface{}{"alive": false, "error": err.Error()}
	}

	if resp.StatusCode != 200 {
		log.Printf("Token 刷新失败: %d", resp.StatusCode)
		return map[string]interface{}{"alive": false, "error": fmt.Sprintf("refresh failed: %d", resp.StatusCode)}
	}

	var tok map[string]interface{}
	json.Unmarshal(body, &tok)
	access, _ := tok["accessToken"].(string)
	expiresIn, _ := tok["expiresIn"].(float64)
	log.Printf("Token 刷新成功, expiresIn=%ds", int(expiresIn))

	usageURL := "https://q.us-east-1.amazonaws.com/getUsageLimits?origin=AI_EDITOR&resourceType=AGENTIC_REQUEST&isEmailRequired=true"
	usageRes := queryGetEndpoint(r.context(), client, access, usageURL)
	if usageRes.suspended {
		return map[string]interface{}{"alive": false, "suspended": true, "error": "suspended"}
	}
	if !usageRes.ok {
		return map[string]interface{}{"alive": false, "error": "usage query failed"}
	}

	modelRes := queryGetEndpoint(r.context(), client, access, "https://q.us-east-1.amazonaws.com/ListAvailableModels?origin=AI_EDITOR")
	if modelRes.suspended {
		return map[string]interface{}{"alive": false, "suspended": true, "error": "suspended"}
	}

	kiroRefreshBody, _ := json.Marshal(map[string]string{
		"clientId":     r.ClientID,
		"clientSecret": r.ClientSecret,
		"refreshToken": refreshToken,
		"grantType":    "refresh_token",
	})
	kiroRes := queryPostEndpoint(r.context(), client, "https://prod.us-east-1.auth.desktop.kiro.dev/refreshToken", kiroRefreshBody)
	if kiroRes.suspended {
		return map[string]interface{}{"alive": false, "suspended": true, "error": "suspended"}
	}

	return r.parseUsage(usageRes.body)
}

type endpointResult struct {
	body      []byte
	ok        bool
	suspended bool
}

func checkEndpointResponse(url string, statusCode int, body []byte) endpointResult {
	label := endpointLabel(url)
	if statusCode == 403 {
		log.Printf("账号已被封禁 (403) [%s]", label)
		return endpointResult{suspended: true}
	}
	if statusCode != 200 {
		log.Printf("端点查询失败 [%s]: %d", label, statusCode)
		return endpointResult{}
	}
	return endpointResult{body: body, ok: true}
}

// endpointLabel 把完整 URL 归一到简短标签，避免日志泄露后端。
func endpointLabel(url string) string {
	switch {
	case strings.Contains(url, "getUsageLimits"):
		return "usage"
	case strings.Contains(url, "ListAvailableModels"):
		return "models"
	case strings.Contains(url, "refreshToken"):
		return "kiro-refresh"
	case strings.Contains(url, "/token"):
		return "oidc-token"
	default:
		return "endpoint"
	}
}

func queryGetEndpoint(ctx context.Context, client interface {
	Do(req *fhttp.Request) (*fhttp.Response, error)
}, access, url string) endpointResult {
	req, err := fhttp.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return endpointResult{}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("User-Agent", "aws-sdk-js/1.0.18 ua/2.1 os/windows lang/js md/nodejs#20.16.0 api/codewhispererstreaming#1.0.18 m/E KiroIDE-0.6.18")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("端点查询异常 [%s]: %s", endpointLabel(url), scrubURLs(err.Error()))
		return endpointResult{}
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return endpointResult{}
	}
	return checkEndpointResponse(url, resp.StatusCode, body)
}

func queryPostEndpoint(ctx context.Context, client interface {
	Do(req *fhttp.Request) (*fhttp.Response, error)
}, url string, payload []byte) endpointResult {
	req, err := fhttp.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return endpointResult{}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("端点查询异常 [%s]: %s", endpointLabel(url), scrubURLs(err.Error()))
		return endpointResult{}
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return endpointResult{}
	}
	return checkEndpointResponse(url, resp.StatusCode, body)
}

func (r *Registrar) parseUsage(body []byte) map[string]interface{} {
	var usage map[string]interface{}
	json.Unmarshal(body, &usage)

	userInfo, _ := usage["userInfo"].(map[string]interface{})
	emailAddr, _ := userInfo["email"].(string)

	var totalLimit, totalUsed float64
	if breakdown, ok := usage["usageBreakdownList"].([]interface{}); ok {
		for _, item := range breakdown {
			b, _ := item.(map[string]interface{})
			rt, _ := b["resourceType"].(string)
			dn, _ := b["displayName"].(string)
			if rt == "CREDIT" || dn == "Credits" {
				baseLimit, _ := b["usageLimitWithPrecision"].(float64)
				if baseLimit == 0 {
					baseLimit, _ = b["usageLimit"].(float64)
				}
				baseUsed, _ := b["currentUsageWithPrecision"].(float64)
				if baseUsed == 0 {
					baseUsed, _ = b["currentUsage"].(float64)
				}
				totalLimit = baseLimit
				totalUsed = baseUsed

				if ft, ok := b["freeTrialInfo"].(map[string]interface{}); ok {
					if ftStatus, _ := ft["freeTrialStatus"].(string); ftStatus == "ACTIVE" {
						ftLimit, _ := ft["usageLimitWithPrecision"].(float64)
						ftUsed, _ := ft["currentUsageWithPrecision"].(float64)
						totalLimit += ftLimit
						totalUsed += ftUsed
					}
				}
				break
			}
		}
	}

	log.Printf("验活成功! 邮箱=%s Credit=%.1f/%.1f", emailAddr, totalUsed, totalLimit)
	return map[string]interface{}{
		"alive": true, "email": emailAddr,
		"credit_used": totalUsed, "credit_limit": totalLimit,
	}
}
