package appconst

const COM_SLEEP = 0x00
const COM_QUIT = 0x01
const COM_INIT_DB = 0x02
const COM_QUERY = 0x03
const COM_FIELD_LIST = 0x04
const COM_CREATE_DB = 0x05
const COM_DROP_DB = 0x06
const COM_REFRESH = 0x07
const COM_SHUTDOWN = 0x08

//包头
const PACKET_OK_HEADER = 0x00
const PACKET_EOF_HEADER = 0xfe
const PACKET_ERR_HEADER = 0xff
const PACKET_LOCAL_INFILE_REQUEST_HEADER = 0xfb
