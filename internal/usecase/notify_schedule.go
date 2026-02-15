package usecase

import (
	"context"
	"log"
	"time"

	"github.com/k-negishi/google-calendar-line-notifier/internal/domain"
)

// CalendarRepository カレンダーからイベントを取得するポート
type CalendarRepository interface {
	GetEvents(ctx context.Context, targetDate time.Time) ([]domain.Event, error)
}

// Notifier 通知を送信するポート
type Notifier interface {
	SendScheduleNotification(ctx context.Context, todayEvents, tomorrowEvents []domain.Event) error
}

// NotifyScheduleUseCase 予定通知ユースケース
type NotifyScheduleUseCase struct {
	calendarRepo CalendarRepository
	notifier     Notifier
}

// NewNotifyScheduleUseCase ユースケースを生成
func NewNotifyScheduleUseCase(calendarRepo CalendarRepository, notifier Notifier) *NotifyScheduleUseCase {
	return &NotifyScheduleUseCase{
		calendarRepo: calendarRepo,
		notifier:     notifier,
	}
}

// Execute 今日と明日の予定を取得し、LINE通知を送信する
func (uc *NotifyScheduleUseCase) Execute(ctx context.Context, today, tomorrow time.Time) (skipped bool, err error) {
	// 今日の予定を取得
	todayEvents, err := uc.calendarRepo.GetEvents(ctx, today)
	if err != nil {
		log.Printf("今日の予定取得に失敗しました: %v", err)
		return false, err
	}

	// 明日の予定を取得
	tomorrowEvents, err := uc.calendarRepo.GetEvents(ctx, tomorrow)
	if err != nil {
		log.Printf("明日の予定取得に失敗しました: %v", err)
		return false, err
	}

	// 予定が両日ともない場合はスキップ
	if len(todayEvents) == 0 && len(tomorrowEvents) == 0 {
		return true, nil
	}

	// LINE通知を送信
	if err := uc.notifier.SendScheduleNotification(ctx, todayEvents, tomorrowEvents); err != nil {
		log.Printf("LINE通知の送信に失敗しました: %v", err)
		return false, err
	}

	return false, nil
}
