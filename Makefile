# Google Calendar LINE Notifier

GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint

BINARY_NAME=bootstrap
MAIN_PATH=cmd
STACK_NAME=google-calendar-line-notifier

# 依存関係インストール
deps:
	go mod tidy

# テスト
test: lint
	go test -cover -race ./...

# Linter
lint:
	@echo "Running linter..."
	@$(GOLANGCI_LINT) run ./...

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