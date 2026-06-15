package i18n

var dictEN = map[string]string{
	"app_title": "SabaLauncher",

	// auth.go
	"welcome_title":        "Welcome to SabaLauncher",
	"login_prompt":         "Please login to continue",
	"login_browser":        "Login with Microsoft (Browser)",
	"login_device":         "Login with Microsoft (Device Code)",
	"logging_in":           "Logging in...",
	"open_browser_btn":     "Open Login Page in Browser",
	"copy_code_btn":        "Copy Code",
	"device_code_step1":    "1. Open: %s",
	"device_code_step2":    "2. Enter code: %s",
	"browser_login_prompt": "Please complete the login in your browser.",
	"logged_in_as":         "Logged in as:",
	"go_to_dashboard":      "Go to Dashboard",
	"logout":               "Logout",
	"auth_error_title":     "Authentication Error",
	"retry_browser":        "Retry Login (Browser)",
	"retry_device":         "Retry Login (Device Code)",
	"default_login_error":  "Something went wrong during login.",
	"minecraft_not_owned":  "Minecraft was not found on this account. Please make sure you are logged in with an account that owns Minecraft.",

	// dashboard.go
	"tab_launcher":                    "Launcher",
	"tab_settings":                    "Settings",
	"error_prefix":                    "Error: %s",
	"import_modpack":                  "Import Modpack",
	"register_remote":                 "Register Remote",
	"version_label":                   "Version: %s",
	"playtime_label":                  "Playtime: %s",
	"unknown_version":                 "Unknown Version",
	"play_btn":                        "PLAY",
	"normal_play":                     "Normal Play",
	"quick_launch_multiplayer_label":  "Multiplayer (Quick Launch)",
	"quick_launch_singleplayer_label": "Singleplayer (Quick Launch)",
	"update_btn":                      "Update",
	"repair_btn":                      "Repair",
	"delete_instance_btn":             "Delete Instance",
	"delete_instance_confirm_title":   "Delete Instance",
	"delete_instance_confirm_body":    "Are you sure you want to delete %s?",
	"select_instance_prompt":          "Select an instance to see details",
	"settings_title":                  "Settings",
	"actions_btn":                     "Actions",
	"account_section_title":           "Account",
	"launcher_section_title":          "Launcher Settings",
	"max_memory_label":                "Max Memory (MB)",
	"memory_limit_title":              "Memory Allocation Limited",
	"memory_limit_body":               "To ensure system stability, memory allocation has been limited to 80%% of physical memory.\nRequested: %d MB -> Capped: %d MB",
	"username_label":                  "Username: %s",
	"uuid_label":                      "UUID: %s",

	// launch overlay
	"preparing": "Preparing...",
	"stop_btn":  "STOP",

	// import.go
	"importing_progress":    "Importing...",
	"cancel":                "Cancel",
	"yes":                   "Yes",
	"no":                    "No",
	"registering_progress":  "Registering...",
	"register_remote_title": "Register Remote Modpack",
	"register_btn":          "Register",
	"manifest_url_label":    "Manifest URL",
	"updating_progress":     "Updating...",
	"repairing_progress":    "Repairing...",
	"downloading_update":    "Downloading Update",

	// updater.go
	"update_available_title":  "Update Available",
	"update_available_header": "A new version (%s) is available. Would you like to update now?",
	"update_available_body":   "A new version (%s) is available.\nWould you like to update now?\n\nRelease Notes:\n%s",

	// instance_setup.go & setup_state.go
	"setup_instance_name": "Setup %s",
	"setup_java":          "Setup Environment",
	"setup_client":        "Download Client",
	"setup_assets":        "Download Assets",
	"setup_library":       "Download Libraries",
	"setup_forge":         "Setup Forge",
	"install_forge":       "Install Forge",
	"setup_modloader":     "Install Mod Loader",
	"setup_mods":          "Download Mods",
	"starting_game":       "Starting Game...",

	// versions.go
	"playing_state":   "Playing %s",
	"playing_details": "Playing Minecraft",
}
