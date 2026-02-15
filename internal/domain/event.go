package domain

import "time"

// Event カレンダーイベントのドメインエンティティ
type Event struct {
	ID          string
	Title       string
	StartTime   time.Time
	EndTime     time.Time
	IsAllDay    bool
	Location    string
	Description string
}
