# Google Calendar LINE Notifier

BINARY_NAME=bootstrap
MAIN_PATH=cmd
STACK_NAME=google-calendar-line-notifier

# 依存関係インストール
deps:
	go mod tidy

# テスト
test:
	go test ./...

# インテグレーションテスト（.envファイルが必要）
test-integration:
	@echo "Running integration tests... (requires .env file)"
	go test -v -tags=integration ./...

# ローカル実行
run-local:
	go run $(MAIN_PATH)/main.go

# ビルド
build:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY_NAME) $(MAIN_PATH)/main.go

# デプロイ
deploy: build
	sam deploy --stack-name $(STACK_NAME) --region ap-northeast-1 --capabilities CAPABILITY_IAM --no-confirm-changeset

# クリーンアップ
clean:
	rm -f $(BINARY_NAME)