package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/k-negishi/google-calendar-line-notifier/internal/domain"
)

// MockCalendarRepository は CalendarRepository のテスト用モック
type MockCalendarRepository struct {
	mock.Mock
}

func (m *MockCalendarRepository) GetEvents(ctx context.Context, targetDate time.Time) ([]domain.Event, error) {
	args := m.Called(ctx, targetDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.Event), args.Error(1)
}

// MockNotifier は Notifier のテスト用モック
type MockNotifier struct {
	mock.Mock
}

func (m *MockNotifier) SendScheduleNotification(ctx context.Context, todayEvents, tomorrowEvents []domain.Event) error {
	args := m.Called(ctx, todayEvents, tomorrowEvents)
	return args.Error(0)
}

// --- Execute テスト ---

func TestExecute_Success(t *testing.T) {
	mockRepo := new(MockCalendarRepository)
	mockNotifier := new(MockNotifier)
	uc := NewNotifyScheduleUseCase(mockRepo, mockNotifier)

	jst := time.FixedZone("JST", 9*60*60)
	today := time.Date(2024, 1, 15, 0, 0, 0, 0, jst)
	tomorrow := time.Date(2024, 1, 16, 0, 0, 0, 0, jst)

	todayEvents := []domain.Event{
		{Title: "朝会", StartTime: today.Add(9 * time.Hour), EndTime: today.Add(10 * time.Hour)},
	}
	tomorrowEvents := []domain.Event{
		{Title: "終日イベント", IsAllDay: true},
	}

	mockRepo.On("GetEvents", mock.Anything, today).Return(todayEvents, nil)
	mockRepo.On("GetEvents", mock.Anything, tomorrow).Return(tomorrowEvents, nil)
	mockNotifier.On("SendScheduleNotification", mock.Anything, todayEvents, tomorrowEvents).Return(nil)

	skipped, err := uc.Execute(context.Background(), today, tomorrow)
	require.NoError(t, err)
	assert.False(t, skipped)
	mockRepo.AssertExpectations(t)
	mockNotifier.AssertExpectations(t)
}

func TestExecute_NoEvents_Skipped(t *testing.T) {
	mockRepo := new(MockCalendarRepository)
	mockNotifier := new(MockNotifier)
	uc := NewNotifyScheduleUseCase(mockRepo, mockNotifier)

	jst := time.FixedZone("JST", 9*60*60)
	today := time.Date(2024, 1, 15, 0, 0, 0, 0, jst)
	tomorrow := time.Date(2024, 1, 16, 0, 0, 0, 0, jst)

	mockRepo.On("GetEvents", mock.Anything, today).Return([]domain.Event{}, nil)
	mockRepo.On("GetEvents", mock.Anything, tomorrow).Return([]domain.Event{}, nil)

	skipped, err := uc.Execute(context.Background(), today, tomorrow)
	require.NoError(t, err)
	assert.True(t, skipped)
	// 予定なしの場合 SendScheduleNotification は呼ばれない
	mockNotifier.AssertNotCalled(t, "SendScheduleNotification")
}

func TestExecute_CalendarError(t *testing.T) {
	mockRepo := new(MockCalendarRepository)
	mockNotifier := new(MockNotifier)
	uc := NewNotifyScheduleUseCase(mockRepo, mockNotifier)

	jst := time.FixedZone("JST", 9*60*60)
	today := time.Date(2024, 1, 15, 0, 0, 0, 0, jst)
	tomorrow := time.Date(2024, 1, 16, 0, 0, 0, 0, jst)

	mockRepo.On("GetEvents", mock.Anything, today).Return(nil, errors.New("calendar API error"))

	_, err := uc.Execute(context.Background(), today, tomorrow)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "calendar API error")
	mockNotifier.AssertNotCalled(t, "SendScheduleNotification")
}

func TestExecute_NotifierError(t *testing.T) {
	mockRepo := new(MockCalendarRepository)
	mockNotifier := new(MockNotifier)
	uc := NewNotifyScheduleUseCase(mockRepo, mockNotifier)

	jst := time.FixedZone("JST", 9*60*60)
	today := time.Date(2024, 1, 15, 0, 0, 0, 0, jst)
	tomorrow := time.Date(2024, 1, 16, 0, 0, 0, 0, jst)

	todayEvents := []domain.Event{
		{Title: "テスト"},
	}

	mockRepo.On("GetEvents", mock.Anything, today).Return(todayEvents, nil)
	mockRepo.On("GetEvents", mock.Anything, tomorrow).Return([]domain.Event{}, nil)
	mockNotifier.On("SendScheduleNotification", mock.Anything, todayEvents, []domain.Event{}).Return(errors.New("LINE API error"))

	_, err := uc.Execute(context.Background(), today, tomorrow)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LINE API error")
}
