package lct

import "encoding/binary"

type LCTHeader struct {
	Version uint8 
	Flags uint8
	CCI uint8 
	TSI uint32
	TOI uint32
	CloseObject bool 
	CloseSession bool
	CodePoint uint8
}


func NewLCTHeader(h LCTHeader) []byte {
	data := make([]byte, 0, 12) // 预分配足够空间

	// 组合 Version 和 Flags
	versionAndFlags := (h.Version << 4) | (h.Flags & 0x0F)
	data = append(data, versionAndFlags) // 1  byte

	// 写入 CCI（假设 CCI 是 uint8）
	data = append(data, h.CCI) // 1 byte

	// 写入 TSI（大端序，4字节）
	tsiBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(tsiBytes, h.TSI) // 4 byte
	data = append(data, tsiBytes...)

	// 写入 TOI（大端序，4字节）
	toiBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(toiBytes, h.TOI) // 4 byte
	data = append(data, toiBytes...)

	// CloseObject 和 CloseSession 无需写入
	
	return data
}


