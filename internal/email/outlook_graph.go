package email

import (
	"context"
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
	Content string `json:"content"`
}

type outlookGraphMessage struct {
	Subject     string           `json:"subject"`
	BodyPreview string           `json:"bodyPreview"`
	Body        outlookGraphBody `json:"body"`
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

func refreshOutlookGraphToken(ctx context.Context, acc OutlookAccount, proxyURL string) (string, error) {
	return refreshOutlookOAuthToken(ctx, acc, proxyURL, "/common/oauth2/v2.0/token", "https://graph.microsoft.com/Mail.Read offline_access")
}

func outlookGraphGet(ctx context.Context, accessToken, path, proxyURL string, out interface{}) error {
	client := httpClientWithProxy(proxyURL, 30*time.Second)
	defer client.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, "GET", outlookGraphAPIBaseURL+path, nil)
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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Graph 请求失败 %d: %s", resp.StatusCode, string(body[:min(300, len(body))]))
	}
	return json.Unmarshal(body, out)
}

func getGraphFolderCountWithToken(ctx context.Context, accessToken, folderID, proxyURL string) (int, error) {
	var folder outlookGraphFolderResponse
	path := fmt.Sprintf("/me/mailFolders/%s?$select=totalItemCount", url.PathEscape(folderID))
	if err := outlookGraphGet(ctx, accessToken, path, proxyURL, &folder); err != nil {
		return 0, err
	}
	return folder.TotalItemCount, nil
}

func getMailboxCountsGraph(ctx context.Context, acc OutlookAccount, proxyURL string) (OutlookMailboxCounts, error) {
	accessToken, err := refreshOutlookGraphToken(ctx, acc, proxyURL)
	if err != nil {
		return OutlookMailboxCounts{}, fmt.Errorf("刷新 Graph Token 失败: %w", err)
	}
	counts := OutlookMailboxCounts{Junk: -1}
	counts.Inbox, err = getGraphFolderCountWithToken(ctx, accessToken, "inbox", proxyURL)
	if err != nil {
		return counts, err
	}
	counts.Junk, err = getGraphFolderCountWithToken(ctx, accessToken, "junkemail", proxyURL)
	if err != nil {
		if ctx.Err() != nil {
			return counts, ctx.Err()
		}
		log.Printf("[Outlook Graph] 无法读取垃圾邮件目录，继续仅监控收件箱: %v", err)
		counts.Junk = -1
	}
	return counts, nil
}

func findOTPGraphWithToken(ctx context.Context, accessToken string, counts OutlookMailboxCounts, codeRegex *regexp.Regexp, proxyURL string) (string, error) {
	for _, folder := range outlookGraphFolders(counts) {
		total, err := getGraphFolderCountWithToken(ctx, accessToken, folder.id, proxyURL)
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
		if err := outlookGraphGet(ctx, accessToken, path, proxyURL, &messages); err != nil {
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

func waitForOTPGraph(ctx context.Context, acc OutlookAccount, counts OutlookMailboxCounts, timeout, interval int, codeRegex *regexp.Regexp, proxyURL string) (string, error) {
	accessToken, err := refreshOutlookGraphToken(ctx, acc, proxyURL)
	if err != nil {
		return "", fmt.Errorf("刷新 Graph Token 失败: %w", err)
	}

	if interval <= 0 {
		interval = 3
	}
	maxRetries := timeout / interval
	for attempt := 1; attempt <= maxRetries; attempt++ {
		code, err := findOTPGraphWithToken(ctx, accessToken, counts, codeRegex, proxyURL)
		if err != nil {
			return "", err
		}
		if code != "" {
			return code, nil
		}

		if err := waitEmailPoll(ctx, time.Duration(interval)*time.Second); err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("等待验证码超时 (%ds)", timeout)
}
