package eden

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSupportedGuestLanguagesAndRegionalVariants(t *testing.T) {
	for _, pair := range [][2]string{
		{"zh_CN.UTF-8", "zh-CN"}, {"zh_TW.UTF-8", "zh-TW"},
		{"en_US.UTF-8", "en-US"}, {"ja_JP.UTF-8", "ja-JP"},
		{"ko_KR.UTF-8", "ko-KR"}, {"de_DE.UTF-8", "de-DE"}, {"fr_FR.UTF-8", "fr-FR"},
		{"en_GB.UTF-8", "en-GB"}, {"en_CA.UTF-8", "en-CA"}, {"zh_HK.UTF-8", "zh-HK"},
		{"de_AT.UTF-8", "de-AT"}, {"fr_CA.UTF-8", "fr-CA"},
	} {
		t.Run(pair[1], func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Locale, cfg.BrowserLanguage = pair[0], pair[1]
			if err := cfg.Validate(); err != nil {
				t.Fatal(err)
			}
			cfg.BrowserLanguage = "en-US"
			if err := cfg.Validate(); err != nil {
				t.Fatal("system and browser languages must remain independent", err)
			}
		})
	}
}

func TestUnsupportedGuestLanguagesRejectedAtConfigurationBoundary(t *testing.T) {
	for _, pair := range [][2]string{
		{"es_ES.UTF-8", "es-ES"}, {"pt_BR.UTF-8", "pt-BR"}, {"ru_RU.UTF-8", "ru-RU"},
		{"ar_SA.UTF-8", "ar-SA"}, {"fa_IR.UTF-8", "fa-IR"}, {"my_MM.UTF-8", "my-MM"},
		{"be_BY.UTF-8", "be-BY"}, {"ps_AF.UTF-8", "ps-AF"}, {"prs_AF.UTF-8", "prs-AF"},
		{"am_ET.UTF-8", "am-ET"}, {"so_SO.UTF-8", "so-SO"}, {"ti_ER.UTF-8", "ti-ER"},
		{"nl_NL.UTF-8", "nl-NL"}, {"it_IT.UTF-8", "it-IT"},
		{"", ""}, {"en_US", "en_US"}, {"en_US.UTF-8\n", "en-US,en"},
	} {
		for _, field := range []string{"locale", "browser"} {
			t.Run(pair[1]+"/"+field, func(t *testing.T) {
				cfg := DefaultConfig()
				if field == "locale" {
					cfg.Locale = pair[0]
				} else {
					cfg.BrowserLanguage = pair[1]
				}
				if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "仅支持") {
					t.Fatal("unsupported language was accepted or not explained", err)
				}
				data, err := json.Marshal(cfg)
				if err != nil {
					t.Fatal(err)
				}
				file := filepath.Join(t.TempDir(), "config.json")
				if err := os.WriteFile(file, data, 0600); err != nil {
					t.Fatal(err)
				}
				if _, err := Load(file); err == nil {
					t.Fatal("persisted unsupported language was accepted")
				}
			})
		}
	}
}
