package config

import (
	"applog"
	"fmt"
	"os"

	"testing"
)

func TestConfig(t *testing.T) {
	applog.SetApp("trade")
	applog.SetLevel(applog.LOG_LEVEL_DEBUG)
	applog.SetLogType(applog.LOG_TYPE_DATE)
	applog.SetLogRootDir("D:/project/go/log")

	configPath := "../config.json"
	appConfig, err := GetConfig(configPath)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if appConfig.Proxy.Port != 8888 || appConfig.Proxy.MaxConnectionNum != 1000 || appConfig.Proxy.InitConnectionNum != 10 {
		t.Error("app Config.proxy  check error.")
	}
	if appConfig.Auth.DbUsername != "root" || appConfig.Auth.DbPassword != "111" {
		t.Error("app Config.auth db  check error.")
	}
	for _, host := range appConfig.Auth.Hosts {
		if host.Master.Host != "192.168.0.1" || host.Master.Port != 8001 {
			t.Error("app Config.auth master db  check error.")
		}

		if host.Slaves[0].Host != "192.168.0.1" || host.Slaves[0].Port != 8001 {
			t.Error("app Config.auth slaves db  check error.")
		}
	}
}
