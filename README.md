# SabaLauncher

![SabaLauncher Logo](assets/launcher_icon.ico)

## Minecraft用のモダンでシンプルなランチャー

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.24.4-blue.svg)](https://golang.org/)
[![Platform](https://img.shields.io/badge/Platform-Windows-lightgrey.svg)](https://www.microsoft.com/windows)
[![Release](https://img.shields.io/github/v/release/ikafly144/sabalauncher)](https://github.com/ikafly144/sabalauncher/releases)

## 概要

SabaLauncherは、Goで開発されたMinecraft Java Edition用のカスタムランチャーです。VanillaおよびForge環境の両方をサポートし、直感的なGUIとマイクロソフトアカウント認証を提供します。

## 主な機能

### 🚀 ゲーム管理

- **Vanilla Minecraft**: 公式バージョンの完全サポート
- **Minecraft Forge**: Forgeモッドローダーの自動セットアップとインストール
- **自動更新**: Javaランタイム、アセット、ライブラリの自動ダウンロード
- **プロファイル管理**: 複数のゲームプロファイルの管理

### 🔐 認証システム

- **Microsoft アカウント**: 公式のMicrosoft認証システム
- **セキュア**: OAuth 2.0による安全な認証
- **キャッシュ**: 認証情報の安全なキャッシュ

### 🎮 ユーザーエクスペリエンス

- **モダンUI**: Gioを使用したネイティブGUI
- **進捗表示**: ダウンロードとセットアップの詳細な進捗
- **Discord Rich Presence**: ゲーム状況のDiscord表示
- **ログ管理**: 詳細なログ出力とエクスポート機能

### 📦 Mod サポート

- **CurseForge**: CurseForge APIを使用したMod管理
- **モッドパック**: ZIPベースのモッドパック配布
- **自動同期**: Modの自動ダウンロードと更新

## システム要件

- **OS**: Windows 10/11
- **Java**: Java 8以上（自動でダウンロードされます）
- **メモリ**: 4GB RAM以上推奨
- **ストレージ**: 2GB以上の空き容量

## インストール

### リリース版（推奨）

1. [Releases](https://github.com/ikafly144/sabalauncher/releases)から最新のMSIファイルをダウンロード
2. `SabaLauncher.msi`を実行してインストール
3. インストール完了後、スタートメニューから起動

### 開発版ビルド

```powershell
# リポジトリをクローン
git clone https://github.com/ikafly144/sabalauncher.git
cd sabalauncher

# 依存関係をインストール
go mod download

# ビルド
go build -o sabalauncher.exe .

# 実行
.\sabalauncher.exe
```

## 設定

### 初回セットアップ

1. アプリケーションを起動
2. Microsoftアカウントでログイン
3. Minecraftライセンスの確認
4. プロファイルを作成または追加

## 使用方法

### プロファイルの追加

1. 「プロファイルを追加」ボタンをクリック
2. 配布URLまたはローカルファイルを指定
3. プロファイルのダウンロードとセットアップを待機

### ゲームの起動

1. プロファイルを選択
2. 「プレイ」ボタンをクリック
3. 必要なファイルの自動ダウンロードを待機
4. ゲームが自動起動

### プロファイル管理

- **削除**: プロファイルを右クリック → 「プロファイルを削除」
- **更新**: プロファイルを右クリック → 「キャッシュを削除」

## ライセンス

MIT License

Copyright (c) 2025 ikafly144

詳細は [LICENSE](LICENSE) ファイルを参照してください。

## 免責事項

このソフトウェアはMojang Studios/Microsoftとは無関係であり、公式サポートは提供されません。使用は自己責任でお願いします。Minecraftの正規ライセンスが必要です。

---

Made with ❤️ by [ikafly144](https://github.com/ikafly144)
