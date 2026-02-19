package config

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockSSMClient は SSMParameterGetter のテスト用モック
type MockSSMClient struct {
	mock.Mock
}

func (m *MockSSMClient) GetParameter(ctx context.Context, params *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ssm.GetParameterOutput), args.Error(1)
}

// --- getEnvOrDefault テスト ---

func TestGetEnvOrDefault_WithValue(t *testing.T) {
	t.Setenv("TEST_ENV_KEY", "test-value")
	result := getEnvOrDefault("TEST_ENV_KEY", "default")
	assert.Equal(t, "test-value", result)
}

func TestGetEnvOrDefault_WithDefault(t *testing.T) {
	result := getEnvOrDefault("NONEXISTENT_KEY_FOR_TEST_12345", "default-value")
	assert.Equal(t, "default-value", result)
}

func TestGetEnvOrDefault_TrimsWhitespace(t *testing.T) {
	t.Setenv("TEST_ENV_WHITESPACE", "  trimmed  ")
	result := getEnvOrDefault("TEST_ENV_WHITESPACE", "default")
	assert.Equal(t, "trimmed", result)
}

// --- GetGoogleCredentialsJSON テスト ---

func TestGetGoogleCredentialsJSON_Valid(t *testing.T) {
	cfg := &Config{GoogleCredentials: `{"type": "service_account", "project_id": "test"}`}
	result, err := cfg.GetGoogleCredentialsJSON()
	require.NoError(t, err)
	assert.Equal(t, "service_account", result["type"])
	assert.Equal(t, "test", result["project_id"])
}

func TestGetGoogleCredentialsJSON_Invalid(t *testing.T) {
	cfg := &Config{GoogleCredentials: "not valid json"}
	_, err := cfg.GetGoogleCredentialsJSON()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Google認証情報のJSON解析に失敗しました")
}

// --- loadLocalConfig テスト ---

func TestLoadLocalConfig_MissingRequired(t *testing.T) {
	// 必須環境変数が未設定の状態をシミュレート
	t.Setenv("GOOGLE_CREDENTIALS", "")
	t.Setenv("LINE_CHANNEL_ACCESS_TOKEN", "")
	t.Setenv("LINE_USER_ID", "")

	_, err := loadLocalConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "環境変数が設定されていません")
}

// --- getParameter テスト（モック使用） ---

func TestGetParameter_Success(t *testing.T) {
	mockSSM := new(MockSSMClient)
	cfg := &Config{ssmClient: mockSSM}

	output := &ssm.GetParameterOutput{
		Parameter: &types.Parameter{
			Value: aws.String("test-value"),
		},
	}

	mockSSM.On("GetParameter", mock.Anything, mock.MatchedBy(func(input *ssm.GetParameterInput) bool {
		return *input.Name == "/test/param" && *input.WithDecryption == true
	})).Return(output, nil)

	result, err := cfg.getParameter(context.Background(), "/test/param", true)
	require.NoError(t, err)
	assert.Equal(t, "test-value", result)
	mockSSM.AssertExpectations(t)
}

func TestGetParameter_EmptyValue(t *testing.T) {
	mockSSM := new(MockSSMClient)
	cfg := &Config{ssmClient: mockSSM}

	output := &ssm.GetParameterOutput{
		Parameter: &types.Parameter{
			Value: aws.String(""),
		},
	}

	mockSSM.On("GetParameter", mock.Anything, mock.Anything).Return(output, nil)

	_, err := cfg.getParameter(context.Background(), "/test/param", true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "空の値です")
}

func TestGetParameter_APIError(t *testing.T) {
	mockSSM := new(MockSSMClient)
	cfg := &Config{ssmClient: mockSSM}

	mockSSM.On("GetParameter", mock.Anything, mock.Anything).Return(nil, errors.New("SSM API error"))

	_, err := cfg.getParameter(context.Background(), "/test/param", true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "パラメータ /test/param の取得に失敗しました")
	mockSSM.AssertExpectations(t)
}

func TestLoadFromParameterStore(t *testing.T) {
	mockSSM := new(MockSSMClient)
	cfg := &Config{ssmClient: mockSSM}

	// デフォルトのパラメータ名を使用させるため環境変数をクリア
	t.Setenv("SSM_GOOGLE_CREDS_PARAM", "")
	t.Setenv("SSM_LINE_TOKEN_PARAM", "")
	t.Setenv("SSM_LINE_USER_ID_PARAM", "")
	t.Setenv("SSM_CALENDAR_ID_PARAM", "")

	// 各パラメータの取得を設定
	mockSSM.On("GetParameter", mock.Anything, mock.MatchedBy(func(input *ssm.GetParameterInput) bool {
		return *input.Name == "/google-calendar-line-notifier/google-creds"
	})).Return(&ssm.GetParameterOutput{
		Parameter: &types.Parameter{Value: aws.String(`{"type":"service_account"}`)},
	}, nil)

	mockSSM.On("GetParameter", mock.Anything, mock.MatchedBy(func(input *ssm.GetParameterInput) bool {
		return *input.Name == "/google-calendar-line-notifier/line-channel-access-token"
	})).Return(&ssm.GetParameterOutput{
		Parameter: &types.Parameter{Value: aws.String("line-token-value")},
	}, nil)

	mockSSM.On("GetParameter", mock.Anything, mock.MatchedBy(func(input *ssm.GetParameterInput) bool {
		return *input.Name == "/google-calendar-line-notifier/line-user-id"
	})).Return(&ssm.GetParameterOutput{
		Parameter: &types.Parameter{Value: aws.String("line-user-id-value")},
	}, nil)

	mockSSM.On("GetParameter", mock.Anything, mock.MatchedBy(func(input *ssm.GetParameterInput) bool {
		return *input.Name == "/google-calendar-line-notifier/calendar-id"
	})).Return(&ssm.GetParameterOutput{
		Parameter: &types.Parameter{Value: aws.String("calendar-id-value")},
	}, nil)

	err := cfg.loadFromParameterStore()
	require.NoError(t, err)
	assert.Equal(t, `{"type":"service_account"}`, cfg.GoogleCredentials)
	assert.Equal(t, "line-token-value", cfg.LineChannelAccessToken)
	assert.Equal(t, "line-user-id-value", cfg.LineUserID)
	assert.Equal(t, "calendar-id-value", cfg.CalendarID)
	mockSSM.AssertExpectations(t)
}
