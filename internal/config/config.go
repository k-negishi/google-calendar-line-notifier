package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/joho/godotenv"
)

// Config アプリケーション設定構造体
type Config struct {
	// Google Calendar設定
	GoogleCredentials string
	CalendarID        string

	// LINE API設定
	LineChannelAccessToken string
	LineUserID             string

	// その他設定
	LogLevel string

	// AWS関連（本番環境でのみ使用）
	ssmClient *ssm.Client
}

// Load 環境に応じて設定を読み込み
func Load() (*Config, error) {
	// AWS Lambda環境かどうか判定
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		return loadAWSConfig()
	}
	return loadLocalConfig()
}

// loadLocalConfig ローカル開発環境用の設定読み込み
func loadLocalConfig() (*Config, error) {
	// .envファイルを読み込み（存在する場合のみ）
	if err := godotenv.Load(); err != nil {
		// .envファイルが存在しない場合はエラーにしない
		fmt.Printf("Warning: .envファイルが見つかりません: %v\n", err)
	}

	cfg := &Config{
		GoogleCredentials:      getEnvOrDefault("GOOGLE_CREDENTIALS", ""),
		CalendarID:             getEnvOrDefault("CALENDAR_ID", "primary"),
		LineChannelAccessToken: getEnvOrDefault("LINE_CHANNEL_ACCESS_TOKEN", ""),
		LineUserID:             getEnvOrDefault("LINE_USER_ID", ""),
		LogLevel:               getEnvOrDefault("LOG_LEVEL", "INFO"),
	}

	// 必須設定項目の確認
	if cfg.GoogleCredentials == "" {
		return nil, fmt.Errorf("GOOGLE_CREDENTIALS環境変数が設定されていません")
	}
	if cfg.LineChannelAccessToken == "" {
		return nil, fmt.Errorf("LINE_CHANNEL_ACCESS_TOKEN環境変数が設定されていません")
	}
	if cfg.LineUserID == "" {
		return nil, fmt.Errorf("LINE_USER_ID環境変数が設定されていません")
	}

	return cfg, nil
}

// loadAWSConfig AWS Lambda環境用の設定読み込み
func loadAWSConfig() (*Config, error) {
	// AWS設定を初期化
	awsConfig, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("AWS設定の読み込みに失敗しました: %v", err)
	}

	ssmClient := ssm.NewFromConfig(awsConfig)

	cfg := &Config{
		CalendarID: getEnvOrDefault("CALENDAR_ID", "primary"),
		LogLevel:   getEnvOrDefault("LOG_LEVEL", "INFO"),
		ssmClient:  ssmClient,
	}

	// Parameter Storeから機密情報を取得
	if err := cfg.loadFromParameterStore(); err != nil {
		return nil, fmt.Errorf("Parameter Storeからの設定読み込みに失敗しました: %v", err)
	}

	return cfg, nil
}

// loadFromParameterStore Parameter Storeから機密情報を読み込み
func (cfg *Config) loadFromParameterStore() error {
	ctx := context.Background()

	// 環境変数からパラメータ名を取得
	googleCredsParam := getEnvOrDefault("SSM_GOOGLE_CREDS_PARAM", "/google-calendar-line-notifier/google-creds")
	lineTokenParam := getEnvOrDefault("SSM_LINE_TOKEN_PARAM", "/google-calendar-line-notifier/line-channel-access-token")
	lineUserIdParam := getEnvOrDefault("SSM_LINE_USER_ID_PARAM", "/google-calendar-line-notifier/line-user-id")
	calendarIdParam := getEnvOrDefault("SSM_CALENDAR_ID_PARAM", "/google-calendar-line-notifier/calendar-id")

	// Parameter Storeから値を取得
	googleCreds, err := cfg.getParameter(ctx, googleCredsParam, true) // SecureString用にwithDecryption=true
	if err != nil {
		return fmt.Errorf("Google認証情報の取得に失敗しました: %v", err)
	}
	cfg.GoogleCredentials = googleCreds

	lineToken, err := cfg.getParameter(ctx, lineTokenParam, true) // SecureString用にwithDecryption=true
	if err != nil {
		return fmt.Errorf("LINE Channel Access Tokenの取得に失敗しました: %v", err)
	}
	cfg.LineChannelAccessToken = lineToken
	// デバッグ: トークンの最初の10文字のみログ出力
	if len(cfg.LineChannelAccessToken) >= 10 {
		fmt.Printf("LINE Token loaded (first 10 chars): %s...\n", cfg.LineChannelAccessToken[:10])
	}

	// LINE User ID も SecureString として取得するように修正
	lineUserId, err := cfg.getParameter(ctx, lineUserIdParam, true) // SecureString用にwithDecryption=true に変更
	if err != nil {
		return fmt.Errorf("LINE User IDの取得に失敗しました: %v", err)
	}
	cfg.LineUserID = lineUserId
	// デバッグ: User IDの長さと最初の5文字をログ出力（セキュリティのため）
	if len(cfg.LineUserID) >= 5 {
		fmt.Printf("LINE User ID loaded: length=%d, first 5 chars=%s...\n", len(cfg.LineUserID), cfg.LineUserID[:5])
	} else {
		fmt.Printf("LINE User ID loaded: length=%d, value=%s\n", len(cfg.LineUserID), cfg.LineUserID)
	}

	calendarId, err := cfg.getParameter(ctx, calendarIdParam, false) // String型
	if err != nil {
		return fmt.Errorf("Calendar IDの取得に失敗しました: %v", err)
	}
	cfg.CalendarID = calendarId
	fmt.Printf("Calendar ID loaded: %s\n", cfg.CalendarID)

	return nil
}

// getParameter Parameter Storeから指定されたパラメータを取得
func (cfg *Config) getParameter(ctx context.Context, paramName string, withDecryption bool) (string, error) {
	input := &ssm.GetParameterInput{
		Name:           aws.String(paramName),
		WithDecryption: aws.Bool(withDecryption),
	}

	result, err := cfg.ssmClient.GetParameter(ctx, input)
	if err != nil {
		return "", fmt.Errorf("パラメータ %s の取得に失敗しました: %v", paramName, err)
	}

	if result.Parameter == nil || result.Parameter.Value == nil {
		return "", fmt.Errorf("パラメータ %s が空です", paramName)
	}

	value := *result.Parameter.Value
	// 空白文字の削除
	value = strings.TrimSpace(value)

	if value == "" {
		return "", fmt.Errorf("パラメータ %s が空の値です", paramName)
	}

	return value, nil
}

// GetGoogleCredentialsJSON Google認証情報をJSONとして解析
func (cfg *Config) GetGoogleCredentialsJSON() (map[string]interface{}, error) {
	var credentials map[string]interface{}
	if err := json.Unmarshal([]byte(cfg.GoogleCredentials), &credentials); err != nil {
		return nil, fmt.Errorf("Google認証情報のJSON解析に失敗しました: %v", err)
	}
	return credentials, nil
}

// getEnvOrDefault 環境変数を取得し、存在しない場合はデフォルト値を返す
func getEnvOrDefault(key, defaultValue string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return defaultValue
}
