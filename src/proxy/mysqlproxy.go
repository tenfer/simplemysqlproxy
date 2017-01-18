package proxy

import (
	"fmt"
	"net"
	"os"
	"strconv"

	"appconst"
	"applog"
	"config"
)

type MysqlServer struct {
	Username  string
	Password  string
	Host      string
	Port      int
	UsedCount int
	IdleCount int
	IsMaster  bool
	IsSlave   bool
}

type ProxyConn struct {
	TcpConn net.Conn
	Db      string //连接的db
}

type MysqlProxy struct {
	Config       *config.AppConfig      //全局配置文件
	MysqlServers map[string]MysqlServer //mysql
	Pool         *ConnPool
	ConnNum      int //连接数量
}

func NewMysqlProxy(config *config.AppConfig, pool *ConnPool) *MysqlProxy {
	mysqlProxy := new(MysqlProxy)
	mysqlProxy.Config = config
	mysqlProxy.Pool = pool
	return mysqlProxy
}

/*得到监听器*/
func (proxy *MysqlProxy) Listener() net.Listener {
	addr := ":" + strconv.Itoa(proxy.Config.Proxy.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		message := fmt.Sprint(err.Error())
		fmt.Println(message)
		applog.Fatal(message, appconst.ERROR_PROXY_LISTEN_FAIL)
		os.Exit(1)
	}
	return listener
}

func (proxy *MysqlProxy) Run() {
	applog.Trace("mysql proxy started.", appconst.ERROR_OK)
	listener := proxy.Listener()
	for {
		conn, err := listener.Accept()
		if err != nil {
			message := fmt.Sprintf("accept error. clientInfo=%s\n", conn.RemoteAddr().String())
			applog.Warning(message, appconst.ERROR_PROXY_ACCEPT_FAIL)
			continue
		}
		//处理连接
		proxyConn := new(ProxyConn)
		proxyConn.TcpConn = conn
		go proxy.handleClientRequest(proxyConn)
	}
}

//处理客户端请求
func (proxy *MysqlProxy) handleClientRequest(conn *ProxyConn) {
	proxy.ConnNum++

	authOk := proxy.clientAuth(conn)
	if !authOk {
		return
	}

	applog.Trace("login ok", appconst.ERROR_OK)

	var mysqlConn *MysqlConn = nil
	mysqlConn = proxy.checkMysqlConn(conn, mysqlConn)

	//处理用户请求
	for {
		readBuf, err := proxy.ReadAll(conn)
		if err != nil {
			return
		}
		len := len(readBuf)

		//得到客户端命令
		command := new(Command)
		command.ComId = int(readBuf[4])
		if len > 5 {
			command.ComStr = string(readBuf[5:])
		}
		//切换数据库操作
		if command.ComId == appconst.COM_INIT_DB {
			conn.Db = command.ComStr
		}

		//每次都做一次检查？很影响性能，待优化 todo
		mysqlConn = proxy.checkMysqlConn(conn, mysqlConn)

		//正常退出情况
		if command.ComId == appconst.COM_QUIT {
			applog.Notice("Client Quit", appconst.ERROR_OK)
			//把连接返回到连接池
			proxy.Pool.AddMysqlConn(mysqlConn)
			return
		}

		//todo=超时退出的情况

		var packets []byte

		packets, err = mysqlConn.TransferPacket(command)
		var writeSize int
		writeSize, err = conn.TcpConn.Write(packets)
		if err != nil {
			SendErrPacket(conn.TcpConn, appconst.ERROR_COMMAND_WRITE_ERROR, err.Error(), 0)
		}
		applog.DebugPrintln("write:", writeSize)
	}

	// length := len(packets)
	// if writeSize != length {
	// 	SendErrPacket(conn, appconst.ERROR_COMMAND_WRITE_ERROR, "command write error.", 0)
	// }

}

//proxy 认证
func (proxy *MysqlProxy) clientAuth(conn *ProxyConn) bool {
	var test int
	salt1 := "%#@ga()!"
	salt2 := "&*-=$uewmHQn"
	//构造init packet
	writeBuf := make([]byte, 0)
	writeBuf = append(writeBuf, byte(0x0a))
	writeBuf = append(writeBuf, []byte("mysqlproxy1.0")...)
	writeBuf = append(writeBuf, byte(0x00))
	writeBuf = append(writeBuf, []byte{0x00, 0x00, 0x00, 0x00}...)
	writeBuf = append(writeBuf, []byte(salt1)...)
	writeBuf = append(writeBuf, byte(0x00))
	cap := 0x800ff7ff
	capablitity := ConvertIntToBytes(cap, 4)
	writeBuf = append(writeBuf, capablitity[:2]...)
	writeBuf = append(writeBuf, byte(33))
	writeBuf = append(writeBuf, ConvertIntToBytes(0x0002, 2)...)
	writeBuf = append(writeBuf, capablitity[2:]...)
	if test = cap & CLIENT_PLUGIN_AUTH; test > 0 {
		writeBuf = append(writeBuf, byte(21))
	} else {
		writeBuf = append(writeBuf, byte(0))
	}
	writeBuf = append(writeBuf, ConvertIntToBytes(0, 10)...)
	if test = cap & CLIENT_SECURE_CONNECTION; test > 0 {
		writeBuf = append(writeBuf, []byte(salt2)...)
		writeBuf = append(writeBuf, byte(0))
	}
	if test = cap & CLIENT_PLUGIN_AUTH; test > 0 {
		writeBuf = append(writeBuf, []byte("mysql_native_password")...)
		writeBuf = append(writeBuf, byte(0))
	}
	writeBuf = GenMysqlPacket(writeBuf, 0)
	//end 构造init packet
	//发送init packet
	_, err := conn.TcpConn.Write(writeBuf)
	applog.DebugPrintln(writeBuf)
	if err != nil {
		errorMessage := err.Error()
		SendErrPacket(conn.TcpConn, appconst.ERROR_WRITE_INITIAL_PACKET, errorMessage, 0)
		return false
	}
	//读取response packet
	readBuf, err1 := proxy.ReadAll(conn)
	applog.DebugPrintln(readBuf)
	if err1 != nil {
		errorMessage := err1.Error()
		SendErrPacket(conn.TcpConn, appconst.ERROR_READ_CLIENT_RESPONSE, errorMessage, 2)
		return false
	}
	packetNo := int(readBuf[3]) + 1
	//解析response packet
	pos := 4
	clientCap := ConvertBytesToInt(readBuf[pos : pos+4])
	pos += 4
	pos += 4 //skip max-packet size
	pos++    //skip character set
	pos += 23

	byteUsername, strLen := GetNullString(readBuf, pos)
	username := string(byteUsername[:strLen])
	pos += strLen + 1
	var password []byte
	var passwordLen int
	if test = clientCap & CLIENT_SECURE_CONNECTION; test > 0 {
		passwordLen = int(readBuf[pos])
		if passwordLen == 0 {
			passwordLen = 20
		}
		pos++
		password = readBuf[pos : pos+passwordLen]
		pos += passwordLen
	} else {
		errorMessage := "client must support secure connection"
		SendErrPacket(conn.TcpConn, appconst.ERROR_MUST_SECURE_CONNECTION, errorMessage, packetNo)
		return false
	}

	if test = clientCap & CLIENT_CONNECT_WITH_DB; test > 0 {
		databaseB, databaseLen := GetNullString(readBuf, pos)
		pos += databaseLen + 1
		conn.Db = string(databaseB[:databaseLen])
	}
	applog.DebugPrintln(username, proxy.Config.Auth.DbUsername)
	//验证用户名
	if username != proxy.Config.Auth.DbUsername {
		errorMessage := "username wrong"
		SendErrPacket(conn.TcpConn, appconst.ERROR_CLIENT_USERANME_WRONG, errorMessage, packetNo)
		return false
	}
	salt := salt1 + salt2
	applog.DebugPrintln(salt, proxy.Config.Auth.DbPassword)
	configPassword := GetMysqlPassword([]byte(salt), proxy.Config.Auth.DbPassword)
	applog.DebugPrintln(password, configPassword)
	if string(password) != string(configPassword) {
		errorMessage := "password wrong"
		SendErrPacket(conn.TcpConn, appconst.ERROR_CLIENT_PASSWORD_WRONG, errorMessage, packetNo)
		return false
	}

	//发送OK packet
	writeBuf = make([]byte, 0)
	writeBuf = append(writeBuf, byte(0x00))
	writeBuf = append(writeBuf, byte(0x00))
	writeBuf = append(writeBuf, byte(0x00))
	if test = clientCap & CLIENT_PROTOCOL_41; test > 0 {
		writeBuf = append(writeBuf, ConvertIntToBytes(0x0002, 2)...)
		writeBuf = append(writeBuf, ConvertIntToBytes(0, 2)...)
	}
	writeBuf = append(writeBuf, []byte("login ok.")...)
	writeBuf = GenMysqlPacket(writeBuf, packetNo)
	conn.TcpConn.Write(writeBuf)
	return true
}

//只适用读取单个包的情况，客户端的命令是单个包
func (proxy *MysqlProxy) ReadAll(conn *ProxyConn) (b []byte, err error) {
	tmp := make([]byte, BUF_SIZE)
	b = make([]byte, 0)
	readSize := 0
	for {
		tmpSize, err1 := conn.TcpConn.Read(tmp)
		if err1 != nil {
			err = err1
			b = nil
			return
		}
		packetLen := ConvertBytesToInt(tmp[0:3])
		for i := 0; i < tmpSize; i++ {
			b = append(b, tmp[i])
			readSize++
		}
		//单个包的情况
		if readSize >= packetLen+4 {
			break
		}
	}
	return
}

//检查mysql链接，并最终返回一个有效的链接
func (proxy *MysqlProxy) checkMysqlConn(conn *ProxyConn, mysqlConn *MysqlConn) *MysqlConn {
	if mysqlConn != nil && mysqlConn.Ping() {
		return mysqlConn
	}
	//从连接池中得到db连接，目前不支持多个主库
	mysqlConn = proxy.Pool.Get(true, 0)
	//执行数据库切换操作
	if conn.Db != "" {
		command := new(Command)
		command.ComId = appconst.COM_INIT_DB
		command.ComStr = conn.Db
		mysqlConn.TransferPacket(command)
	}
	return mysqlConn
}
