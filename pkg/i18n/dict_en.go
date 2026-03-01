package i18n

var dictEN = map[string]string{
	"app_title": "SabaLauncher",
	
	// auth.go
	"welcome_title": "Welcome to SabaLauncher",
	"login_prompt": "Please login to continue",
	"login_browser": "Login with Microsoft (Browser)",
	"login_device": "Login with Microsoft (Device Code)",
	"logging_in": "Logging in...",
	"open_browser_btn": "Open Login Page in Browser",
	"copy_code_btn": "Copy Code",
	"device_code_step1": "1. Open: %s",
	"device_code_step2": "2. Enter code: %s",
	"browser_login_prompt": "Please complete the login in your browser.",
	"logged_in_as": "Logged in as:",
	"go_to_dashboard": "Go to Dashboard",
	"logout": "Logout",
	"auth_error_title": "Authentication Error",
	"retry_browser": "Retry Login (Browser)",
	"retry_device": "Retry Login (Device Code)",
	"default_login_error": "Something went wrong during login.",

	// dashboard.go
	"tab_launcher": "Launcher",
	"tab_settings": "Settings",
	"error_prefix": "Error: %s",
	"import_modpack": "Import Modpack",
	"register_remote": "Register Remote",
	"version_label": "Version: %s",
	"unknown_version": "Unknown Version",
	"play_btn": "PLAY",
	"update_btn": "Update",
	"delete_instance_btn": "Delete Instance",
	"delete_instance_confirm_title": "Delete Instance",
	"delete_instance_confirm_body": "Are you sure you want to delete %s?",
	"select_instance_prompt": "Select an instance to see details",
	"settings_title": "Settings",
	"account_section_title": "Account",
	"username_label": "Username: %s",
	"uuid_label": "UUID: %s",

	// launch overlay
	"preparing": "Preparing...",
	"stop_btn": "STOP",

	// import.go
	"importing_progress": "Importing...",
	"cancel": "Cancel",
	"registering_progress": "Registering...",
	"register_remote_title": "Register Remote Modpack",
	"register_btn": "Register",
	"manifest_url_label": "Manifest URL",
	"updating_progress": "Updating...",
	"downloading_update": "Downloading Update",

	// updater.go
	"update_available_title": "Update Available",
	"update_available_body": "A new version (%s) is available.\nWould you like to update now?\n\nRelease Notes:\n%s",

	// instance_setup.go & setup_state.go
	"setup_instance_name": "Setup %s",
	"setup_java": "Setup Environment",
	"setup_client": "Download Client",
	"setup_assets": "Download Assets",
	"setup_library": "Download Libraries",
	"setup_forge": "Setup Forge",
	"install_forge": "Install Forge",
	"setup_modloader": "Install Mod Loader",
	"setup_mods": "Download Mods",
	"starting_game": "Starting Game...",

	// versions.go
	"playing_state": "Playing %s",
	"playing_details": "Playing Minecraft",
}