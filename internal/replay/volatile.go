package replay

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
)

type VolatileConfig struct {
	IgnoreFields   []string
	IgnorePatterns []*regexp.Regexp
}

func DefaultVolatileConfig() *VolatileConfig {
	return &VolatileConfig{
		IgnoreFields: []string{
			"timestamp",
			"createdAt",
			"updatedAt",
			"id",
			"uuid",
			"requestId",
			"traceId",
			"spanId",
			"date",
			"time",
			"version",
		},
		IgnorePatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i).*_at$`),
			regexp.MustCompile(`(?i).*_id$`),
			regexp.MustCompile(`(?i).*timestamp.*`),
			regexp.MustCompile(`(?i).*uuid.*`),
		},
	}
}

func normalizeToInterface(jsonStr string, config *VolatileConfig) (any, error) {
	if config == nil {
		var v any
		if err := json.Unmarshal([]byte(jsonStr), &v); err != nil {
			return nil, err
		}
		return v, nil
	}

	var data any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, err
	}

	normalized := removeVolatileFields(data, config)
	return normalized, nil
}

func NormalizeJSON(jsonStr string, config *VolatileConfig) (string, error) {
	normalizedIface, err := normalizeToInterface(jsonStr, config)
	if err != nil {
		return jsonStr, err
	}

	result, err := json.Marshal(normalizedIface)
	if err != nil {
		return jsonStr, err
	}

	return string(result), nil
}

func removeVolatileFields(data any, config *VolatileConfig) any {
	switch v := data.(type) {
	case map[string]any:
		result := make(map[string]any)

		for key, value := range v {
			if shouldIgnoreField(key, config) {
				continue
			}

			result[key] = removeVolatileFields(value, config)
		}

		return result

	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = removeVolatileFields(item, config)
		}

		return result

	default:
		return v
	}
}

func shouldIgnoreField(fieldName string, config *VolatileConfig) bool {
	if config == nil {
		return false
	}

	for _, ignore := range config.IgnoreFields {
		if strings.EqualFold(fieldName, ignore) {
			return true
		}
	}

	for _, pattern := range config.IgnorePatterns {
		if pattern.MatchString(fieldName) {
			return true
		}
	}

	return false
}

func CompareWithVolatility(body1, body2 string, config *VolatileConfig) (bool, error) {
	i1, err := normalizeToInterface(body1, config)
	if err != nil {
		return false, err
	}

	i2, err := normalizeToInterface(body2, config)
	if err != nil {
		return false, err
	}

	return reflect.DeepEqual(i1, i2), nil
}

type VolatileDiff struct {
	HasDiff          bool
	VolatileOnly     bool
	StableFieldsDiff bool
	NormalizedBody1  string
	NormalizedBody2  string
	IgnoredFields    []string
}

func DetailedCompare(body1, body2 string, config *VolatileConfig) (*VolatileDiff, error) {
	if config == nil {
		config = DefaultVolatileConfig()
	}

	rawDiff := body1 != body2

	normalized1Iface, err := normalizeToInterface(body1, config)
	if err != nil {
		return nil, err
	}

	normalized2Iface, err := normalizeToInterface(body2, config)
	if err != nil {
		return nil, err
	}

	normalizedEqual := reflect.DeepEqual(normalized1Iface, normalized2Iface)

	nb1b, _ := json.Marshal(normalized1Iface)
	nb2b, _ := json.Marshal(normalized2Iface)

	diff := &VolatileDiff{
		HasDiff:          rawDiff,
		VolatileOnly:     rawDiff && !normalizedEqual,
		StableFieldsDiff: !normalizedEqual,
		NormalizedBody1:  string(nb1b),
		NormalizedBody2:  string(nb2b),
		IgnoredFields:    collectIgnoredFields(body1, body2, config),
	}

	return diff, nil
}

func collectIgnoredFields(body1, body2 string, config *VolatileConfig) []string {
	var fields []string
	seen := make(map[string]bool)

	for _, body := range []string{body1, body2} {
		var data any
		if err := json.Unmarshal([]byte(body), &data); err != nil {
			continue
		}

		collectFieldNames(data, "", config, seen, &fields)
	}

	return fields
}

func collectFieldNames(data any, prefix string, config *VolatileConfig, seen map[string]bool, fields *[]string) {
	switch v := data.(type) {
	case map[string]any:
		for key, value := range v {
			fullPath := key
			if prefix != "" {
				fullPath = prefix + "." + key
			}

			if shouldIgnoreField(key, config) && !seen[fullPath] {
				seen[fullPath] = true
				*fields = append(*fields, fullPath)
			}

			collectFieldNames(value, fullPath, config, seen, fields)
		}

	case []any:
		for _, item := range v {
			collectFieldNames(item, prefix, config, seen, fields)
		}
	}
}

func ConfigFromFlags(ignoreFields, ignorePatterns []string) *VolatileConfig {
	config := DefaultVolatileConfig()

	if len(ignoreFields) > 0 {
		config.IgnoreFields = append(config.IgnoreFields, ignoreFields...)
	}

	for _, pattern := range ignorePatterns {
		if re, err := regexp.Compile(pattern); err == nil {
			config.IgnorePatterns = append(config.IgnorePatterns, re)
		}
	}

	return config
}
