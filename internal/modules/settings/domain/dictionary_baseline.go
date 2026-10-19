package domain

import "time"

// StatusDictionaryCode identifies the installation-baseline status dictionary.
// The usecase layer protects this dictionary from deletion or re-coding, so
// its identity and baseline content live together here.
const StatusDictionaryCode = "status"

// StatusDictionaryBaseline returns the status dictionary required by a fresh
// installation: the enabled/disabled labels used across administration pages.
func StatusDictionaryBaseline(now time.Time) (Dictionary, error) {
	enabled, err := RestoreDictionaryItem(0, 0, "启用", "enabled", "", 10, true, 0, "", nil)
	if err != nil {
		return Dictionary{}, err
	}
	disabled, err := RestoreDictionaryItem(0, 0, "禁用", "disabled", "", 20, true, 0, "", nil)
	if err != nil {
		return Dictionary{}, err
	}
	return RestoreDictionary(0, StatusDictionaryCode, "状态", []DictionaryItem{enabled, disabled}, now, now)
}
