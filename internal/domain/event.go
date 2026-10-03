package domain

import "time"

// Event カレンダーイベントのドメインエンティティ
type Event struct {
	ID    string
	Title string
	// StartTime 時刻ありイベントは開始時刻、終日イベントは開始日の00:00(JST)
	StartTime time.Time
	// EndTime 時刻ありイベントは終了時刻、終日イベントは最終日の00:00(JST)
	// 終日イベントは「最終日の00:00」を指すため、EndTime.Sub(StartTime) は1日の予定では0になる
	EndTime     time.Time
	IsAllDay    bool
	Location    string
	Description string
}
