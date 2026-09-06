package core

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// ServiceError contains the safe, useful fields from an AWS error response.
// Response bodies are deliberately excluded because they may contain CAPTCHA tokens.
type ServiceError struct {
	Code      string
	Message   string
	RequestID string
	Captcha   bool
}

func (e *ServiceError) Error() string {
	if e == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	if e.Code != "" {
		parts = append(parts, e.Code)
	}
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	if e.RequestID != "" {
		parts = append(parts, "requestId="+e.RequestID)
	}
	if len(parts) == 0 {
		return "服务返回未知错误"
	}
	return strings.Join(parts, ": ")
}

type serviceErrorResponse struct {
	RequestID           string `json:"requestId"`
	StepID              string `json:"stepId"`
	WorkflowStateHandle string `json:"workflowStateHandle"`
	Message             struct {
		Text      string `json:"text"`
		Heading   string `json:"heading"`
		Type      string `json:"type"`
		ErrorCode string `json:"errorCode"`
		RequestID string `json:"requestId"`
	} `json:"message"`
	CaptchaResponse struct {
		CaptchaURL   string `json:"captchaURL"`
		CaptchaCES   string `json:"captchaCES"`
		CaptchaToken string `json:"captchaToken"`
		CaptchaCDN   string `json:"captchaCDN"`
	} `json:"captchaResponse"`
}

type awsWAFChallenge struct {
	RedemptionToken     string
	JSAPIScript         string
	StepID              string
	WorkflowStateHandle string
}

func parseAWSWAFChallenge(body []byte) (awsWAFChallenge, bool) {
	var response serviceErrorResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return awsWAFChallenge{}, false
	}
	challenge := awsWAFChallenge{
		RedemptionToken:     strings.TrimSpace(response.CaptchaResponse.CaptchaToken),
		JSAPIScript:         strings.TrimSpace(response.CaptchaResponse.CaptchaCDN),
		StepID:              strings.TrimSpace(response.StepID),
		WorkflowStateHandle: strings.TrimSpace(response.WorkflowStateHandle),
	}
	return challenge, challenge.RedemptionToken != "" && challenge.JSAPIScript != ""
}

func parseServiceError(body []byte) *ServiceError {
	var response serviceErrorResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil
	}

	requestID := response.Message.RequestID
	if requestID == "" {
		requestID = response.RequestID
	}
	captcha := response.CaptchaResponse.CaptchaURL != "" ||
		response.CaptchaResponse.CaptchaCES != "" ||
		response.CaptchaResponse.CaptchaToken != "" ||
		response.CaptchaResponse.CaptchaCDN != ""
	code := strings.TrimSpace(response.Message.ErrorCode)
	message := repairMojibake(strings.TrimSpace(response.Message.Text))
	if message == "" {
		message = repairMojibake(strings.TrimSpace(response.Message.Heading))
	}
	if code == "" && captcha {
		code = "CAPTCHA_REQUIRED"
	}
	if code == "" && message == "" && !captcha {
		return nil
	}

	return &ServiceError{
		Code:      code,
		Message:   message,
		RequestID: requestID,
		Captcha:   captcha,
	}
}

func unexpectedServiceResponse(context string, body []byte) error {
	if serviceErr := parseServiceError(body); serviceErr != nil {
		return serviceErr
	}
	return fmt.Errorf("%s（响应格式异常）", context)
}

// repairMojibake repairs UTF-8 text that was decoded once as ISO-8859-1.
func repairMojibake(value string) string {
	if value == "" {
		return ""
	}
	bytes := make([]byte, 0, len(value))
	for _, r := range value {
		if r > 0xff {
			return value
		}
		bytes = append(bytes, byte(r))
	}
	if !utf8.Valid(bytes) {
		return value
	}
	decoded := string(bytes)
	if decoded == value {
		return value
	}
	return decoded
}
