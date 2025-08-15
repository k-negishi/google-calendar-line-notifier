package line_notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/k-negishi/google-calendar-line-notifier/internal/calendar"
	"github.com/k-negishi/google-calendar-line-notifier/internal/config"
)

// Notifier LINE Messaging API通知クライアント
type Notifier struct {
	channelAccessToken string
	userID             string
	httpClient         *http.Client
}

// Message LINE APIに送信するメッセージ構造体
type Message struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// PushRequest LINE Push APIのリクエスト構造体
type PushRequest struct {
	To       string    `json:"to"`
	Messages []Message `json:"messages"`
}

// ErrorResponse LINE APIのエラーレスポンス構造体
type ErrorResponse struct {
	Message string `json:"message"`
	Details []struct {
		Message  string `json:"message"`
		Property string `json:"property"`
	} `json:"details"`
}

// NewNotifier LINE通知クライアントを作成
func NewNotifier(cfg *config.Config) *Notifier {
	return &Notifier{
		channelAccessToken: cfg.LineChannelAccessToken,
		userID:             cfg.LineUserID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SendScheduleNotification カレンダー予定をLINEで通知
func (notifier *Notifier) SendScheduleNotification(ctx context.Context, todayEvents, tomorrowEvents []calendar.Event) error {
	// 通知メッセージを作成
	message := notifier.buildScheduleMessage(todayEvents, tomorrowEvents)

	// LINE Push APIでメッセージを送信
	return notifier.sendPushMessage(ctx, message)
}

// SendTestMessage テスト用メッセージを送信（開発・デバッグ用）
func (notifier *Notifier) SendTestMessage(ctx context.Context, message string) error {
	testMessage := fmt.Sprintf("🧪 テストメッセージ\n\n%s\n\n⏰ 送信時刻: %s",
		message,
		time.Now().Format("2006/01/02 15:04:05"))

	return notifier.sendPushMessage(ctx, testMessage)
}

// buildScheduleMessage 予定通知用のメッセージを構築
func (notifier *Notifier) buildScheduleMessage(todayEvents, tomorrowEvents []calendar.Event) string {
	var messageBuilder strings.Builder

	// ヘッダー
	jst, _ := time.LoadLocation("Asia/Tokyo")
	now := time.Now().In(jst)
	messageBuilder.WriteString(fmt.Sprintf("🌅 今日の予定 (%s)\n\n", now.Format("1/2 Mon")))

	// 今日の予定
	if len(todayEvents) > 0 {
		messageBuilder.WriteString(fmt.Sprintf("📅 今日 (%d件):\n", len(todayEvents)))
		for _, event := range todayEvents {
			notifier.appendEventToMessage(&messageBuilder, event)
		}
	} else {
		messageBuilder.WriteString("📅 今日: 予定なし\n")
	}

	messageBuilder.WriteString("\n")

	// 明日の予定
	tomorrow := now.Add(24 * time.Hour)
	if len(tomorrowEvents) > 0 {
		messageBuilder.WriteString(fmt.Sprintf("📅 明日 %s (%d件):\n", tomorrow.Format("1/2 Mon"), len(tomorrowEvents)))
		for _, event := range tomorrowEvents {
			notifier.appendEventToMessage(&messageBuilder, event)
		}
	} else {
		messageBuilder.WriteString(fmt.Sprintf("📅 明日 %s: 予定なし\n", tomorrow.Format("1/2 Mon")))
	}

	// フッター
	messageBuilder.WriteString("\n✨ 良い一日をお過ごしください！")

	return messageBuilder.String()
}

// appendEventToMessage イベントをメッセージに追加
func (notifier *Notifier) appendEventToMessage(builder *strings.Builder, event calendar.Event) {
	if event.IsAllDay {
		builder.WriteString(fmt.Sprintf("🔸 %s (終日)\n", event.Title))
	} else {
		timeRange := fmt.Sprintf("%s〜%s",
			event.StartTime.Format("15:04"),
			event.EndTime.Format("15:04"))
		builder.WriteString(fmt.Sprintf("🔸 %s %s\n", timeRange, event.Title))
	}

	// 場所情報があれば追加
	if event.Location != "" {
		builder.WriteString(fmt.Sprintf("   📍 %s\n", event.Location))
	}
}

// sendPushMessage LINE Push APIでメッセージを送信
func (notifier *Notifier) sendPushMessage(ctx context.Context, message string) error {
	// リクエストボディを作成
	pushRequest := PushRequest{
		To: notifier.userID,
		Messages: []Message{
			{
				Type: "text",
				Text: message,
			},
		},
	}

	requestBody, err := json.Marshal(pushRequest)
	if err != nil {
		return fmt.Errorf("リクエストボディのJSON変換に失敗しました: %v", err)
	}

	// HTTPリクエストを作成
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		"https://api.line.me/v2/bot/message/push",
		bytes.NewBuffer(requestBody),
	)
	if err != nil {
		return fmt.Errorf("HTTPリクエストの作成に失敗しました: %v", err)
	}

	// ヘッダーを設定
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", notifier.channelAccessToken))

	// APIリクエストを送信
	resp, err := notifier.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("LINE APIリクエストの送信に失敗しました: %v", err)
	}
	defer resp.Body.Close()

	// レスポンスを確認
	if resp.StatusCode != http.StatusOK {
		// エラーレスポンスの詳細を取得
		var errorResponse ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errorResponse); err != nil {
			return fmt.Errorf("LINE API呼び出しが失敗しました (Status: %d, レスポンス解析不可: %v)", resp.StatusCode, err)
		}

		errorDetails := errorResponse.Message
		if len(errorResponse.Details) > 0 {
			errorDetails += fmt.Sprintf(" (詳細: %s)", errorResponse.Details[0].Message)
		}

		return fmt.Errorf("LINE API呼び出しが失敗しました (Status: %d): %s", resp.StatusCode, errorDetails)
	}

	return nil
}
