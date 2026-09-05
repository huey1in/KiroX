package storage

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	persistentLogMu      sync.Mutex
	persistentLogEnabled bool
	persistentLogDays    = 7
	secretLogPattern     = regexp.MustCompile(`(?i)(accessToken|refreshToken|clientSecret|device_code|user_code|regCode|workflowState|code|验证码)(\s*[:=]\s*)([^\s,}]+)`)
)

// ConfigurePersistentLogs updates file logging and removes expired files.
func ConfigurePersistentLogs(enabled bool, retentionDays int) {
	persistentLogMu.Lock()
	defer persistentLogMu.Unlock()
	persistentLogEnabled = enabled
	if retentionDays >= 1 && retentionDays <= 90 {
		persistentLogDays = retentionDays
	}
	pruneLogsLocked(time.Now())
}

// AppendPersistentLog writes a redacted copy. UI logs remain unchanged in memory.
func AppendPersistentLog(message string) {
	persistentLogMu.Lock()
	defer persistentLogMu.Unlock()
	if !persistentLogEnabled {
		return
	}
	_ = os.MkdirAll(GetLogsDir(), 0o755)
	path := filepath.Join(GetLogsDir(), time.Now().Format("2006-01-02")+".log")
	redacted := secretLogPattern.ReplaceAllString(message, "$1$2<redacted>")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_, _ = file.WriteString(redacted)
	_ = file.Close()
}

func pruneLogsLocked(now time.Time) {
	entries, err := os.ReadDir(GetLogsDir())
	if err != nil {
		return
	}
	cutoff := now.AddDate(0, 0, -persistentLogDays)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		info, err := entry.Info()
		if err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(GetLogsDir(), entry.Name()))
		}
	}
}

// ClearLogs removes only KiroX .log files.
func ClearLogs() error {
	persistentLogMu.Lock()
	defer persistentLogMu.Unlock()
	entries, err := os.ReadDir(GetLogsDir())
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".log") {
			if err := os.Remove(filepath.Join(GetLogsDir(), entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
