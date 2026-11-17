package logging

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

// ToJSON returns the JSON string of the given value.
func ToJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

// ToPrettyJSON returns the pretty-printed JSON string of the given value.
func ToPrettyJSON(value any) string {
	data, _ := json.MarshalIndent(value, "", "  ")
	return string(data)
}

// ToYAML returns the YAML string of the given value.
func ToYAML(value any) string {
	data, _ := yaml.Marshal(value)
	return string(data)
}
