package proxy

import (
	"crypto/sha1"
	"net"

	"appconst"
	"applog"
)

//length encoded int
const LENGTH_ENCODED_ONE = 0xfb
const LENGTH_ENCODED_TWO = 0xfc
const LENGTH_ENCODED_THREE = 0xfd
const LENGTH_ENCODED_EIGHT = 0xfe

//===========Common functions===============
func GetLengthEncodedInt(bytes []byte, pos int) (intval int, newPos int) {
	intval = int(bytes[pos])
	pos++
	if intval == LENGTH_ENCODED_TWO {
		intval = ConvertBytesToInt(bytes[pos : pos+2])
		pos = pos + 2
	} else if intval == LENGTH_ENCODED_THREE {
		intval = ConvertBytesToInt(bytes[pos : pos+3])
		pos = pos + 3
	} else if intval == LENGTH_ENCODED_EIGHT {
		intval = ConvertBytesToInt(bytes[pos : pos+8])
		pos = pos + 8
	}
	newPos = pos
	return
}
func GetLengthEncodedString(bytes []byte, pos int) (str string, newPos int) {
	intval := int(bytes[pos])
	pos++
	if intval == LENGTH_ENCODED_TWO {
		intval = ConvertBytesToInt(bytes[pos : pos+2])
		pos = pos + 2
	} else if intval == LENGTH_ENCODED_THREE {
		intval = ConvertBytesToInt(bytes[pos : pos+3])
		pos = pos + 3
	} else if intval == LENGTH_ENCODED_EIGHT {
		intval = ConvertBytesToInt(bytes[pos : pos+8])
		pos = pos + 8
	}
	str = string(bytes[pos : pos+intval])
	pos = pos + intval
	newPos = pos
	return
}
func GetLengthEncodedBytes(bytes []byte, pos int) (strBytes []byte, newPos int) {
	intval := int(bytes[pos])
	pos++
	if intval == LENGTH_ENCODED_TWO {
		intval = ConvertBytesToInt(bytes[pos : pos+2])
		pos = pos + 2
	} else if intval == LENGTH_ENCODED_THREE {
		intval = ConvertBytesToInt(bytes[pos : pos+3])
		pos = pos + 3
	} else if intval == LENGTH_ENCODED_EIGHT {
		intval = ConvertBytesToInt(bytes[pos : pos+8])
		pos = pos + 8
	}
	strBytes = bytes[pos : pos+intval]
	pos = pos + intval
	newPos = pos
	return
}

func ParseOkPacket(okBytes []byte) *OkPacket {
	packetLen := ConvertBytesToInt(okBytes[0:3])
	packetNo := int(okBytes[3])
	header := int(okBytes[4])
	isEof := false

	if packetLen > 7 && header == appconst.PACKET_OK_HEADER {
		isEof = false
	} else if packetLen < 9 && header == appconst.PACKET_EOF_HEADER {
		isEof = true
	}
	pos := 5
	packet := &OkPacket{Len: packetLen, No: packetNo, IsEof: isEof}

	packet.AffectedRows, pos = GetLengthEncodedInt(okBytes, pos)
	packet.LastInsertId, pos = GetLengthEncodedInt(okBytes, pos)
	packet.StatusFlags = ConvertBytesToInt(okBytes[pos : pos+2])
	pos += 2
	packet.Warning = ConvertBytesToInt(okBytes[pos : pos+2])
	pos += 2
	packet.Info = string(okBytes[pos : packetLen+4])
	return packet
}
func ParseErrPacket(errBytes []byte) *ErrPacket {
	packetLen := ConvertBytesToInt(errBytes[0:3])
	packetNo := int(errBytes[3])
	errcode := ConvertBytesToInt(errBytes[5:7])

	pos := 7
	pos = pos + 6
	errorMessageLen := packetLen - pos + 4
	if errorMessageLen < 0 {
		errorMessageLen = 0
	}

	errMessage := string(errBytes[pos : pos+errorMessageLen])
	return &ErrPacket{Len: packetLen, No: packetNo, ErrorCode: errcode, ErrorMessage: errMessage}
}

func ConvertBytesToInt(bytes []byte) int {
	var intval int = 0
	len := len(bytes)
	for i := 0; i < len; i++ {
		intval |= int(bytes[i]) << uint(8*i)
	}
	return intval
}
func ConvertIntToBytes(val, byteSize int) []byte {
	bytes := make([]byte, byteSize)
	var uval, ui uint
	for i := 0; i < byteSize; i++ {
		uval = uint(val)
		ui = uint(i)
		bytes[i] = byte(uval >> (ui * 8) & 0xff)
	}
	return bytes
}

func GetNullString(packet []byte, pos int) (str []byte, strLen int) {
	strLen = 0
	str = make([]byte, MAX_NULL_STRING_LEN)
	for ; packet[pos] != 0; pos++ {
		str[strLen] = packet[pos]
		strLen++
	}
	return
}

func GetMysqlPassword(salt []byte, password string) []byte {
	sha1Inst := sha1.New()

	sha1Inst.Write([]byte(password))
	firstSha1 := sha1Inst.Sum(nil)

	sha1Inst.Reset()
	sha1Inst.Write(firstSha1)
	secondSha1 := sha1Inst.Sum(nil)

	secondSha1 = append(salt, secondSha1...)

	sha1Inst.Reset()
	sha1Inst.Write(secondSha1)
	thirdSha1 := sha1Inst.Sum(nil)

	var ret = make([]byte, 0)
	for key, val := range firstSha1 {
		ret = append(ret, val^thirdSha1[key])
	}

	return ret
}

func InArray(ele int, arr []int) bool {
	for _, val := range arr {
		if val == ele {
			return true
		}
	}
	return false
}

func GenMysqlPacket(buf []byte, packetNo int) []byte {
	tmp := make([]byte, 0)
	tmp = append(tmp, ConvertIntToBytes(len(buf), 3)...)
	tmp = append(tmp, byte(packetNo))
	return append(tmp, buf...)
}

//发送err packet
func SendErrPacket(conn net.Conn, errorCode int, errorMessage string, packetNo int) {
	writeBuf := make([]byte, 0)
	writeBuf = append(writeBuf, byte(0xff))
	writeBuf = append(writeBuf, ConvertIntToBytes(errorCode, 2)...)

	// writeBuf = append(writeBuf, byte(23)) //字符#
	// writeBuf = append(writeBuf, []byte{0, 0, 0, 0, 0}...)
	writeBuf = append(writeBuf, []byte(errorMessage)...)
	writeBuf = GenMysqlPacket(writeBuf, packetNo)

	conn.Write(writeBuf)
	applog.Warning(errorMessage, errorCode)
}
