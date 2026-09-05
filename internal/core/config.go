package core

import (
	"math/rand"
	"strings"

	"reg_go/internal/email"
)

// Config 注册配置
type Config struct {
	OIDCBase    string
	SigninBase  string
	ProfileBase string
	ViewBase    string
	PortalBase  string
	DirectoryID string
	StartURL    string

	KiroBase        string
	KiroRedirectURI string

	Password string
	FullName string

	Proxy                string
	Debug                bool
	EmailProxy           string
	OTPTimeout           int
	TelemetryEnabled     bool
	HTTPRetries          int
	FingerprintAlgorithm string
	FingerprintOffsets   []int

	EmailProvider  string
	UseOutlook     bool
	OutlookAccount *email.OutlookAccount

	UseMoeMail      bool
	MoeMailConfig   *email.MoeMailConfig
	MoeMailProvider *email.MoeMailProvider

	UseCloudMail      bool
	CloudMailConfig   *email.CloudMailConfig
	CloudMailProvider *email.CloudMailProvider

	UseMailNest      bool
	MailNestConfig   *email.MailNestConfig
	MailNestProvider *email.MailNestProvider

	UseICloud     bool
	ICloudCSV     string
	ICloudAccount *email.ICloudAccount

	MoEmailBaseURL string
	MoEmailAPIKey  string
}

// NewConfig 创建默认配置
func NewConfig() *Config {
	return &Config{
		OIDCBase:             "https://oidc.us-east-1.amazonaws.com",
		SigninBase:           "https://us-east-1.signin.aws",
		ProfileBase:          "https://profile.aws.amazon.com",
		ViewBase:             "https://view.awsapps.com",
		PortalBase:           "https://portal.sso.us-east-1.amazonaws.com",
		DirectoryID:          "d-9067642ac7",
		StartURL:             "https://view.awsapps.com/start",
		KiroBase:             "https://app.kiro.dev",
		KiroRedirectURI:      "https://app.kiro.dev/signin/oauth",
		Password:             GenPassword(),
		FullName:             "Test User",
		OTPTimeout:           120,
		TelemetryEnabled:     true,
		HTTPRetries:          2,
		FingerprintAlgorithm: "balanced",
		FingerprintOffsets:   []int{0, 0, 0, 15, 100},
	}
}

// GenPassword 生成随机密码
func GenPassword() string {
	upper := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	lower := "abcdefghijklmnopqrstuvwxyz"
	digits := "0123456789"
	special := "!@#$%^&*"

	var b strings.Builder
	for i := 0; i < 3; i++ {
		b.WriteByte(upper[rand.Intn(len(upper))])
	}
	for i := 0; i < 6; i++ {
		b.WriteByte(lower[rand.Intn(len(lower))])
	}
	for i := 0; i < 3; i++ {
		b.WriteByte(digits[rand.Intn(len(digits))])
	}
	for i := 0; i < 2; i++ {
		b.WriteByte(special[rand.Intn(len(special))])
	}
	pw := []byte(b.String())
	rand.Shuffle(len(pw), func(i, j int) { pw[i], pw[j] = pw[j], pw[i] })
	return string(pw)
}
