package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/k-negishi/google-calendar-line-notifier/internal/domain"
)

// newTestLINENotifier テスト用の LINENotifier を構築するヘルパー
func newTestLINENotifier(token, userID string, httpClient *http.Client, endpoint string, clock func() time.Time) *LINENotifier {
	return &LINENotifier{
		channelAccessToken: token,
		userID:             userID,
		httpClient:         httpClient,
		endpoint:           endpoint,
		clock:              clock,
	}
}

// --- getWeekdayJapanese テスト ---

func TestGetWeekdayJapanese(t *testing.T) {
	tests := []struct {
		weekday  time.Weekday
		expected string
	}{
		{time.Sunday, "日"},
		{time.Monday, "月"},
		{time.Tuesday, "火"},
		{time.Wednesday, "水"},
		{time.Thursday, "木"},
		{time.Friday, "金"},
		{time.Saturday, "土"},
	}

	for _, tt := range tests {
		t.Run(tt.weekday.String(), func(t *testing.T) {
			assert.Equal(t, tt.expected, getWeekdayJapanese(tt.weekday))
		})
	}
}

// --- buildScheduleMessage テスト ---

func TestBuildScheduleMessage_WithEvents(t *testing.T) {
	jst := time.FixedZone("JST", 9*60*60)
	fixedTime := time.Date(2024, 1, 15, 9, 0, 0, 0, jst)

	n := newTestLINENotifier("token", "user", http.DefaultClient, "", func() time.Time {
		return fixedTime
	})

	todayEvents := []domain.Event{
		{Title: "朝会", StartTime: fixedTime, EndTime: fixedTime.Add(30 * time.Minute), IsAllDay: false},
	}
	tomorrowEvents := []domain.Event{
		{
			Title:     "終日イベント",
			StartTime: time.Date(2024, 1, 16, 0, 0, 0, 0, jst),
			EndTime:   time.Date(2024, 1, 16, 0, 0, 0, 0, jst),
			IsAllDay:  true,
		},
	}

	message := n.buildScheduleMessage(todayEvents, tomorrowEvents)

	assert.Contains(t, message, "本日 1/15(月)")
	assert.Contains(t, message, "(1件)")
	assert.Contains(t, message, "朝会")
	assert.Contains(t, message, "翌日 1/16(火)")
	assert.Contains(t, message, "終日イベント")
}

func TestBuildScheduleMessage_NoEvents(t *testing.T) {
	jst := time.FixedZone("JST", 9*60*60)
	fixedTime := time.Date(2024, 1, 15, 9, 0, 0, 0, jst)

	n := newTestLINENotifier("token", "user", http.DefaultClient, "", func() time.Time {
		return fixedTime
	})

	message := n.buildScheduleMessage(nil, nil)

	assert.Contains(t, message, "本日 1/15(月): 予定なし")
	assert.Contains(t, message, "翌日 1/16(火): 予定なし")
}

// --- appendEventToMessage テスト ---

func TestAppendEventToMessage_TimedEvent(t *testing.T) {
	var builder strings.Builder

	jst := time.FixedZone("JST", 9*60*60)
	event := domain.Event{
		Title:     "定例ミーティング",
		StartTime: time.Date(2024, 1, 15, 10, 0, 0, 0, jst),
		EndTime:   time.Date(2024, 1, 15, 11, 0, 0, 0, jst),
		IsAllDay:  false,
	}

	appendEventToMessage(&builder, event)

	assert.Equal(t, "🔸 10:00 ～ 11:00 定例ミーティング\n", builder.String())
}

func TestAppendEventToMessage_MultiDayTimedEvent(t *testing.T) {
	var builder strings.Builder

	jst := time.FixedZone("JST", 9*60*60)
	event := domain.Event{
		Title:     "1次先行受付(抽選)",
		StartTime: time.Date(2024, 10, 2, 18, 0, 0, 0, jst),
		EndTime:   time.Date(2024, 10, 12, 23, 59, 0, 0, jst),
		IsAllDay:  false,
	}

	appendEventToMessage(&builder, event)

	assert.Equal(t, "🔸 10/2 18:00 ～ 10/12 23:59 1次先行受付(抽選)\n", builder.String())
}

func TestAppendEventToMessage_AllDayEvent(t *testing.T) {
	var builder strings.Builder

	jst := time.FixedZone("JST", 9*60*60)
	event := domain.Event{
		Title:     "休暇",
		StartTime: time.Date(2024, 1, 15, 0, 0, 0, 0, jst),
		EndTime:   time.Date(2024, 1, 15, 0, 0, 0, 0, jst),
		IsAllDay:  true,
	}

	appendEventToMessage(&builder, event)

	assert.Equal(t, "🔸 1/15 (終日) 休暇\n", builder.String())
}

func TestAppendEventToMessage_WithLocation(t *testing.T) {
	var builder strings.Builder

	jst := time.FixedZone("JST", 9*60*60)
	event := domain.Event{
		Title:     "外部ミーティング",
		StartTime: time.Date(2024, 1, 15, 14, 0, 0, 0, jst),
		EndTime:   time.Date(2024, 1, 15, 15, 0, 0, 0, jst),
		IsAllDay:  false,
		Location:  "渋谷オフィス",
	}

	appendEventToMessage(&builder, event)

	result := builder.String()
	assert.Contains(t, result, "外部ミーティング")
	assert.Contains(t, result, "📍 渋谷オフィス")
}

// --- formatEventDateTimeRange テスト ---

func TestFormatEventDateTimeRange(t *testing.T) {
	jst := time.FixedZone("JST", 9*60*60)

	tests := []struct {
		name     string
		event    domain.Event
		expected string
	}{
		{
			name: "時刻あり・同日は時刻のみ",
			event: domain.Event{
				StartTime: time.Date(2024, 1, 15, 13, 20, 0, 0, jst),
				EndTime:   time.Date(2024, 1, 15, 14, 20, 0, 0, jst),
			},
			expected: "13:20 ～ 14:20",
		},
		{
			name: "時刻あり・日をまたぐと開始終了の両方に日付が付く",
			event: domain.Event{
				StartTime: time.Date(2024, 10, 2, 18, 0, 0, 0, jst),
				EndTime:   time.Date(2024, 10, 12, 23, 59, 0, 0, jst),
			},
			expected: "10/2 18:00 ～ 10/12 23:59",
		},
		{
			name: "時刻あり・年をまたぐ場合も日付のみで表現する",
			event: domain.Event{
				StartTime: time.Date(2024, 12, 31, 22, 0, 0, 0, jst),
				EndTime:   time.Date(2025, 1, 1, 2, 0, 0, 0, jst),
			},
			expected: "12/31 22:00 ～ 1/1 02:00",
		},
		{
			name: "終日・1日は日付のみ",
			event: domain.Event{
				StartTime: time.Date(2024, 1, 15, 0, 0, 0, 0, jst),
				EndTime:   time.Date(2024, 1, 15, 0, 0, 0, 0, jst),
				IsAllDay:  true,
			},
			expected: "1/15 (終日)",
		},
		{
			name: "終日・複数日は期間を表示する",
			event: domain.Event{
				StartTime: time.Date(2024, 10, 2, 0, 0, 0, 0, jst),
				EndTime:   time.Date(2024, 10, 12, 0, 0, 0, 0, jst),
				IsAllDay:  true,
			},
			expected: "10/2 ～ 10/12 (終日)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatEventDateTimeRange(tt.event))
		})
	}
}

// --- sendPushMessage テスト（httptest 使用） ---

func TestSendPushMessage_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ヘッダーを検証
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// リクエストボディを検証
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var pushReq linePushRequest
		err = json.Unmarshal(body, &pushReq)
		require.NoError(t, err)
		assert.Equal(t, "test-user", pushReq.To)
		assert.Len(t, pushReq.Messages, 1)
		assert.Equal(t, "text", pushReq.Messages[0].Type)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := newTestLINENotifier("test-token", "test-user", server.Client(), server.URL, time.Now)

	err := n.sendPushMessage(context.Background(), "テストメッセージ")
	assert.NoError(t, err)
}

func TestSendPushMessage_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		err := json.NewEncoder(w).Encode(lineErrorResponse{
			Message: "Invalid request",
		})
		require.NoError(t, err)
	}))
	defer server.Close()

	n := newTestLINENotifier("test-token", "test-user", server.Client(), server.URL, time.Now)

	err := n.sendPushMessage(context.Background(), "テストメッセージ")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LINE API呼び出しが失敗しました")
}

func TestSendScheduleNotification(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var pushReq linePushRequest
		err = json.Unmarshal(body, &pushReq)
		require.NoError(t, err)

		// メッセージが構築されていることを確認
		assert.Contains(t, pushReq.Messages[0].Text, "Google Calendar LINE Notifier")

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	jst := time.FixedZone("JST", 9*60*60)
	fixedTime := time.Date(2024, 1, 15, 9, 0, 0, 0, jst)

	n := newTestLINENotifier("test-token", "test-user", server.Client(), server.URL, func() time.Time {
		return fixedTime
	})

	todayEvents := []domain.Event{
		{
			Title:     "テストイベント",
			StartTime: time.Date(2024, 1, 15, 10, 0, 0, 0, jst),
			EndTime:   time.Date(2024, 1, 15, 11, 0, 0, 0, jst),
			IsAllDay:  false,
		},
	}

	err := n.SendScheduleNotification(context.Background(), todayEvents, nil)
	assert.NoError(t, err)
}
