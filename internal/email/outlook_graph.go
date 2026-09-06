package email

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
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

var outlookGraphAPIBaseURL = "https://graph.microsoft.com/v1.0"

type outlookGraphFolder struct {
	id     string
	label  string
	before int
}

func outlookGraphFolders(counts OutlookMailboxCounts) []outlookGraphFolder {
	folders := []outlookGraphFolder{{id: "inbox", label: "收件箱", before: counts.Inbox}}
	if counts.Junk >= 0 {
		folders = append(folders, outlookGraphFolder{id: "junkemail", label: "垃圾邮件", before: counts.Junk})
	}
	return folders
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
	req, err := http.NewRequest("GET", outlookGraphAPIBaseURL+path, nil)
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
	return getGraphFolderCountWithToken(accessToken, "inbox", proxyURL)
}

func getGraphFolderCountWithToken(accessToken, folderID, proxyURL string) (int, error) {
	var folder outlookGraphFolderResponse
	path := fmt.Sprintf("/me/mailFolders/%s?$select=totalItemCount", url.PathEscape(folderID))
	if err := outlookGraphGet(accessToken, path, proxyURL, &folder); err != nil {
		return 0, err
	}
	return folder.TotalItemCount, nil
}

func getMailboxCountsGraph(acc OutlookAccount, proxyURL string) (OutlookMailboxCounts, error) {
	accessToken, err := refreshOutlookGraphToken(acc, proxyURL)
	if err != nil {
		return OutlookMailboxCounts{}, fmt.Errorf("刷新 Graph Token 失败: %v", err)
	}
	counts := OutlookMailboxCounts{Junk: -1}
	counts.Inbox, err = getGraphFolderCountWithToken(accessToken, "inbox", proxyURL)
	if err != nil {
		return counts, err
	}
	counts.Junk, err = getGraphFolderCountWithToken(accessToken, "junkemail", proxyURL)
	if err != nil {
		log.Printf("[Outlook Graph] 无法读取垃圾邮件目录，继续仅监控收件箱: %v", err)
		counts.Junk = -1
	}
	return counts, nil
}

func findOTPGraphWithToken(accessToken string, counts OutlookMailboxCounts, codeRegex *regexp.Regexp, proxyURL string) (string, error) {
	for _, folder := range outlookGraphFolders(counts) {
		total, err := getGraphFolderCountWithToken(accessToken, folder.id, proxyURL)
		if err != nil {
			return "", err
		}
		if total <= folder.before {
			continue
		}

		limit := total - folder.before
		if limit < 1 {
			limit = 1
		}
		if limit > 10 {
			limit = 10
		}
		path := fmt.Sprintf("/me/mailFolders/%s/messages?$top=%d&$orderby=receivedDateTime%%20desc&$select=subject,bodyPreview,body,receivedDateTime", url.PathEscape(folder.id), limit)
		var messages outlookGraphMessagesResponse
		if err := outlookGraphGet(accessToken, path, proxyURL, &messages); err != nil {
			return "", err
		}
		for _, msg := range messages.Value {
			if code := extractCodeFromText(msg.searchText(), codeRegex); code != "" {
				log.Printf("[Outlook Graph] 从%s获取到验证码", folder.label)
				return code, nil
			}
		}
	}
	return "", nil
}

func waitForOTPGraph(acc OutlookAccount, counts OutlookMailboxCounts, timeout, interval int, codeRegex *regexp.Regexp, proxyURL string) (string, error) {
	accessToken, err := refreshOutlookGraphToken(acc, proxyURL)
	if err != nil {
		return "", fmt.Errorf("刷新 Graph Token 失败: %v", err)
	}

	maxRetries := timeout / interval
	for attempt := 1; attempt <= maxRetries; attempt++ {
		code, err := findOTPGraphWithToken(accessToken, counts, codeRegex, proxyURL)
		if err != nil {
			return "", err
		}
		if code != "" {
			return code, nil
		}

		time.Sleep(time.Duration(interval) * time.Second)
	}
	return "", fmt.Errorf("等待验证码超时 (%ds)", timeout)
}
