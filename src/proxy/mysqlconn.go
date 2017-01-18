package proxy

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	"appconst"
	"applog"
)

const BUF_SIZE = 1024
const MAX_NULL_STRING_LEN = 100
const CLIENT_PLUGIN_AUTH = 0x00080000
const CLIENT_SECURE_CONNECTION = 0x00008000
const CLIENT_CONNECT_WITH_DB = 0x00000008
const CLIENT_PROTOCOL_41 = 0x00000200
const CLIENT_DEPRECATE_EOF = 0x01000000

type OkPacket struct {
	Len          int
	No           int
	IsEof        bool
	AffectedRows int
	LastInsertId int
	StatusFlags  int
	Warning      int
	Info         string
}
type ErrPacket struct {
	Len          int
	No           int
	ErrorCode    int
	ErrorMessage string
}
type Command struct {
	ComId  int
	ComStr string
}

type MysqlConn struct {
	Username string
	Password string
	Host     string
	Port     int
	Database string

	Used    bool
	TcpConn net.Conn

	AffectedRows int
	InsertId     int
	ServerStatus int
	Capability   int
}

func NewMysqlConn() *MysqlConn {
	conn := new(MysqlConn)
	conn.Used = false
	return conn
}

//连接mysql
func (conn *MysqlConn) Connect() error {
	var errStr string
	var test, readSize int
	var err error
	var readBuf, writeBuf []byte

	addr := conn.Host + ":" + strconv.Itoa(conn.Port)
	conn.TcpConn, err = net.Dial("tcp", addr)
	if err != nil {
		return err
	}

	readSize, err = conn.ReadAll(appconst.COM_QUIT, &readBuf)
	if err != nil {
		return err
	}
	if readSize == 0 {
		errStr = "没有收到服务器的initial packet."
		return errors.New(errStr)
	}

	//packetLen := ConvertBytesToInt(readBuf[0:3])
	packetNo := ConvertBytesToInt(readBuf[3:4])

	serverVersion, serverVersionLen := GetNullString(readBuf, 5)
	serverVersion = serverVersion[:serverVersionLen]
	applog.DebugPrintln("server version :", string(serverVersion), serverVersion, serverVersionLen)

	pos := 5 + serverVersionLen + 1
	connectionId := ConvertBytesToInt(readBuf[pos : pos+4])
	applog.DebugPrintln("raw:", readBuf[pos:pos+4], "  connectionId :", connectionId)
	pos = pos + 4
	salt := readBuf[pos : pos+8]
	pos = pos + 9

	capability := readBuf[pos : pos+2]
	pos = pos + 2

	applog.DebugPrintln("default character set : ", readBuf[pos:pos+1])
	pos++
	pos = pos + 2
	capability = append(capability, readBuf[pos:pos+2]...)

	serverCap := ConvertBytesToInt(capability)

	applog.DebugPrintf("server capability:%x\n", serverCap)

	pos = pos + 2 + 1 + 10

	if test = serverCap & CLIENT_SECURE_CONNECTION; test > 0 {
		//12位字符
		salt = append(salt, readBuf[pos:pos+12]...)
	}
	applog.DebugPrintln("salt:", salt)
	//start 构造往服务器发送的认证数据
	var clientCap int
	if conn.Database == "" {
		clientCap = 0x000ea281
	} else {
		clientCap = 0x000ea289
	}

	//得到双方都兼容的capablity
	clientCap = clientCap & serverCap
	conn.Capability = clientCap
	applog.DebugPrintf("client capability:%x\n", clientCap)

	writeBuf = append(writeBuf, ConvertIntToBytes(clientCap, 4)...)

	packetSize := 0xffffff00
	writeBuf = append(writeBuf, ConvertIntToBytes(packetSize, 4)...)
	writeBuf = append(writeBuf, byte(0x21))
	writeBuf = append(writeBuf, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}...)

	writeBuf = append(writeBuf, []byte(conn.Username)...)
	writeBuf = append(writeBuf, byte(0))

	if test = clientCap & CLIENT_SECURE_CONNECTION; test > 0 {
		writeBuf = append(writeBuf, byte(20))

		mysqlPassword := GetMysqlPassword(salt, conn.Password)
		writeBuf = append(writeBuf, mysqlPassword...)
	} else {
		errStr = "[error]Not Supported！Must Support protocol 41"
		applog.DebugPrintln(errStr)
		return errors.New(errStr)
	}

	if test = clientCap & CLIENT_CONNECT_WITH_DB; test > 0 {
		writeBuf = append(writeBuf, []byte(conn.Database)...)
		writeBuf = append(writeBuf, byte(0))
	}
	if test = clientCap & CLIENT_PLUGIN_AUTH; test > 0 {
		writeBuf = append(writeBuf, []byte("mysql_native_password")...)
		writeBuf = append(writeBuf, byte(0))
	}

	retPacketLen := len(writeBuf)
	retPacketNo := packetNo + 1

	tmp := append(ConvertIntToBytes(retPacketLen, 3), byte(retPacketNo))
	writeBuf = append(tmp, writeBuf...)
	applog.DebugPrintln(writeBuf)
	//end 构造往服务器发送的认证数据

	//发送认证数据
	writeSize, err1 := conn.TcpConn.Write(writeBuf)
	if err1 != nil {
		return err1
	}
	if writeSize != retPacketLen+4 {
		errStr = "[error]send response packet failed"
		applog.DebugPrintln(errStr)
		return errors.New(errStr)
	}

	applog.Trace("[proxy=>mysqlserver]send login response", appconst.ERROR_OK)

	//读取服务端的返回包
	readSize, err = conn.ReadAll(appconst.COM_QUIT, &readBuf)
	if readBuf[4] == 0 {
		//OK Packet
		applog.Trace("[mysqlserver=>proxy] login successfully", appconst.ERROR_OK)
		return nil
	} else if readBuf[4] == 0xff {
		errPacket := ParseErrPacket(readBuf)
		applog.Warning("[mysqlserver=>proxy] login failed", appconst.ERROR_AUTH_FAIL)
		return errors.New(errPacket.ErrorMessage)
	}
	return nil
}

//读取所有数据
func (conn *MysqlConn) ReadAll(comId int, b *[]byte) (readSize int, err error) {
	pos, tmpPos, findEOF := 0, 0, 0
	*b = make([]byte, 0)
	readSize = 0
	for {
		tmp := make([]byte, BUF_SIZE)
		tmpSize, err1 := conn.TcpConn.Read(tmp)

		if err1 != nil {
			err = err1
			return
		}
		if tmpSize == 0 {
			break
		}

		for i := 0; i < tmpSize; i++ {
			*b = append(*b, tmp[i])
			readSize++
		}

		responseType := int((*b)[4])
		responseTypes := []int{appconst.PACKET_OK_HEADER, appconst.PACKET_ERR_HEADER, appconst.PACKET_LOCAL_INFILE_REQUEST_HEADER}
		//COM_QUERY的情况
		if comId == appconst.COM_QUERY && !InArray(responseType, responseTypes) {
			for {
				//保证获取包长度不能出错
				if (pos + 3) > readSize {
					break
				}
				packetLen := ConvertBytesToInt((*b)[pos : pos+3])
				tmpPos = pos + 4 + packetLen
				//保证读取一个完整的包
				if tmpPos > readSize {
					break
				}

				header := int((*b)[pos+4])
				if header == appconst.PACKET_EOF_HEADER && packetLen < 9 {
					findEOF++
					if findEOF == 2 {
						goto RET
					}
				} else if header == appconst.PACKET_ERR_HEADER {
					//出错的情况
					goto RET
				}
				//准备读取下个包
				pos = tmpPos
			}
			//列定义的情况
		} else if comId == appconst.COM_FIELD_LIST && responseType != appconst.PACKET_ERR_HEADER {
			for {
				//保证获取包长度不能出错
				if (pos + 3) > readSize {
					break
				}
				packetLen := ConvertBytesToInt((*b)[pos : pos+3])
				tmpPos = pos + 4 + packetLen
				//保证读取一个完整的包
				if tmpPos > readSize {
					break
				}

				header := int((*b)[pos+4])
				if header == appconst.PACKET_EOF_HEADER && packetLen < 9 {
					goto RET
				} else if header == appconst.PACKET_ERR_HEADER {
					//出错的情况
					goto RET
				}
				//准备读取下个包
				pos = tmpPos
			}
		} else { //server 只返回单个包的情况
			packetLen := ConvertBytesToInt((*b)[0:3])
			//单个包的情况
			if readSize >= packetLen+4 {
				break
			}
		}
	}
RET:
	return
}

//执行命令，并且返回mysql server 返回的字节流
func (conn *MysqlConn) TransferPacket(command *Command) ([]byte, error) {

	var writeSize, readSize int
	var err error
	var writeBuf, readBuf []byte

	writeBuf = make([]byte, 0)
	readBuf = make([]byte, 0)

	writeBuf = append(writeBuf, byte(command.ComId))
	if len(command.ComStr) > 0 {
		writeBuf = append(writeBuf, []byte(command.ComStr)...)
	}

	writeBuf = GenMysqlPacket(writeBuf, 0)

	logstr := fmt.Sprintf("[proxy=>mysqlserver] comId=%d comStr=%s rawBytes=%x", command.ComId, command.ComStr, writeBuf[5:])
	applog.Trace(logstr, appconst.ERROR_OK)
	writeSize, err = conn.TcpConn.Write(writeBuf)
	if err != nil {
		return nil, err
	}
	if writeSize == 0 {
		return nil, errors.New("write error.")
	}

	readSize, err = conn.ReadAll(command.ComId, &readBuf)
	applog.DebugPrintln("server response=", readBuf)

	if err != nil {
		return nil, err
	}
	if readSize == 0 {
		return nil, errors.New("read error.")
	}
	return readBuf, nil
}

func (conn *MysqlConn) Close() {
	conn.Used = false
}
func (conn *MysqlConn) RealClose() error {
	var writeBuf []byte
	writeBuf = make([]byte, 0)
	writeBuf = append(writeBuf, []byte{0x01, 0x00, 0x00, 0x00, appconst.COM_QUIT}...)
	conn.TcpConn.Write(writeBuf)

	return conn.TcpConn.Close()
}
func (conn *MysqlConn) Ping() bool {
	command := new(Command)
	command.ComId = 0x0e
	bytes, err := conn.TransferPacket(command)
	if err != nil {
		applog.DebugPrintln(err)
		return false
	} else if len(bytes) > 0 && int(bytes[4]) == appconst.PACKET_OK_HEADER {
		return true
	}
	return false
}
