package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/calendar/v3"

	"github.com/k-negishi/google-calendar-line-notifier/internal/domain"
)

// MockEventsProvider は EventsProvider のテスト用モック
type MockEventsProvider struct {
	mock.Mock
}

func (m *MockEventsProvider) ListEvents(calendarID, timeMin, timeMax string) ([]*calendar.Event, error) {
	args := m.Called(calendarID, timeMin, timeMax)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*calendar.Event), args.Error(1)
}

// --- convertToEvent テスト（純粋ロジック） ---

func TestConvertToEvent_TimedEvent(t *testing.T) {
	jst, _ := time.LoadLocation("Asia/Tokyo")
	repo := NewGoogleCalendarRepositoryWithProvider(nil, "test", jst)

	event := &calendar.Event{
		Id:       "1",
		Summary:  "テストイベント",
		Location: "東京",
		Start:    &calendar.EventDateTime{DateTime: "2024-01-15T10:00:00+09:00"},
		End:      &calendar.EventDateTime{DateTime: "2024-01-15T11:00:00+09:00"},
	}

	result, err := repo.convertToEvent(event)
	require.NoError(t, err)
	assert.Equal(t, "1", result.ID)
	assert.Equal(t, "テストイベント", result.Title)
	assert.Equal(t, "東京", result.Location)
	assert.False(t, result.IsAllDay)
	assert.Equal(t, 10, result.StartTime.Hour())
	assert.Equal(t, 11, result.EndTime.Hour())
}

func TestConvertToEvent_AllDayEvent(t *testing.T) {
	jst, _ := time.LoadLocation("Asia/Tokyo")
	repo := NewGoogleCalendarRepositoryWithProvider(nil, "test", jst)

	event := &calendar.Event{
		Id:      "2",
		Summary: "終日イベント",
		Start:   &calendar.EventDateTime{Date: "2024-01-15"},
		End:     &calendar.EventDateTime{Date: "2024-01-16"},
	}

	result, err := repo.convertToEvent(event)
	require.NoError(t, err)
	assert.True(t, result.IsAllDay)
	assert.Equal(t, "終日イベント", result.Title)
}

func TestConvertToEvent_EmptyTitle(t *testing.T) {
	jst, _ := time.LoadLocation("Asia/Tokyo")
	repo := NewGoogleCalendarRepositoryWithProvider(nil, "test", jst)

	event := &calendar.Event{
		Id:      "3",
		Summary: "",
		Start:   &calendar.EventDateTime{DateTime: "2024-01-15T10:00:00+09:00"},
		End:     &calendar.EventDateTime{DateTime: "2024-01-15T11:00:00+09:00"},
	}

	result, err := repo.convertToEvent(event)
	require.NoError(t, err)
	assert.Equal(t, "（無題）", result.Title)
}

func TestConvertToEvent_NoStartTime(t *testing.T) {
	jst, _ := time.LoadLocation("Asia/Tokyo")
	repo := NewGoogleCalendarRepositoryWithProvider(nil, "test", jst)

	event := &calendar.Event{
		Id:    "4",
		Start: &calendar.EventDateTime{},
		End:   &calendar.EventDateTime{DateTime: "2024-01-15T11:00:00+09:00"},
	}

	_, err := repo.convertToEvent(event)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "開始時刻が設定されていません")
}

// --- GetEvents テスト（モック使用） ---

func TestGetEvents_Success(t *testing.T) {
	jst, _ := time.LoadLocation("Asia/Tokyo")
	mockProvider := new(MockEventsProvider)
	repo := NewGoogleCalendarRepositoryWithProvider(mockProvider, "test-calendar", jst)

	targetDate := time.Date(2024, 1, 15, 0, 0, 0, 0, jst)

	events := []*calendar.Event{
		{
			Id:      "1",
			Summary: "朝会",
			Start:   &calendar.EventDateTime{DateTime: "2024-01-15T09:00:00+09:00"},
			End:     &calendar.EventDateTime{DateTime: "2024-01-15T09:30:00+09:00"},
		},
	}

	mockProvider.On("ListEvents", "test-calendar", mock.AnythingOfType("string"), mock.AnythingOfType("string")).
		Return(events, nil)

	result, err := repo.GetEvents(context.Background(), targetDate)
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "朝会", result[0].Title)
	assert.IsType(t, domain.Event{}, result[0])
	mockProvider.AssertExpectations(t)
}

func TestGetEvents_APIError(t *testing.T) {
	jst, _ := time.LoadLocation("Asia/Tokyo")
	mockProvider := new(MockEventsProvider)
	repo := NewGoogleCalendarRepositoryWithProvider(mockProvider, "test-calendar", jst)

	targetDate := time.Date(2024, 1, 15, 0, 0, 0, 0, jst)

	mockProvider.On("ListEvents", "test-calendar", mock.AnythingOfType("string"), mock.AnythingOfType("string")).
		Return(nil, errors.New("API error"))

	_, err := repo.GetEvents(context.Background(), targetDate)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "カレンダーイベントの取得に失敗しました")
	mockProvider.AssertExpectations(t)
}

func TestGetEvents_EmptyResult(t *testing.T) {
	jst, _ := time.LoadLocation("Asia/Tokyo")
	mockProvider := new(MockEventsProvider)
	repo := NewGoogleCalendarRepositoryWithProvider(mockProvider, "test-calendar", jst)

	targetDate := time.Date(2024, 1, 15, 0, 0, 0, 0, jst)

	mockProvider.On("ListEvents", "test-calendar", mock.AnythingOfType("string"), mock.AnythingOfType("string")).
		Return([]*calendar.Event{}, nil)

	result, err := repo.GetEvents(context.Background(), targetDate)
	require.NoError(t, err)
	assert.Empty(t, result)
	mockProvider.AssertExpectations(t)
}
