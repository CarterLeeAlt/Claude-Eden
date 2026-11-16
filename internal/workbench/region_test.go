package workbench

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/sylvinus/agent-vm/internal/eden"
)

func TestRegionSuggestionsStayWithinSupportedLanguages(t *testing.T) {
	for country := range regionLanguage {
		locale, browser := languageSuggestion(country)
		cfg := eden.DefaultConfig()
		cfg.Locale, cfg.BrowserLanguage = locale, browser
		if err := cfg.Validate(); err != nil {
			t.Fatalf("%s suggested an unsupported language: %v", country, err)
		}
	}
	for _, country := range []string{"ES", "MX", "PT", "BR", "RU", "AE", "IR", "MM", "BY", "AF", "ET", "SO", "ER", "IT", "NL", "ZZ", ""} {
		if locale, browser := languageSuggestion(country); locale != "" || browser != "" {
			t.Fatalf("%s still has a removed language suggestion", country)
		}
		cfg := eden.DefaultConfig()
		cfg.Locale, cfg.BrowserLanguage = "ko_KR.UTF-8", "ko-KR"
		got := autoRegionSettings(cfg, Preferences{AutoLanguage: true, AutoTimezone: true}, Location{CountryCode: country, Timezone: "Europe/London"})
		if got.Locale != cfg.Locale || got.BrowserLanguage != cfg.BrowserLanguage || got.Timezone != "Europe/London" {
			t.Fatal("missing language suggestion altered the selected language or prevented timezone selection")
		}
	}
	if locale, browser := languageSuggestion("KR"); locale != "ko_KR.UTF-8" || browser != "ko-KR" {
		t.Fatal("Korean suggestion missing")
	}
}

func TestSettingsRejectUnsupportedLanguagesWithoutWriting(t *testing.T) {
	s := testServer(t)
	before, err := os.ReadFile(s.configPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(s.configPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, language := range []string{"es", "pt", "ru", "ar", "it"} {
		for _, field := range []string{"locale", "browser"} {
			input := settingsView(cfg, Preferences{Keyboard: "native"})
			if field == "locale" {
				input.Locale = language + "_US.UTF-8"
			} else {
				input.BrowserLanguage = language + "-US"
			}
			body, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			for _, endpoint := range []string{"/api/settings", "/api/settings/probe"} {
				if w := request(s, "POST", endpoint, s.origin, string(body), true); w.Code != 400 {
					t.Fatalf("%s accepted unsupported %s (%d)", endpoint, field, w.Code)
				}
			}
			after, err := os.ReadFile(s.configPath)
			if err != nil || string(after) != string(before) {
				t.Fatal("invalid language replaced configuration", err)
			}
		}
	}
}

func TestAutomaticRegionSettingsPreserveManualOverrides(t *testing.T) {
	cfg := eden.DefaultConfig()
	loc := Location{CountryCode: "JP", Timezone: "Asia/Tokyo"}
	manual := autoRegionSettings(cfg, Preferences{}, loc)
	if manual != cfg {
		t.Fatal("location changed manually selected settings")
	}
	tzOnly := autoRegionSettings(cfg, Preferences{AutoTimezone: true}, loc)
	if tzOnly.Timezone != "Asia/Tokyo" || tzOnly.Locale != cfg.Locale {
		t.Fatal("timezone option changed language")
	}
	languageOnly := autoRegionSettings(cfg, Preferences{AutoLanguage: true}, loc)
	if languageOnly.Locale != "ja_JP.UTF-8" || languageOnly.BrowserLanguage != "ja-JP" || languageOnly.Timezone != cfg.Timezone {
		t.Fatal("language option did not preserve independent timezone")
	}
	unknown := autoRegionSettings(cfg, Preferences{AutoLanguage: true}, Location{CountryCode: "ZZ"})
	if unknown != cfg {
		t.Fatal("unknown region invented a language setting")
	}
}

func TestMissingExitPolicyDoesNotRelaxSavedStaticRule(t *testing.T) {
	s := testServer(t)
	old := eden.DefaultConfig()
	old.ExitPolicy = eden.ExitPolicy{Mode: "static", Scope: "country", IP: "8.8.8.8"}
	input := settingsView(old, Preferences{Keyboard: "mac"})
	input.ExitPolicy = eden.ExitPolicy{}
	got, _, err := proposedSettings(input, old, s.profile)
	if err != nil {
		t.Fatal(err)
	}
	if got.ExitPolicy != old.ExitPolicy {
		t.Fatal("legacy form silently relaxed static IP")
	}
}
