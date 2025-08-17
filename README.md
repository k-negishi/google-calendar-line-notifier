# Google Calendar LINE Notifier

[![Go](https://img.shields.io/badge/go-1.23+-blue.svg)](https://golang.org/dl/)
[![AWS SAM](https://img.shields.io/badge/AWS-SAM-blueviolet.svg)](https://aws.amazon.com/serverless/sam/)
[![AWS EventBridge](https://img.shields.io/badge/AWS-EventBridge-blue.svg)](https://aws.amazon.com/eventbridge/)
[![AWS Lambda](https://img.shields.io/badge/AWS-Lambda-orange.svg)](https://aws.amazon.com/lambda/)

<table>
    <thead>
        <tr>
           <th style="text-align:center"><a href="#日本語版">日本語版</a></th>
           <th style="text-align:center"><a href="#english-version">English Version</a></th>     
        </tr>
    </thead>
</table>

---

## 日本語版

### 概要

Google Calendarから本日と翌日の予定を取得し、毎朝LINE通知するAWS Lambdaベースのアプリケーションです。

### 使用技術
- Go 1.23
- AWS Lambda
- AWS EventBridge
- AWS SAM
- Google Calendar API
- LINE Messaging API


### LINE 通知メッセージの例

例1（通常の予定がある場合）:
```
Google Calendar LINE Notifier

本日 8/17(日) (2件):
🔸 09:00〜10:30 チームミーティング
   📍 会議室A
🔸 プロジェクト準備 (終日)


翌日 8/18(月) (1件):
🔸 14:00〜15:00 顧客打ち合わせ
   📍 オンライン
```

例2（予定がない場合）:
```
Google Calendar LINE Notifier

本日 8/17(日): 予定なし


翌日 8/18(月): 予定なし
```


### 環境構築手順

#### Go環境セットアップ

```bash
# 依存関係をインストール
make deps
```

#### ローカル実行

```bash
# 直接実行
make run-local

# または
go run cmd/main.go
```

#### テスト実行

```bash
# テスト実行
make test
```

#### CI/CDによる自動デプロイ

GitHub Actionsワークフローが設定されており、mainブランチへのプッシュで自動デプロイされます。

---

## English Version

### Overview

An AWS Lambda-based application that fetches today's and tomorrow's events from Google Calendar and sends daily LINE notifications every morning.

### Technologies Used
- Go 1.23
- AWS Lambda
- AWS EventBridge
- AWS SAM
- Google Calendar API
- LINE Messaging API

### Example LINE Notification Messages

Example 1 (with scheduled events):
```
Google Calendar LINE Notifier

本日 8/17(日) (2件):
🔸 09:00〜10:30 チームミーティング
   📍 会議室A
🔸 プロジェクト準備 (終日)


翌日 8/18(月) (1件):
🔸 14:00〜15:00 顧客打ち合わせ
   📍 オンライン
```

Example 2 (no events):
```
Google Calendar LINE Notifier

本日 8/17(日): 予定なし


翌日 8/18(月): 予定なし
```

### Environment Setup

#### Go Environment Setup

```bash
# Install dependencies
make deps
```

#### Local Execution

```bash
# Direct execution
make run-local

# Or
go run cmd/main.go
```

#### Run Tests

```bash
# Run tests
make test
```

#### Automated CI/CD Deployment

GitHub Actions workflow is configured to automatically deploy on push to main branch.