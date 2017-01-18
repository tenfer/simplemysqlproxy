package proxy

import (
	"config"
	"fmt"
	"math/rand"
	"strconv"

	"appconst"
	"applog"
)

const SLAVE_STRATEGY_RANDOM = 1
const SLAVE_STRATEGY_LOOP = 2

type ConnPool struct {
	config      *config.AppConfig
	pool        map[string][]*MysqlConn //存储空闲连接
	lastSlaveNo int
}

func NewConnPool(config *config.AppConfig) *ConnPool {
	connPool := new(ConnPool)
	connPool.pool = make(map[string][]*MysqlConn, 0)
	connPool.config = config
	connPool.lastSlaveNo = -1
	return connPool
}

//从连接池中得到一个连接，得到的连接一定有效吗？
func (connPool *ConnPool) Get(isMaster bool, indexNo int) *MysqlConn {
	var host string
	var port int
	if isMaster {
		host = connPool.config.Auth.Hosts[indexNo].Master.Host
		port = connPool.config.Auth.Hosts[indexNo].Master.Port
	} else {
		slaveNo := connPool.getSlaveNo(indexNo)
		host = connPool.config.Auth.Hosts[indexNo].Slaves[slaveNo].Host
		port = connPool.config.Auth.Hosts[indexNo].Slaves[slaveNo].Port
	}
	//查找空闲的连接
	addr := connPool.getAddr(host, port)
	mysqlConn, pos := connPool.findIdleConn(connPool.pool[addr])
	if mysqlConn == nil {
		mysqlConn = connPool.getNewMysqlConn(host, port)
	} else {
		connPool.pool[addr] = append(connPool.pool[addr][:pos], connPool.pool[addr][pos+1:]...)
		//如果链接失效了，重新得到一个新的链接
		if !mysqlConn.Ping() {
			applog.Trace("connection expire.reconnect...", appconst.ERROR_OK)
			mysqlConn = connPool.getNewMysqlConn(host, port)
		}
	}
	return mysqlConn
}

//把用完的mysql连接,放回连接池
func (connPool *ConnPool) AddMysqlConn(mysqlConn *MysqlConn) {
	addr := connPool.getAddr(mysqlConn.Host, mysqlConn.Port)
	connPool.pool[addr] = append(connPool.pool[addr], mysqlConn)
	mysqlConn.Close()
}

//得到新的mysql连接
func (connPool *ConnPool) getNewMysqlConn(host string, port int) *MysqlConn {
	mysqlConn := NewMysqlConn()
	mysqlConn.Username = connPool.config.Auth.DbUsername
	mysqlConn.Password = connPool.config.Auth.DbPassword
	mysqlConn.Host = host
	mysqlConn.Port = port
	err := mysqlConn.Connect()
	if err != nil {
		//写日志
		message := fmt.Sprintf("connect mysql server error. errInfo={host=%s port=%d username=%s password=%s}\n", host, port, mysqlConn.Username, mysqlConn.Password)
		applog.Fatal(message, appconst.ERROR_CONNECT_MYSQL_FAIL)
		return nil
	}
	addr := connPool.getAddr(host, port)

	conns := connPool.pool[addr]
	if conns == nil {
		conns = make([]*MysqlConn, 0)
	}
	mysqlConn.Used = true
	return mysqlConn
}

//查找空闲的mysql连接
func (connPool *ConnPool) findIdleConn(mysqlConns []*MysqlConn) (*MysqlConn, int) {
	for key, conn := range mysqlConns {
		if !conn.Used {
			conn.Used = true
			return conn, key
		}
	}
	return nil, -1
}

func (connPool *ConnPool) getSlaveNo(indexNo int) int {
	strategyType := connPool.config.Proxy.Strategy
	slaveNum := len(connPool.config.Auth.Hosts[indexNo].Slaves)
	if strategyType == SLAVE_STRATEGY_LOOP {
		connPool.lastSlaveNo++
		return connPool.lastSlaveNo % slaveNum
	} else {
		return rand.Intn(slaveNum)
	}
}

func (connPool *ConnPool) getAddr(host string, port int) string {
	return host + ":" + strconv.Itoa(port)
}
