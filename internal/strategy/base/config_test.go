package base

import (
	"fmt"
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNewConfig(t *testing.T) {
	var config Config

	fileData, err := os.ReadFile("config.yaml")
	if err != nil {
		fmt.Println(err)
	}

	if err := yaml.Unmarshal(fileData, &config); err != nil {
		fmt.Println(err)
	}
	fmt.Println(config)
}
