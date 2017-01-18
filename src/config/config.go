package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

type AppConfig struct {
	Proxy struct {
		Port              int `json:"port"`
		MaxConnectionNum  int `json:"max_connection_num"`
		InitConnectionNum int `json:"init_connection_num"`
		Strategy          int `json:"strategy"`

		AppName  string `json:"app_name"`
		LogRoot  string `json:"log_root"`
		LogLevel int    `json:"log_level"`
		LogType  int    `json:"log_type"`
	} `json:"proxy"`

	Auth struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		DbUsername string `json:"db_username"`
		DbPassword string `json:"db_password"`
		Hosts      []struct {
			Master struct {
				Host string `json:"host"`
				Port int    `json:"port"`
			} `json:"master"`
			Slaves []struct {
				Host string `json:"host"`
				Port int    `json:"port"`
			} `json:"slaves"`
		} `json:"hosts"`
	} `json:"auth"`
	WhiteIps []string `json:"white_ips"`
}

func GetConfig(configPath string) (appConfig *AppConfig, err error) {
	f, err1 := os.Open(configPath)
	if err1 != nil {
		appConfig = nil
		err = err1
		return
	}

	data, err2 := ioutil.ReadAll(f)

	if err2 != nil {
		appConfig = nil
		err = err2
	}
	appConfig = new(AppConfig)
	err = json.Unmarshal(data, appConfig)

	return
}
