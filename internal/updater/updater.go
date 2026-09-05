package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	githubReleasesURL    = "https://api.github.com/repos/huey1in/kirox/releases/latest"
	githubReleasePageURL = "https://github.com/huey1in/kirox/releases/latest"
)

type githubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
	HTMLURL     string `json:"html_url"`
}

// GetCurrentVersion returns the version embedded in the application.
func GetCurrentVersion() string {
	return "v1.0.4"
}

func semverGreater(a, b string) bool {
	parse := func(v string) [3]int {
		v = strings.TrimPrefix(v, "v")
		parts := strings.SplitN(v, ".", 3)
		var nums [3]int
		for i, part := range parts {
			if i >= len(nums) {
				break
			}
			fmt.Sscanf(part, "%d", &nums[i])
		}
		return nums
	}

	va, vb := parse(a), parse(b)
	for i := range va {
		if va[i] != vb[i] {
			return va[i] > vb[i]
		}
	}
	return false
}

// CheckUpdate fetches release metadata. Installation is handled by the user
// after opening the release page; the application never replaces its binary.
func CheckUpdate() map[string]interface{} {
	currentVersion := GetCurrentVersion()
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest(http.MethodGet, githubReleasesURL, nil)
	if err != nil {
		return map[string]interface{}{"error": "构建请求失败: " + err.Error()}
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "kirox/"+currentVersion)

	resp, err := client.Do(req)
	if err != nil {
		return map[string]interface{}{"error": "检查更新失败: " + err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return map[string]interface{}{
			"hasUpdate":      false,
			"currentVersion": currentVersion,
			"latestVersion":  currentVersion,
			"releaseURL":     githubReleasePageURL,
			"message":        "暂无发布版本",
		}
	}
	if resp.StatusCode != http.StatusOK {
		return map[string]interface{}{"error": fmt.Sprintf("GitHub API 返回 %d", resp.StatusCode)}
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return map[string]interface{}{"error": "解析响应失败: " + err.Error()}
	}

	latestVersion := release.TagName
	if latestVersion == "" {
		latestVersion = release.Name
	}
	releaseURL := release.HTMLURL
	if releaseURL == "" {
		releaseURL = githubReleasePageURL
	}
	releaseDate := ""
	if len(release.PublishedAt) >= 10 {
		releaseDate = release.PublishedAt[:10]
	}

	return map[string]interface{}{
		"hasUpdate":      latestVersion != "" && semverGreater(latestVersion, currentVersion),
		"currentVersion": currentVersion,
		"latestVersion":  latestVersion,
		"releaseDate":    releaseDate,
		"changelog":      release.Body,
		"releaseURL":     releaseURL,
	}
}
