package calendar

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// newTestClient はテスト用のGoogle Calendarクライアントとモックサーバーを返します
func newTestClient(t *testing.T, handler http.Handler) (*Client, *httptest.Server) {
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	ctx := context.Background()
	svc, err := calendar.NewService(ctx,
		option.WithEndpoint(server.URL),
		option.WithHTTPClient(server.Client()),
		option.WithoutAuthentication(),
	)
	assert.NoError(t, err)

	jst, err := time.LoadLocation("Asia/Tokyo")
	assert.NoError(t, err)

	return &Client{
		service:    svc,
		calendarID: "test-calendar-id",
		timezone:   jst,
	}, server
}

func TestGetEvents(t *testing.T) {
	t.Run("正常系: 複数のイベントが取得できる", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.Path, "test-calendar-id/events")

			events := &calendar.Events{
				Items: []*calendar.Event{
					{
						Id:      "event1",
						Summary: "Test Event 1",
						Start:   &calendar.EventDateTime{DateTime: "2024-08-23T10:00:00+09:00"},
						End:     &calendar.EventDateTime{DateTime: "2024-08-23T11:00:00+09:00"},
					},
					{
						Id:      "event2",
						Summary: "All Day Event",
						Start:   &calendar.EventDateTime{Date: "2024-08-23"},
						End:     &calendar.EventDateTime{Date: "2024-08-24"},
					},
				},
			}
			err := json.NewEncoder(w).Encode(events)
			assert.NoError(t, err)
		})

		client, _ := newTestClient(t, handler)
		jst, _ := time.LoadLocation("Asia/Tokyo")
		targetDate := time.Date(2024, 8, 23, 0, 0, 0, 0, jst)

		events, err := client.GetEvents(context.Background(), targetDate)
		assert.NoError(t, err)
		assert.Len(t, events, 2)

		// 1つ目のイベントの検証
		assert.Equal(t, "event1", events[0].ID)
		assert.Equal(t, "Test Event 1", events[0].Title)
		assert.False(t, events[0].IsAllDay)

		// 2つ目のイベントの検証
		assert.Equal(t, "event2", events[1].ID)
		assert.Equal(t, "All Day Event", events[1].Title)
		assert.True(t, events[1].IsAllDay)
		expectedStartTime := time.Date(2024, 8, 23, 0, 0, 0, 0, jst)
		assert.Equal(t, expectedStartTime, events[1].StartTime)
	})

	t.Run("異常系: Google Calendar APIがエラーを返す", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})

		client, _ := newTestClient(t, handler)
		jst, _ := time.LoadLocation("Asia/Tokyo")
		targetDate := time.Date(2024, 8, 23, 0, 0, 0, 0, jst)

		_, err := client.GetEvents(context.Background(), targetDate)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "カレンダーイベントの取得に失敗しました")
	})
}


func TestConvertToEvent(t *testing.T) {
	jst, err := time.LoadLocation("Asia/Tokyo")
	assert.NoError(t, err)

	client := &Client{timezone: jst}

	tests := []struct {
		name          string
		input         *calendar.Event
		expected      Event
		expectError   bool
		expectedError string
	}{
		{
			name: "通常イベント（時刻指定あり）",
			input: &calendar.Event{
				Id:      "test-id-1",
				Summary: "Test Event 1",
				Start:   &calendar.EventDateTime{DateTime: "2024-01-01T10:00:00+09:00"},
				End:     &calendar.EventDateTime{DateTime: "2024-01-01T11:00:00+09:00"},
				Location: "Test Location",
				Description: "Test Description",
			},
			expected: Event{
				ID:          "test-id-1",
				Title:       "Test Event 1",
				StartTime:   time.Date(2024, 1, 1, 10, 0, 0, 0, jst),
				EndTime:     time.Date(2024, 1, 1, 11, 0, 0, 0, jst),
				IsAllDay:    false,
				Location:    "Test Location",
				Description: "Test Description",
			},
			expectError: false,
		},
		{
			name: "終日イベント",
			input: &calendar.Event{
				Id:      "test-id-2",
				Summary: "All Day Event",
				Start:   &calendar.EventDateTime{Date: "2024-01-02"},
				End:     &calendar.EventDateTime{Date: "2024-01-03"},
			},
			expected: Event{
				ID:        "test-id-2",
				Title:     "All Day Event",
				StartTime: time.Date(2024, 1, 2, 0, 0, 0, 0, jst),
				EndTime:   time.Date(2024, 1, 3, 0, 0, 0, 0, jst),
				IsAllDay:  true,
			},
			expectError: false,
		},
		{
			name: "タイトルが空のイベント",
			input: &calendar.Event{
				Id:      "test-id-3",
				Summary: "",
				Start:   &calendar.EventDateTime{DateTime: "2024-01-01T10:00:00+09:00"},
				End:     &calendar.EventDateTime{DateTime: "2024-01-01T11:00:00+09:00"},
			},
			expected: Event{
				ID:        "test-id-3",
				Title:     "（無題）",
				StartTime: time.Date(2024, 1, 1, 10, 0, 0, 0, jst),
				EndTime:   time.Date(2024, 1, 1, 11, 0, 0, 0, jst),
				IsAllDay:  false,
			},
			expectError: false,
		},
		{
			name: "開始時刻がない",
			input: &calendar.Event{
				Id:      "test-id-4",
				Summary: "No Start Time",
				Start:   &calendar.EventDateTime{},
				End:     &calendar.EventDateTime{DateTime: "2024-01-01T11:00:00+09:00"},
			},
			expectError:   true,
			expectedError: "開始時刻が設定されていません",
		},
		{
			name: "終了時刻がない",
			input: &calendar.Event{
				Id:      "test-id-5",
				Summary: "No End Time",
				Start:   &calendar.EventDateTime{DateTime: "2024-01-01T10:00:00+09:00"},
				End:     &calendar.EventDateTime{},
			},
			expectError:   true,
			expectedError: "終了時刻が設定されていません",
		},
		{
			name: "不正な開始時刻フォーマット",
			input: &calendar.Event{
				Id:      "test-id-6",
				Summary: "Bad Start Time",
				Start:   &calendar.EventDateTime{DateTime: "invalid-time"},
				End:     &calendar.EventDateTime{DateTime: "2024-01-01T11:00:00+09:00"},
			},
			expectError:   true,
			expectedError: "開始時刻の解析に失敗しました",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := client.convertToEvent(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, actual)
			}
		})
	}
}
