package conf

import (
	"os"

	"gopkg.in/yaml.v3"
)

func LoadConfigFromFile(configPath string, config *ProjectConfig) error {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(content, &config)
	if err != nil {
		return err
	}

	return nil
}
