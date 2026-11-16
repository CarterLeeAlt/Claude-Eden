package workbench

import "github.com/sylvinus/agent-vm/internal/eden"

// These are product defaults for common regions, not a claim that a country
// has one language. Unlisted regions retain the user's applied language.
var regionLanguage = map[string]string{
	"US": "en", "GB": "en", "CA": "en", "AU": "en", "NZ": "en", "IE": "en", "SG": "en", "IN": "en", "PH": "en", "ZA": "en",
	"JP": "ja", "KR": "ko", "CN": "zh", "TW": "zh", "HK": "zh",
	"DE": "de", "AT": "de", "CH": "de", "FR": "fr",
}

func languageSuggestion(country string) (locale, browser string) {
	if language := regionLanguage[country]; language != "" {
		return language + "_" + country + ".UTF-8", language + "-" + country
	}
	return "", ""
}

func autoRegionSettings(cfg eden.Config, preferences Preferences, location Location) eden.Config {
	if preferences.AutoTimezone && location.Timezone != "" {
		cfg.Timezone = location.Timezone
	}
	if preferences.AutoLanguage {
		if locale, browser := languageSuggestion(location.CountryCode); locale != "" {
			cfg.Locale, cfg.BrowserLanguage = locale, browser
		}
	}
	return cfg
}
