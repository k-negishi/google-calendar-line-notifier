# Google Calendar LINE Notifier

BINARY_NAME=bootstrap
MAIN_PATH=cmd

# 依存関係インストール
deps:
	go mod tidy

# テスト
test:
	go test ./...

# ローカル実行
run-local:
	go run $(MAIN_PATH)/main.go

# ビルド
build:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BINARY_NAME) $(MAIN_PATH)/main.go

# デプロイ
deploy: build
	sam deploy

# クリーンアップ
clean:
	rm -f $(BINARY_NAME)