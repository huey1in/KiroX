package email

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type outlookGraphBody struct {
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

type outlookGraphMessage struct {
	Subject          string           `json:"subject"`
	BodyPreview      string           `json:"bodyPreview"`
	Body             outlookGraphBody `json:"body"`
	ReceivedDateTime string           `json:"receivedDateTime"`
}

func (m outlookGraphMessage) searchText() string {
	return strings.Join([]string{m.BodyPreview, m.Subject, m.Body.Content}, "\n")
}

type outlookGraphMessagesResponse struct {
	Value []outlookGraphMessage `json:"value"`
}

type outlookGraphFolderResponse struct {
	TotalItemCount int `json:"totalItemCount"`
}

func refreshOutlookGraphToken(acc OutlookAccount, proxyURL string) (string, error) {
	form := url.Values{
		"client_id":     {acc.ClientID},
		"refresh_token": {acc.RefreshToken},
		"grant_type":    {"refresh_token"},
		"scope":         {"https://graph.microsoft.com/Mail.Read offline_access"},
	}

	tryPost := func(p string) (*http.Response, error) {
		client := httpClientWithProxy(p, 30*time.Second)
		return client.Post(
			"https://login.microsoftonline.com/common/oauth2/v2.0/token",
			"application/x-www-form-urlencoded",
			strings.NewReader(form.Encode()),
		)
	}

	resp, err := tryPost(proxyURL)
	if err != nil {
		return "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("刷新失败 %d: %s", resp.StatusCode, string(body[:min(300, len(body))]))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	token, _ := result["access_token"].(string)
	if token == "" {
		return "", fmt.Errorf("响应中无 access_token")
	}
	return token, nil
}

func outlookGraphGet(accessToken, path, proxyURL string, out interface{}) error {
	client := httpClientWithProxy(proxyURL, 30*time.Second)
	req, err := http.NewRequest("GET", "https://graph.microsoft.com/v1.0"+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Prefer", `outlook.body-content-type="text"`)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Graph 请求失败 %d: %s", resp.StatusCode, string(body[:min(300, len(body))]))
	}
	return json.Unmarshal(body, out)
}

func getInboxCountGraph(acc OutlookAccount, proxyURL string) (int, error) {
	accessToken, err := refreshOutlookGraphToken(acc, proxyURL)
	if err != nil {
		return 0, fmt.Errorf("刷新 Graph Token 失败: %v", err)
	}
	return getInboxCountGraphWithToken(accessToken, proxyURL)
}

func getInboxCountGraphWithToken(accessToken, proxyURL string) (int, error) {
	var folder outlookGraphFolderResponse
	if err := outlookGraphGet(accessToken, "/me/mailFolders/inbox?$select=totalItemCount", proxyURL, &folder); err != nil {
		return 0, err
	}
	return folder.TotalItemCount, nil
}

func waitForOTPGraph(acc OutlookAccount, beforeCount, timeout, interval int, codeRegex *regexp.Regexp, proxyURL string) (string, error) {
	accessToken, err := refreshOutlookGraphToken(acc, proxyURL)
	if err != nil {
		return "", fmt.Errorf("刷新 Graph Token 失败: %v", err)
	}

	maxRetries := timeout / interval
	for attempt := 1; attempt <= maxRetries; attempt++ {
		total, err := getInboxCountGraphWithToken(accessToken, proxyURL)
		if err != nil {
			return "", err
		}
		if total <= beforeCount {
			time.Sleep(time.Duration(interval) * time.Second)
			continue
		}

		limit := total - beforeCount
		if limit < 1 {
			limit = 1
		}
		if limit > 10 {
			limit = 10
		}
		path := fmt.Sprintf("/me/mailFolders/inbox/messages?$top=%d&$orderby=receivedDateTime%%20desc&$select=subject,bodyPreview,body,receivedDateTime", limit)
		var messages outlookGraphMessagesResponse
		if err := outlookGraphGet(accessToken, path, proxyURL, &messages); err != nil {
			return "", err
		}
		for _, msg := range messages.Value {
			if code := extractCodeFromText(msg.searchText(), codeRegex); code != "" {
				return code, nil
			}
		}

		time.Sleep(time.Duration(interval) * time.Second)
	}
	return "", fmt.Errorf("等待验证码超时 (%ds)", timeout)
}
