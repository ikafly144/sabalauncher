package i18n

import (
	"fmt"
	"strings"

	"github.com/Xuanwo/go-locale"
)

var currentLanguage string = "en"
var dictionaries = make(map[string]map[string]string)

func init() {
	// Register languages
	dictionaries["en"] = dictEN
	dictionaries["ja"] = dictJA

	// Detect system language
	tag, err := locale.Detect()
	if err == nil {
		base, _ := tag.Base()
		langCode := base.String()
		if _, exists := dictionaries[langCode]; exists {
			currentLanguage = langCode
		} else if strings.HasPrefix(langCode, "ja") {
			currentLanguage = "ja"
		}
	}
}

// T translates a key into the current language, supporting fmt.Sprintf format if args are provided.
func T(key string, args ...interface{}) string {
	dict, ok := dictionaries[currentLanguage]
	if !ok {
		dict = dictionaries["en"] // fallback to English
	}

	str, ok := dict[key]
	if !ok {
		// Fallback to English if key is missing in current language
		enDict := dictionaries["en"]
		if enStr, enOk := enDict[key]; enOk {
			str = enStr
		} else {
			str = key // Fallback to key itself
		}
	}

	if len(args) > 0 {
		return fmt.Sprintf(str, args...)
	}
	return str
}

// SetLanguage forces the application to use a specific language code (e.g. "en", "ja")
func SetLanguage(lang string) {
	if _, ok := dictionaries[lang]; ok {
		currentLanguage = lang
	}
}

// GetLanguage returns the current language code
func GetLanguage() string {
	return currentLanguage
}
