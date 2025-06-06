package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

type Info struct {
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
}

type RequestConfig struct {
	Info    *Info             `yaml:"info"`
	Method  string            `yaml:"method"`
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers"`
}

type ConfigFile struct {
	Requests []RequestConfig   `yaml:"requests"`
	Envs     map[string]string `yaml:"envs"`
}

func LoadConfigFile() *ConfigFile {
	yamlFile, err := os.ReadFile("./data/teste.reqs.yml")
	if err != nil {
		log.Printf("yamlFile.Get err #%v", err)
	}

	config := &ConfigFile{}
	yaml.Unmarshal([]byte(yamlFile), config)

	return config
}
