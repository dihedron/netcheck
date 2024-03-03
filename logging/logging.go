package logging

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

func ToJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func ToPrettyJSON(value any) string {
	data, _ := json.MarshalIndent(value, "", "  ")
	return string(data)
}

func ToYAML(value any) string {
	data, _ := yaml.Marshal(value)
	return string(data)
}
