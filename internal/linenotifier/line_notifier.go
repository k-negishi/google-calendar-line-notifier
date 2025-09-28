package linenotifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Event struct {
	Title     string
	StartTime time.Time
	EndTime   time.Time
	IsAllDay  bool
	Location  string
}

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
func NewNotifier(channelAccessToken, userID string) *Notifier {
	return &Notifier{
		channelAccessToken: channelAccessToken,
		userID:             userID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SendScheduleNotification カレンダー予定をLINEで通知
func (notifier *Notifier) SendScheduleNotification(ctx context.Context, todayEvents, tomorrowEvents []Event) error {
	// 通知メッセージを作成
	message := notifier.buildScheduleMessage(todayEvents, tomorrowEvents)

	// LINE Push APIでメッセージを送信
	return notifier.sendPushMessage(ctx, message)
}

// buildScheduleMessage 予定通知用のメッセージを構築
func (notifier *Notifier) buildScheduleMessage(todayEvents, tomorrowEvents []Event) string {
	var messageBuilder strings.Builder
	jst, _ := time.LoadLocation("Asia/Tokyo")
	today := time.Now().In(jst)

	// Google Calendar LINE Notifier
	messageBuilder.WriteString("Google Calendar LINE Notifier\n\n")

	// 本日の予定
	dowToday := getWeekdayJapanese(today.Weekday())
	if len(todayEvents) > 0 {
		messageBuilder.WriteString(fmt.Sprintf("本日 %s(%s) (%d件):\n", today.Format("1/2"), dowToday, len(todayEvents)))
		for _, event := range todayEvents {
			notifier.appendEventToMessage(&messageBuilder, event)
		}
	} else {
		messageBuilder.WriteString(fmt.Sprintf("本日 %s(%s): 予定なし\n", today.Format("1/2"), dowToday))
	}

	messageBuilder.WriteString("\n\n")

	// 翌日の予定
	tomorrow := today.Add(24 * time.Hour)
	dowTomorrow := getWeekdayJapanese(tomorrow.Weekday())
	if len(tomorrowEvents) > 0 {
		messageBuilder.WriteString(fmt.Sprintf("翌日 %s(%s) (%d件):\n", tomorrow.Format("1/2"), dowTomorrow, len(tomorrowEvents)))
		for _, event := range tomorrowEvents {
			notifier.appendEventToMessage(&messageBuilder, event)
		}
	} else {
		messageBuilder.WriteString(fmt.Sprintf("翌日 %s(%s): 予定なし\n", tomorrow.Format("1/2"), dowTomorrow))
	}

	return messageBuilder.String()
}

// appendEventToMessage イベントをメッセージに追加
func (notifier *Notifier) appendEventToMessage(builder *strings.Builder, event Event) {
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

// getWeekdayJapanese 曜日を日本語に変換
func getWeekdayJapanese(weekday time.Weekday) string {
	weekdays := map[time.Weekday]string{
		time.Sunday:    "日",
		time.Monday:    "月",
		time.Tuesday:   "火",
		time.Wednesday: "水",
		time.Thursday:  "木",
		time.Friday:    "金",
		time.Saturday:  "土",
	}
	return weekdays[weekday]
}
