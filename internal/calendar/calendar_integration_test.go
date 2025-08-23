//go:build integration

package calendar

import (
	"context"
	"testing"
	"time"

	"github.com/k-negishi/google-calendar-line-notifier/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetEvents_Integration(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("インテグレーションテストの実行には.envファイルに設定された有効な認証情報が必要です: %v", err)
	}
	require.NotEmpty(t, cfg.GoogleCredentials, "GOOGLE_CREDENTIALSが設定されていません")
	require.NotEmpty(t, cfg.CalendarID, "CALENDAR_IDが設定されていません")

	client, err := NewClient([]byte(cfg.GoogleCredentials), cfg.CalendarID)
	require.NoError(t, err, "カレンダーのクライアント作成に失敗しました")

	t.Run("Google Calendarから実際にイベントを取得する", func(t *testing.T) {
		// このテストは、Google Calendarに実際に接続してイベントを取得しようとします。
		// タイムアウトを設定して、無限に待機しないようにします。
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// テスト当日の日付でイベントを取得
		// このテストが成功するかどうかは、実行する日のカレンダーの状態に依存します。
		// ここでは、API呼び出しがエラーなく完了することを確認するのが主目的です。
		_, err := client.GetEvents(ctx, time.Now())
		assert.NoError(t, err, "GetEventsで予期せぬエラーが発生しました")
	})
}
