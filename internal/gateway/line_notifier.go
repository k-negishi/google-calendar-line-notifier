package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/k-negishi/google-calendar-line-notifier/internal/domain"
)

// LINENotifier LINE Messaging APIを使用したNotifierの実装
type LINENotifier struct {
	channelAccessToken string
	userID             string
	httpClient         *http.Client
	endpoint           string
	clock              func() time.Time
}

// lineMessage LINE APIに送信するメッセージ構造体
type lineMessage struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// linePushRequest LINE Push APIのリクエスト構造体
type linePushRequest struct {
	To       string        `json:"to"`
	Messages []lineMessage `json:"messages"`
}

// lineErrorResponse LINE APIのエラーレスポンス構造体
type lineErrorResponse struct {
	Message string `json:"message"`
	Details []struct {
		Message  string `json:"message"`
		Property string `json:"property"`
	} `json:"details"`
}

// NewLINENotifier LINE通知クライアントを作成
func NewLINENotifier(channelAccessToken, userID string) *LINENotifier {
	return &LINENotifier{
		channelAccessToken: channelAccessToken,
		userID:             userID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		endpoint: "https://api.line.me/v2/bot/message/push",
		clock:    time.Now,
	}
}

// SendScheduleNotification カレンダー予定をLINEで通知
func (n *LINENotifier) SendScheduleNotification(ctx context.Context, todayEvents, tomorrowEvents []domain.Event) error {
	// 通知メッセージを作成
	message := n.buildScheduleMessage(todayEvents, tomorrowEvents)

	// LINE Push APIでメッセージを送信
	return n.sendPushMessage(ctx, message)
}

// buildScheduleMessage 予定通知用のメッセージを構築
func (n *LINENotifier) buildScheduleMessage(todayEvents, tomorrowEvents []domain.Event) string {
	var messageBuilder strings.Builder
	jst, _ := time.LoadLocation("Asia/Tokyo")
	today := n.clock().In(jst)

	// Google Calendar LINE Notifier
	messageBuilder.WriteString("Google Calendar LINE Notifier\n\n")

	// 本日の予定
	dowToday := getWeekdayJapanese(today.Weekday())
	if len(todayEvents) > 0 {
		messageBuilder.WriteString(fmt.Sprintf("本日 %s(%s) (%d件):\n", today.Format("1/2"), dowToday, len(todayEvents)))
		for _, event := range todayEvents {
			appendEventToMessage(&messageBuilder, event)
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
			appendEventToMessage(&messageBuilder, event)
		}
	} else {
		messageBuilder.WriteString(fmt.Sprintf("翌日 %s(%s): 予定なし\n", tomorrow.Format("1/2"), dowTomorrow))
	}

	return messageBuilder.String()
}

// appendEventToMessage イベントをメッセージに追加
func appendEventToMessage(builder *strings.Builder, event domain.Event) {
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
func (n *LINENotifier) sendPushMessage(ctx context.Context, message string) error {
	// リクエストボディを作成
	pushRequest := linePushRequest{
		To: n.userID,
		Messages: []lineMessage{
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
		n.endpoint,
		bytes.NewBuffer(requestBody),
	)
	if err != nil {
		return fmt.Errorf("HTTPリクエストの作成に失敗しました: %v", err)
	}

	// ヘッダーを設定
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", n.channelAccessToken))

	// APIリクエストを送信
	resp, err := n.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("LINE APIリクエストの送信に失敗しました: %v", err)
	}
	defer resp.Body.Close()

	// レスポンスを確認
	if resp.StatusCode != http.StatusOK {
		// エラーレスポンスの詳細を取得
		var errorResponse lineErrorResponse
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
