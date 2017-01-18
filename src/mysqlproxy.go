/**
 * @Author fansichi@qq.com
 * @CreateData 2016/8/31
 * @Desc mysql proxy程序主入口
 */

package main

import (
	"flag"
	"fmt"
	"os"

	"applog"
	"config"
	"proxy"
)

func main() {
	var configPath string
	var appConfig *config.AppConfig
	var connPool *proxy.ConnPool
	var mysqlProxy *proxy.MysqlProxy
	var err error

	flag.StringVar(&configPath, "config", "config.json", "the path of app config file")

	//初始化配置文件
	appConfig, err = config.GetConfig(configPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	//设置日志类
	applog.SetApp(appConfig.Proxy.AppName)
	applog.SetLevel(appConfig.Proxy.LogLevel)
	applog.SetLogType(appConfig.Proxy.LogType)
	applog.SetLogRootDir(appConfig.Proxy.LogRoot)

	//初始化连接池
	connPool = proxy.NewConnPool(appConfig)

	//得到proxy对象
	mysqlProxy = proxy.NewMysqlProxy(appConfig, connPool)
	mysqlProxy.Run()

}
