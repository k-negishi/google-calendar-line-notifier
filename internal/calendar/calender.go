package calendar

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/k-negishi/google-calendar-line-notifier/internal/config"
)

// Event カレンダーイベントの構造体
type Event struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	IsAllDay    bool      `json:"is_all_day"`
	Location    string    `json:"location,omitempty"`
	Description string    `json:"description,omitempty"`
}

// Client Google Calendar APIクライアント
type Client struct {
	service    *calendar.Service
	calendarID string
	timezone   *time.Location
}

// NewClient Google Calendarクライアントを作成
func NewClient(cfg *config.Config) (*Client, error) {
	// タイムゾーンを設定
	timezone, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, fmt.Errorf("タイムゾーンの読み込みに失敗しました: %v", err)
	}

	// Google認証情報を準備
	credentialsJSON := []byte(cfg.GoogleCredentials)

	// サービスアカウント認証でCalendar APIクライアントを作成
	creds, err := google.CredentialsFromJSON(
		context.Background(),
		credentialsJSON,
		calendar.CalendarReadonlyScope,
	)
	if err != nil {
		return nil, fmt.Errorf("Google認証情報の読み込みに失敗しました: %v", err)
	}

	service, err := calendar.NewService(
		context.Background(),
		option.WithCredentials(creds),
	)
	if err != nil {
		return nil, fmt.Errorf("Google Calendar APIサービスの作成に失敗しました: %v", err)
	}

	return &Client{
		service:    service,
		calendarID: cfg.CalendarID,
		timezone:   timezone,
	}, nil
}

// GetEvents 指定された期間のイベントを取得
func (c *Client) GetEvents(ctx context.Context, startTime, endTime time.Time) ([]Event, error) {
	// startTimeとendTimeを指定されたタイムゾーン（JST）で明示的に設定
	// Pythonの例: datetime(year, month, day, 0, 0, 0, tzinfo=jst).isoformat()
	startTimeInTZ := time.Date(
		startTime.Year(), startTime.Month(), startTime.Day(),
		0, 0, 0, 0, c.timezone,
	)
	endTimeInTZ := time.Date(
		endTime.Year(), endTime.Month(), endTime.Day(),
		23, 59, 59, 0, c.timezone,
	)

	// JST タイムゾーン情報を含んだRFC3339形式で送信
	timeMinStr := startTimeInTZ.Format(time.RFC3339)
	timeMaxStr := endTimeInTZ.Format(time.RFC3339)

	fmt.Printf("Google Calendar API リクエスト: timeMin=%s, timeMax=%s\n", timeMinStr, timeMaxStr)

	// Google Calendar APIの呼び出し
	eventsCall := c.service.Events.List(c.calendarID).
		TimeMin(timeMinStr).
		TimeMax(timeMaxStr).
		SingleEvents(true).
		OrderBy("startTime").
		MaxResults(50) // 1日の予定上限を50件に設定

	events, err := eventsCall.Do()
	if err != nil {
		return nil, fmt.Errorf("カレンダーイベントの取得に失敗しました: %v", err)
	}

	// イベントを変換
	var calendarEvents []Event
	for _, event := range events.Items {
		calendarEvent, err := c.convertToEvent(event)
		if err != nil {
			fmt.Printf("Warning: イベントの変換をスキップしました: %v\n", err)
			continue
		}
		calendarEvents = append(calendarEvents, calendarEvent)
	}

	return calendarEvents, nil
}

// convertToEvent Google Calendar APIのイベントを内部構造体に変換
func (c *Client) convertToEvent(event *calendar.Event) (Event, error) {
	calendarEvent := Event{
		ID:          event.Id,
		Title:       event.Summary,
		Location:    event.Location,
		Description: event.Description,
	}

	// タイトルが空の場合は「（無題）」に設定
	if calendarEvent.Title == "" {
		calendarEvent.Title = "（無題）"
	}

	// 開始時刻の処理
	if event.Start.DateTime != "" {
		// 時刻指定ありのイベント
		startTime, err := time.Parse(time.RFC3339, event.Start.DateTime)
		if err != nil {
			return Event{}, fmt.Errorf("開始時刻の解析に失敗しました: %v", err)
		}
		calendarEvent.StartTime = startTime.In(c.timezone)
		calendarEvent.IsAllDay = false
	} else if event.Start.Date != "" {
		// 終日イベント
		startTime, err := time.Parse("2006-01-02", event.Start.Date)
		if err != nil {
			return Event{}, fmt.Errorf("開始日の解析に失敗しました: %v", err)
		}
		calendarEvent.StartTime = startTime.In(c.timezone)
		calendarEvent.IsAllDay = true
	} else {
		return Event{}, fmt.Errorf("開始時刻が設定されていません")
	}

	// 終了時刻の処理
	if event.End.DateTime != "" {
		// 時刻指定ありのイベント
		endTime, err := time.Parse(time.RFC3339, event.End.DateTime)
		if err != nil {
			return Event{}, fmt.Errorf("終了時刻の解析に失敗しました: %v", err)
		}
		calendarEvent.EndTime = endTime.In(c.timezone)
	} else if event.End.Date != "" {
		// 終日イベント
		endTime, err := time.Parse("2006-01-02", event.End.Date)
		if err != nil {
			return Event{}, fmt.Errorf("終了日の解析に失敗しました: %v", err)
		}
		calendarEvent.EndTime = endTime.In(c.timezone)
	} else {
		return Event{}, fmt.Errorf("終了時刻が設定されていません")
	}

	return calendarEvent, nil
}
