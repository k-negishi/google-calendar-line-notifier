package main

import (
	"context"
	"log"
	"time"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/k-negishi/google-calendar-line-notifier/internal/calendar"
	"github.com/k-negishi/google-calendar-line-notifier/internal/config"
	"github.com/k-negishi/google-calendar-line-notifier/internal/line_notifier"
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
func handler(ctx context.Context, event LambdaEvent) (LambdaResponse, error) {
	log.Println("Google Calendar LINE通知処理を開始します")

	// 設定を読み込み
	cfg, err := config.Load()
	if err != nil {
		log.Printf("設定の読み込みに失敗しました: %v", err)
		return LambdaResponse{
			StatusCode: 500,
			Message:    "設定読み込みエラー",
		}, err
	}

	// Google Calendarクライアントを初期化
	calendarClient, err := calendar.NewClient(cfg)
	if err != nil {
		log.Printf("Google Calendarクライアントの初期化に失敗しました: %v", err)
		return LambdaResponse{
			StatusCode: 500,
			Message:    "Google Calendar初期化エラー",
		}, err
	}

	// LINE通知クライアントを初期化
	lineNotifier := line_notifier.NewNotifier(cfg)

	// 現在時刻（日本時間）を取得
	jst, _ := time.LoadLocation("Asia/Tokyo")
	now := time.Now().In(jst)
	today := now.Truncate(24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)

	log.Printf("通知対象日: 今日=%s, 明日=%s", today.Format("2006-01-02"), tomorrow.Format("2006-01-02"))

	// 今日と明日の予定を取得
	todayEvents, err := calendarClient.GetEvents(ctx, today, today.Add(24*time.Hour))
	if err != nil {
		log.Printf("今日の予定取得に失敗しました: %v", err)
		return LambdaResponse{
			StatusCode: 500,
			Message:    "今日の予定取得エラー",
		}, err
	}

	tomorrowEvents, err := calendarClient.GetEvents(ctx, tomorrow, tomorrow.Add(24*time.Hour))
	if err != nil {
		log.Printf("明日の予定取得に失敗しました: %v", err)
		return LambdaResponse{
			StatusCode: 500,
			Message:    "明日の予定取得エラー",
		}, err
	}

	// 予定が両日ともない場合はスキップ
	if len(todayEvents) == 0 && len(tomorrowEvents) == 0 {
		log.Println("今日と明日の予定がないため、通知をスキップします")
		return LambdaResponse{
			StatusCode: 200,
			Message:    "予定なしのため通知スキップ",
		}, nil
	}

	// LINE通知メッセージを作成・送信
	err = lineNotifier.SendScheduleNotification(ctx, todayEvents, tomorrowEvents)
	if err != nil {
		log.Printf("LINE通知の送信に失敗しました: %v", err)
		return LambdaResponse{
			StatusCode: 500,
			Message:    "LINE通知送信エラー",
		}, err
	}

	log.Println("Google Calendar LINE通知処理が正常に完了しました")
	return LambdaResponse{
		StatusCode: 200,
		Message:    "通知送信完了",
	}, nil
}

func main() {
	lambda.Start(handler)
}
