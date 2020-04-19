package config

import (
	"io/ioutil"

	"github.com/BurntSushi/toml"
)

type RedisConfig struct {
	Port     int
	DB       int
	Address  string
	Password string
	Prefix   string
}

type SwitchPortConfig struct {
	Trunk        bool
	AllowedVLANs []int
	Up           bool
}

type ControlProcessConfig struct {
	Layer      int
	Name       string
	ConfigFile string
}

type Config struct {
	Redis          RedisConfig
	SwitchPorts    map[string]SwitchPortConfig
	ControlProcess []ControlProcessConfig
}

func ReadConfig(path string) (Config, error) {
	confBin, err := ioutil.ReadFile(path)
	var config Config
	if err != nil {
		return config, err
	}
	confStr := string(confBin)
	_, err = toml.Decode(confStr, &config)
	if err != nil {
		return config, err
	}
	return config, nil
}
