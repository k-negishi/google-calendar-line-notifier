package main

import (
	"context"
	"time"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/k-negishi/google-calendar-line-notifier/internal/config"
	"github.com/k-negishi/google-calendar-line-notifier/internal/gateway"
	"github.com/k-negishi/google-calendar-line-notifier/internal/usecase"
)

// LambdaEvent Lambda実行時のイベント構造体
type LambdaEvent struct {
	// EventBridge Schedulerからの実行なので特に使用しない
}

// LambdaResponse Lambda実行結果のレスポンス
type LambdaResponse struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
}

// handler Lambda関数のメインハンドラー
func handler(ctx context.Context, _ LambdaEvent) (LambdaResponse, error) {
	// 設定を読み込み
	cfg, err := config.Load()
	if err != nil {
		return LambdaResponse{
			StatusCode: 500,
			Message:    "設定読み込みエラー",
		}, err
	}

	// 依存性の注入: Google Calendarリポジトリを初期化
	calendarRepo, err := gateway.NewGoogleCalendarRepository([]byte(cfg.GoogleCredentials), cfg.CalendarID)
	if err != nil {
		return LambdaResponse{
			StatusCode: 500,
			Message:    "Google Calendar初期化エラー",
		}, err
	}

	// 依存性の注入: LINE通知クライアントを初期化
	notifier := gateway.NewLINENotifier(cfg.LineChannelAccessToken, cfg.LineUserID)

	// ユースケースを生成
	uc := usecase.NewNotifyScheduleUseCase(calendarRepo, notifier)

	// JST固定で現在時刻を取得
	jst, _ := time.LoadLocation("Asia/Tokyo")
	now := time.Now().In(jst)

	// JST固定で今日と明日の日付を確実に計算
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, jst)
	tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, jst)

	// ユースケースを実行
	skipped, err := uc.Execute(ctx, today, tomorrow)
	if err != nil {
		return LambdaResponse{
			StatusCode: 500,
			Message:    "通知処理エラー",
		}, err
	}

	if skipped {
		return LambdaResponse{
			StatusCode: 200,
			Message:    "予定なしのため通知スキップ",
		}, nil
	}

	return LambdaResponse{
		StatusCode: 200,
		Message:    "通知送信完了",
	}, nil
}

func main() {
	lambda.Start(handler)
}
