package data

import (
	"encoding/binary"
	"hash/crc32"
)

// LogRecordPos 内存索引，主要是数据在磁盘上的位置,这里其实就是内存上key所连接的表项
type LogRecordPos struct {
	Fid    uint32
	Offset int64
	Size   int64 // 数据在磁盘上的大小
}

// 自定义一个变量类型
type LogRecordType = byte

// 其实这个地方发现go的特性还是非常有意思的，下面的内容似乎是会自动继承
const (
	LogRecordNormal LogRecordType = iota
	LogRecordDeleted
	LogRecordTxnFinished
)

// 写入到文件中的记录
type LogRecord struct {
	Key   []byte
	Value []byte
	Type  LogRecordType
}
type LogRecordHeader struct {
	crc        uint32        // 校验和
	recordType LogRecordType // 类型
	keySize    uint32
	valueSize  uint32
}

// 事务相关的内容
type TransactionRecord struct {
	Record *LogRecord
	Pos    *LogRecordPos
}

// EncodeLogRecord 对 LogRecord 进行编码，返回字节数组及长度
//
//	+-------------+-------------+-------------+--------------+-------------+--------------+
//	| crc 校验值  |  type 类型   |    key size |   value size |      key    |      value   |
//	+-------------+-------------+-------------+--------------+-------------+--------------+
//	    4字节          1字节        变长（最大5）   变长（最大5）     变长           变长
func EncodeLogRecord(logRecord *LogRecord) ([]byte, int64) {
	// 初始化一个 header 部分的字节数组
	header := make([]byte, maxLogRecordHeaderSize)

	// 第五个字节存储 Type
	header[4] = logRecord.Type
	var index = 5
	// 5 字节之后，存储的是 key 和 value 的长度信息
	// 使用变长类型，节省空间
	index += binary.PutVarint(header[index:], int64(len(logRecord.Key)))
	index += binary.PutVarint(header[index:], int64(len(logRecord.Value)))

	var size = index + len(logRecord.Key) + len(logRecord.Value)
	encBytes := make([]byte, size)

	// 将 header 部分的内容拷贝过来
	copy(encBytes[:index], header[:index])
	// 将 key 和 value 数据拷贝到字节数组中
	copy(encBytes[index:], logRecord.Key)
	copy(encBytes[index+len(logRecord.Key):], logRecord.Value)

	// 对整个 LogRecord 的数据进行 crc 校验
	crc := crc32.ChecksumIEEE(encBytes[4:])
	binary.LittleEndian.PutUint32(encBytes[:4], crc)

	return encBytes, int64(size)
}

// 对logRecordPos进行编码的方法
func EncodeLogRecordPos(logRecordPos *LogRecordPos) []byte {
	// 只有Fid和offset两个字段
	buf := make([]byte, binary.MaxVarintLen32*2+binary.Size(logRecordPos))
	var index = 0
	// 这个变长编码的确是需要注意的，还是需要写熟练一些
	index += binary.PutVarint(buf[index:], int64(logRecordPos.Fid))
	index += binary.PutVarint(buf[index:], logRecordPos.Offset)
	index += binary.PutVarint(buf[index:], logRecordPos.Size)
	// 只返回有效部分的数据，这个方法还是要注意的
	return buf[:index]
}

func DecodeLogRecordHeader(buf []byte) (*LogRecordHeader, int64) {
	if len(buf) <= 4 {
		return nil, 0
	}

	header := &LogRecordHeader{
		crc:        binary.LittleEndian.Uint32(buf[:4]),
		recordType: buf[4],
	}

	var index = 5
	// 取出实际的 key size
	keySize, n := binary.Varint(buf[index:])
	header.keySize = uint32(keySize)
	index += n

	// 取出实际的 value size
	valueSize, n := binary.Varint(buf[index:])
	header.valueSize = uint32(valueSize)
	index += n

	return header, int64(index)
}

// DecodeLogRecordPos 对LogRecordPos进行解码
func DecodeLogRecordPos(buf []byte) *LogRecordPos {
	var index = 0
	fileId, i := binary.Varint(buf[index:]) // 解码出第一个binary变长变量
	index += i
	offset, n := binary.Varint(buf[index:])
	index += n
	size, _ := binary.Varint(buf[index:])
	return &LogRecordPos{
		Fid:    uint32(fileId),
		Offset: offset,
		Size:   size,
	}
}

// 获得LogRecord的校验信息: 在校验的时候从第四个字节开始往后进行校验
func getLogRecordCRC(lr *LogRecord, header []byte) uint32 {
	if lr == nil {
		return 0
	}
	crc := crc32.ChecksumIEEE(header[:])
	crc = crc32.Update(crc, crc32.IEEETable, lr.Key)
	crc = crc32.Update(crc, crc32.IEEETable, lr.Value)

	return crc
}
