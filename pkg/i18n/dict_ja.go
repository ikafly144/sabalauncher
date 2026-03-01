package i18n

var dictJA = map[string]string{
	"app_title": "SabaLauncher",
	
	// auth.go
	"welcome_title": "SabaLauncherへようこそ",
	"login_prompt": "続けるにはログインしてください",
	"login_browser": "Microsoftでログイン (ブラウザ)",
	"login_device": "Microsoftでログイン (デバイスコード)",
	"logging_in": "ログイン中...",
	"open_browser_btn": "ブラウザでログインページを開く",
	"copy_code_btn": "コードをコピー",
	"device_code_step1": "1. 開く: %s",
	"device_code_step2": "2. コードを入力: %s",
	"browser_login_prompt": "ブラウザでログインを完了してください。",
	"logged_in_as": "ログイン中のユーザー:",
	"go_to_dashboard": "ダッシュボードへ",
	"logout": "ログアウト",
	"auth_error_title": "認証エラー",
	"retry_browser": "再試行 (ブラウザ)",
	"retry_device": "再試行 (デバイスコード)",
	"default_login_error": "ログイン中にエラーが発生しました。",

	// dashboard.go
	"tab_launcher": "ランチャー",
	"tab_settings": "設定",
	"error_prefix": "エラー: %s",
	"import_modpack": "Modpackをインポート",
	"register_remote": "リモート登録",
	"version_label": "バージョン: %s",
	"unknown_version": "不明なバージョン",
	"play_btn": "プレイ",
	"update_btn": "アップデート",
	"delete_instance_btn": "インスタンスを削除",
	"delete_instance_confirm_title": "インスタンスの削除",
	"delete_instance_confirm_body": "%s を削除してもよろしいですか？",
	"select_instance_prompt": "インスタンスを選択して詳細を表示",
	"settings_title": "設定",

	// launch overlay
	"preparing": "準備中...",
	"stop_btn": "停止",

	// import.go
	"importing_progress": "インポート中...",
	"cancel": "キャンセル",
	"registering_progress": "登録中...",
	"register_remote_title": "リモートModpackの登録",
	"register_btn": "登録",
	"manifest_url_label": "マニフェストURL",
	"updating_progress": "アップデート中...",
	"downloading_update": "アップデートをダウンロード中",

	// updater.go
	"update_available_title": "アップデート利用可能",
	"update_available_body": "新しいバージョン (%s) が利用可能です。\n今すぐアップデートしますか？\n\nリリースノート:\n%s",

	// instance_setup.go & setup_state.go
	"setup_instance_name": "%s のセットアップ",
	"setup_java": "実行環境のセットアップ",
	"setup_client": "クライアントのダウンロード",
	"setup_assets": "アセットのダウンロード",
	"setup_library": "ライブラリのダウンロード",
	"setup_forge": "Forgeのセットアップ",
	"install_forge": "Forgeのインストール",
	"setup_modloader": "Mod Loaderのインストール",
	"setup_mods": "Modのダウンロード",
	"starting_game": "ゲームを起動中...",

	// versions.go
	"playing_state": "%sをプレイ中",
	"playing_details": "SabaLauncherでプレイ中",
}