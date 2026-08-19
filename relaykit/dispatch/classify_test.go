package dispatch

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/qianfree/team-api/relaykit/types"
)

func TestClassify_状态码全表(t *testing.T) {
	tests := []struct {
		code int
		want ErrorClass
	}{
		{400, ErrClassClient},
		{401, ErrClassCredential},
		{402, ErrClassChannelFatal},
		{403, ErrClassCredential},
		{404, ErrClassModelFatal}, // 模型不存在：单模型信号，喂模型级熔断
		{408, ErrClassTimeout},
		{409, ErrClassClient},
		{413, ErrClassClient},
		{422, ErrClassClient},
		{429, ErrClassRateLimit},
		{500, ErrClassTransient},
		{502, ErrClassTransient},
		{503, ErrClassTransient},
		{504, ErrClassTimeout}, // 504 不再归 TRANSIENT
		{520, ErrClassTransient},
		{527, ErrClassTransient},
		{599, ErrClassTransient},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.code), func(t *testing.T) {
			assert.Equal(t, tt.want, Classify(tt.code, nil, DeliveryResponseReceived))
		})
	}
}

func TestClassify_NewAPIError错误码(t *testing.T) {
	tests := []struct {
		name string
		code types.ErrorCode
		want ErrorClass
	}{
		{"invalid_key 归凭证", types.ErrorCodeChannelInvalidKey, ErrClassCredential},
		{"no_available_key 归渠道级", types.ErrorCodeChannelNoAvailableKey, ErrClassChannelFatal},
		{"protocol_mismatch 归渠道级（驱动换渠道）", types.ErrorCodeChannelProtocolMismatch, ErrClassChannelFatal},
		{"model_not_found 归模型级", types.ErrorCodeModelNotFound, ErrClassModelFatal},
		{"model_mapped_error 归模型级", types.ErrorCodeChannelModelMappedError, ErrClassModelFatal},
		{"response_time_exceeded 归超时", types.ErrorCodeChannelResponseTimeExceeded, ErrClassTimeout},
		{"aws_client_error 归渠道级", types.ErrorCodeChannelAwsClientError, ErrClassChannelFatal},
		{"prompt_blocked 归客户端", types.ErrorCodePromptBlocked, ErrClassClient},
		{"insufficient_user_quota 归客户端", types.ErrorCodeInsufficientUserQuota, ErrClassClient},
		{"convert_request_failed 归客户端", types.ErrorCodeConvertRequestFailed, ErrClassClient},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := types.NewError(errors.New("x"), tt.code)
			assert.Equal(t, tt.want, Classify(0, e, DeliveryResponseReceived))
		})
	}
}

func TestClassify_SkipRetry语义(t *testing.T) {
	// SkipRetry 标记优先于一切：即使状态码本可重试也归客户端类（不重试）
	e := types.NewErrorWithStatusCode(errors.New("x"), types.ErrorCodeBadResponse, 502, types.ErrOptionWithSkipRetry())
	assert.Equal(t, ErrClassClient, Classify(502, e, DeliveryResponseReceived))
}

func TestClassify_NewAPIError状态码兜底(t *testing.T) {
	// 错误码不能定类时用其携带的状态码
	e := types.NewErrorWithStatusCode(errors.New("x"), types.ErrorCodeBadResponse, 429)
	assert.Equal(t, ErrClassRateLimit, Classify(0, e, DeliveryResponseReceived))
}

func TestClassify_上下文错误(t *testing.T) {
	assert.Equal(t, ErrClassTimeout, Classify(0, context.DeadlineExceeded, DeliveryMaybeSent))
	assert.Equal(t, ErrClassClient, Classify(0, context.Canceled, DeliveryMaybeSent), "客户端取消不重试不计健康")
	wrapped := fmt.Errorf("do request: %w", context.DeadlineExceeded)
	assert.Equal(t, ErrClassTimeout, Classify(0, wrapped, DeliveryNotSent))
}

func TestClassify_纯网络错误(t *testing.T) {
	err := errors.New("connection refused")
	assert.Equal(t, ErrClassTransient, Classify(0, err, DeliveryNotSent))
	assert.Equal(t, ErrClassTransient, Classify(0, err, DeliveryMaybeSent), "MaybeSent 仍归 TRANSIENT，禁止原地由 FSM 硬规则处理")
}

func TestClassify_无错误(t *testing.T) {
	assert.Equal(t, ErrClassNone, Classify(0, nil, DeliveryNotSent))
	assert.Equal(t, ErrClassNone, Classify(200, nil, DeliveryResponseReceived))
}

func TestErrorClass_String(t *testing.T) {
	assert.Equal(t, "credential", ErrClassCredential.String())
	assert.Equal(t, "timeout", ErrClassTimeout.String())
	assert.Equal(t, "model_fatal", ErrClassModelFatal.String())
	assert.Equal(t, "none", ErrClassNone.String())
}
