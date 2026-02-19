package gateway

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/k-negishi/google-calendar-line-notifier/internal/domain"
)

// EventsProvider はカレンダーイベントの取得を抽象化する
type EventsProvider interface {
	ListEvents(calendarID, timeMin, timeMax string) ([]*calendar.Event, error)
}

// googleEventsProvider は Google Calendar API を使用した EventsProvider の実装
type googleEventsProvider struct {
	service *calendar.Service
}

func (p *googleEventsProvider) ListEvents(calendarID, timeMin, timeMax string) ([]*calendar.Event, error) {
	eventsCall := p.service.Events.List(calendarID).
		TimeMin(timeMin).
		TimeMax(timeMax).
		SingleEvents(true).
		OrderBy("startTime").
		MaxResults(50)

	events, err := eventsCall.Do()
	if err != nil {
		return nil, err
	}
	return events.Items, nil
}

// GoogleCalendarRepository Google Calendar APIを使用したCalendarRepositoryの実装
type GoogleCalendarRepository struct {
	provider   EventsProvider
	calendarID string
	timezone   *time.Location
}

// NewGoogleCalendarRepository Google Calendarリポジトリを作成
func NewGoogleCalendarRepository(credentialsJSON []byte, calendarID string) (*GoogleCalendarRepository, error) {
	// JST固定でタイムゾーンを設定
	timezone, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		return nil, fmt.Errorf("JSTタイムゾーンの読み込みに失敗しました: %v", err)
	}

	// サービスアカウント認証でCalendar APIクライアントを作成
	creds, err := google.CredentialsFromJSON(
		context.Background(),
		credentialsJSON,
		calendar.CalendarReadonlyScope,
	)
	if err != nil {
		return nil, fmt.Errorf("google認証情報の読み込みに失敗しました: %v", err)
	}

	service, err := calendar.NewService(
		context.Background(),
		option.WithCredentials(creds),
	)
	if err != nil {
		return nil, fmt.Errorf("google Calendar APIサービスの作成に失敗しました: %v", err)
	}

	provider := &googleEventsProvider{service: service}
	return NewGoogleCalendarRepositoryWithProvider(provider, calendarID, timezone), nil
}

// NewGoogleCalendarRepositoryWithProvider EventsProviderを指定してリポジトリを作成（テスト用）
func NewGoogleCalendarRepositoryWithProvider(provider EventsProvider, calendarID string, timezone *time.Location) *GoogleCalendarRepository {
	return &GoogleCalendarRepository{
		provider:   provider,
		calendarID: calendarID,
		timezone:   timezone,
	}
}

// GetEvents 指定された日の予定を取得
func (r *GoogleCalendarRepository) GetEvents(_ context.Context, targetDate time.Time) ([]domain.Event, error) {
	// JST固定で開始時刻と終了時刻を設定
	jst, _ := time.LoadLocation("Asia/Tokyo")

	// 開始時刻: 指定日の00:00:00 (JST) - inclusive
	startTimeInJST := time.Date(
		targetDate.Year(), targetDate.Month(), targetDate.Day(),
		0, 0, 0, 0, jst,
	)

	// 終了時刻: 翌日の00:00:00 (JST) - exclusive
	endTimeInJST := startTimeInJST.Add(24 * time.Hour)

	// RFC3339形式に変換（タイムゾーン情報付き）
	timeMinStr := startTimeInJST.Format(time.RFC3339)
	timeMaxStr := endTimeInJST.Format(time.RFC3339)

	// EventsProvider経由でイベントを取得
	items, err := r.provider.ListEvents(r.calendarID, timeMinStr, timeMaxStr)
	if err != nil {
		return nil, fmt.Errorf("カレンダーイベントの取得に失敗しました: %v", err)
	}

	// イベントを変換
	domainEvents := make([]domain.Event, 0, len(items))
	for _, event := range items {
		domainEvent, err := r.convertToEvent(event)
		if err != nil {
			fmt.Printf("Warning: イベントの変換をスキップしました: %v\n", err)
			continue
		}
		domainEvents = append(domainEvents, domainEvent)
	}

	return domainEvents, nil
}

// convertToEvent Google Calendar APIのイベントをドメインエンティティに変換
func (r *GoogleCalendarRepository) convertToEvent(event *calendar.Event) (domain.Event, error) {
	domainEvent := domain.Event{
		ID:          event.Id,
		Title:       event.Summary,
		Location:    event.Location,
		Description: event.Description,
	}

	// タイトルが空の場合は「（無題）」に設定
	if domainEvent.Title == "" {
		domainEvent.Title = "（無題）"
	}

	// 開始時刻の処理
	if event.Start.DateTime != "" {
		// 時刻指定ありのイベント
		startTime, err := time.Parse(time.RFC3339, event.Start.DateTime)
		if err != nil {
			return domain.Event{}, fmt.Errorf("開始時刻の解析に失敗しました: %v", err)
		}
		domainEvent.StartTime = startTime.In(r.timezone)
		domainEvent.IsAllDay = false
	} else if event.Start.Date != "" {
		// 終日イベント
		startTime, err := time.Parse("2006-01-02", event.Start.Date)
		if err != nil {
			return domain.Event{}, fmt.Errorf("開始日の解析に失敗しました: %v", err)
		}
		domainEvent.StartTime = startTime.In(r.timezone)
		domainEvent.IsAllDay = true
	} else {
		return domain.Event{}, fmt.Errorf("開始時刻が設定されていません")
	}

	// 終了時刻の処理
	if event.End.DateTime != "" {
		// 時刻指定ありのイベント
		endTime, err := time.Parse(time.RFC3339, event.End.DateTime)
		if err != nil {
			return domain.Event{}, fmt.Errorf("終了時刻の解析に失敗しました: %v", err)
		}
		domainEvent.EndTime = endTime.In(r.timezone)
	} else if event.End.Date != "" {
		// 終日イベント
		endTime, err := time.Parse("2006-01-02", event.End.Date)
		if err != nil {
			return domain.Event{}, fmt.Errorf("終了日の解析に失敗しました: %v", err)
		}
		domainEvent.EndTime = endTime.In(r.timezone)
	} else {
		return domain.Event{}, fmt.Errorf("終了時刻が設定されていません")
	}

	return domainEvent, nil
}
