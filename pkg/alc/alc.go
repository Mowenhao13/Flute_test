package alc

import (
	fdt "FluteTest/pkg/fdt"
	lct "FluteTest/pkg/lct"
	oti "FluteTest/pkg/oti"
	"bytes"
	"encoding/binary"
	"fmt"
	"time"
)

const (
	lctHeaderLen = 12 // 有两个保留字段
	fecIDLen     = 11 // 1(FECEncodingID) + 2(FECInstanceID) + 4(SourceBlockNb) + 4(EncodingSymbol)
	metaLen      = 10 // 元数据
	headerLen    = lctHeaderLen + fecIDLen + metaLen
)

type AlcPkt struct {
	// LCT头部 (ALC控制信息)
	LCTHeader lct.LCTHeader

	// OTI
	OTI oti.Oti

	// FEC PayloadID
	SourceBlockNb  uint32 // 源块编号
	EncodingSymbol uint32 // 编码符号ID（块内序号）

	// 元数据
	TotalChunks   uint32 // 总块数
	PayloadLength uint32 // 原始数据长度（去除编码补零）

	// FDT
	FDT fdt.ExtFDT

	// 数据载荷
	TransferLength  uint64
	EncodingSymbols []byte // 编码符号数据

	// 时间
	ServerTime time.Time
}

func (pkt *AlcPkt) Serialize() []byte {
	// LCT头部: 10字节 0-11
	lctHeader := make([]byte, lctHeaderLen)
	lctHeader[0] = (pkt.LCTHeader.Version << 4) | (pkt.LCTHeader.Flags & 0x0F)
	lctHeader[1] = pkt.LCTHeader.CCI
	// RFC 5052 要求保留，通常填充0
	lctHeader[2] = 0
	lctHeader[3] = 0
	binary.BigEndian.PutUint32(lctHeader[4:8], pkt.LCTHeader.TSI)
	binary.BigEndian.PutUint32(lctHeader[8:12], pkt.LCTHeader.TOI)

	// FEC Payload ID: 13字节 12-24
	fecID := make([]byte, fecIDLen)
	fecID[0] = pkt.OTI.FECEncodingID                              // 12
	binary.BigEndian.PutUint16(fecID[1:3], pkt.OTI.FECInstanceID) // 13 - 14
	binary.BigEndian.PutUint32(fecID[3:7], pkt.SourceBlockNb)     // 15 - 18
	binary.BigEndian.PutUint32(fecID[7:11], pkt.EncodingSymbol)   // 19 - 22

	// 自定义元数据: 总块数（共6字节）
	meta := make([]byte, metaLen)
	binary.BigEndian.PutUint32(meta[0:4], pkt.TotalChunks)
	binary.BigEndian.PutUint32(meta[4:8], pkt.PayloadLength)

	// FDT 纳入元数据中
	fdtbuf := new(bytes.Buffer)
	if err := binary.Write(fdtbuf, binary.BigEndian, pkt.FDT.FDTInstanceID); err != nil {
		panic(fmt.Errorf("failed to serialize FDTInstanceID: %w", err))
	}
	contentTypeLen := len(pkt.FDT.ContentType)
	if err := binary.Write(fdtbuf, binary.BigEndian, uint16(contentTypeLen)); err != nil {
		panic(fmt.Errorf("failed to serialize ContentType length: %w", err))
	}
	if contentTypeLen > 0 {
		if _, err := fdtbuf.WriteString(pkt.FDT.ContentType); err != nil {
			panic(fmt.Errorf("failed to write ContentType: %w", err))
		}
	}
	fileNameLen := len(pkt.FDT.FileName)
	if err := binary.Write(fdtbuf, binary.BigEndian, uint16(fileNameLen)); err != nil {
		panic(fmt.Errorf("failed to serialize FileName length: %w", err))
	}
	if fileNameLen > 0 {
		if _, err := fdtbuf.WriteString(pkt.FDT.FileName); err != nil {
			panic(fmt.Errorf("failed to write FileName: %w", err))
		}
	}
	fdtbytes := fdtbuf.Bytes()
	if len(fdtbytes) > 0xFFFF {
		panic("FDT section too large (max 65535 bytes)")
	}
	binary.BigEndian.PutUint16(meta[8:10], uint16(len(fdtbytes)))

	fdtOffset := headerLen
	payloadOffset := fdtOffset + len(fdtbytes)
	packet := make([]byte, payloadOffset+len(pkt.EncodingSymbols))
	copy(packet[0:lctHeaderLen], lctHeader)
	copy(packet[lctHeaderLen:lctHeaderLen+fecIDLen], fecID)
	copy(packet[lctHeaderLen+fecIDLen:headerLen], meta)
	if len(fdtbytes) > 0 {
		copy(packet[fdtOffset:payloadOffset], fdtbytes)
	}
	if pkt.EncodingSymbols != nil {
		// 处理编码符号数据
		copy(packet[payloadOffset:], pkt.EncodingSymbols)
	}

	return packet
}

func ParseAlcPkt(data []byte) (*AlcPkt, error) {
	if len(data) < lctHeaderLen {
		return nil, fmt.Errorf("数据包过小: %d", len(data))
	}

	pkt := &AlcPkt{}

	// 解析LCT头部
	pkt.LCTHeader.Version = data[0] >> 4
	pkt.LCTHeader.Flags = data[0] & 0x0F
	pkt.LCTHeader.CloseObject = pkt.LCTHeader.Flags&0x04 != 0
	pkt.LCTHeader.CloseSession = pkt.LCTHeader.Flags&0x02 != 0
	pkt.LCTHeader.CCI = data[1]
	pkt.LCTHeader.TSI = binary.BigEndian.Uint32(data[4:8])
	pkt.LCTHeader.TOI = binary.BigEndian.Uint32(data[8:12])

	if len(data) < lctHeaderLen+fecIDLen {
		if pkt.LCTHeader.CloseSession {
			return pkt, nil
		}
		return nil, fmt.Errorf("数据包缺少 FEC PayloadID: %d", len(data))
	}

	// 解析OTI (3字节)
	pkt.OTI.FECEncodingID = data[12]
	pkt.OTI.FECInstanceID = binary.BigEndian.Uint16(data[13:15])

	// 解析 FEC PayloadID
	pkt.SourceBlockNb = binary.BigEndian.Uint32(data[15:19])
	pkt.EncodingSymbol = binary.BigEndian.Uint32(data[19:23])

	metaOffset := lctHeaderLen + fecIDLen
	if len(data) < metaOffset+metaLen {
		if pkt.LCTHeader.CloseSession {
			return pkt, nil
		}
		return nil, fmt.Errorf("数据包缺少元数据: %d", len(data))
	}

	pkt.TotalChunks = binary.BigEndian.Uint32(data[metaOffset : metaOffset+4])
	pkt.PayloadLength = binary.BigEndian.Uint32(data[metaOffset+4 : metaOffset+8])

	// 解析FDT
	fdtLen := binary.BigEndian.Uint16(data[metaOffset+8 : metaOffset+metaLen])
	payloadOffset := headerLen + int(fdtLen)
	if len(data) < payloadOffset {
		return nil, fmt.Errorf("FDT 数据不完整，期望长度 %d 实际 %d", payloadOffset, len(data))
	}

	if fdtLen > 0 {
		fdtBytes := data[headerLen:payloadOffset]
		fdtInfo, err := unmarshalFDT(fdtBytes)
		if err != nil {
			return nil, err
		}
		pkt.FDT = *fdtInfo
	}

	// 是否传输实际数据
	if len(data) > payloadOffset {
		pkt.EncodingSymbols = make([]byte, len(data)-payloadOffset)
		copy(pkt.EncodingSymbols, data[payloadOffset:])
	}

	return pkt, nil
}

func unmarshalFDT(data []byte) (*fdt.ExtFDT, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("FDT 数据过短: %d", len(data))
	}

	fdtInstanceID := binary.BigEndian.Uint32(data[:4])
	contentTypeLen := binary.BigEndian.Uint16(data[4:6])
	cursor := 6
	expectedLen := cursor + int(contentTypeLen)
	if len(data) < expectedLen+2 {
		return nil, fmt.Errorf("FDT 数据不完整，缺少 ContentType 或 FileName 长度信息: %d", len(data))
	}

	info := &fdt.ExtFDT{FDTInstanceID: fdtInstanceID}
	if contentTypeLen > 0 {
		info.ContentType = string(data[cursor:expectedLen])
	}

	cursor = expectedLen
	fileNameLen := binary.BigEndian.Uint16(data[cursor : cursor+2])
	cursor += 2
	end := cursor + int(fileNameLen)
	if len(data) < end {
		return nil, fmt.Errorf("FDT FileName 数据不完整，期望 %d 实际 %d", end, len(data))
	}
	if fileNameLen > 0 {
		info.FileName = string(data[cursor:end])
	}

	return info, nil
}

func NewAlcPkt(
	oti oti.Oti,
	cci uint8,
	tsi uint32,
	alcPkt *AlcPkt,
	serverTime time.Time,
	sourceBlockNb uint32,
) ([]byte, error) {
	data := make([]byte, 0, 1500)

	lctHeader := lct.LCTHeader{
		Version:      1,
		Flags:        CalculateFlags(alcPkt.LCTHeader.CloseObject, alcPkt.LCTHeader.CloseSession, false),
		CCI:          cci,
		TSI:          tsi,
		TOI:          alcPkt.LCTHeader.TOI,
		CloseObject:  alcPkt.LCTHeader.CloseObject,
		CloseSession: false,
		CodePoint:    oti.FECEncodingID,
	}
	lcth := lct.NewLCTHeader(lctHeader)
	data = append(data, lcth...)

	if alcPkt.LCTHeader.Flags&0x08 != 0 {
		timestamp := serverTime.Unix()
		data = binary.BigEndian.AppendUint64(data, uint64(timestamp))
	}

	fecPayloadID := make([]byte, 4)
	binary.BigEndian.PutUint32(fecPayloadID, sourceBlockNb) // 直接使用参数
	data = append(data, fecPayloadID...)

	data = append(data, alcPkt.EncodingSymbols...)
	return data, nil
}

func NewAlcPktCloseSession(
	oti oti.Oti, // 传入 OTI 配置
	cci uint8,
	tsi uint32,
	sourceBlockNb uint32, // 可选参数，默认 0
) []byte {
	data := make([]byte, 0)

	// 动态计算 Flags（CloseSession=true, 其他 false）
	flags := CalculateFlags(false, true, false)

	// 构造 LCT 头部
	lctHeader := lct.LCTHeader{
		Version:      1,
		Flags:        flags,
		CCI:          cci,
		TSI:          tsi,
		TOI:          0, // TOI=0 表示无关联对象
		CloseObject:  false,
		CloseSession: true,
		CodePoint:    oti.FECEncodingID, // 使用传入的 FEC 编码 ID
	}
	data = append(data, lct.NewLCTHeader(lctHeader)...)

	// 添加 FEC Payload ID（允许外部控制）
	fecPayloadID := make([]byte, 4)
	binary.BigEndian.PutUint32(fecPayloadID, sourceBlockNb)
	data = append(data, fecPayloadID...)

	return data
}

func CalculateFlags(closeObject, closeSession, senderCurrentTime bool) uint8 {
	var flags uint8

	// S 位（Bit 3）：Sender Current Time
	if senderCurrentTime {
		flags |= 0b0000_1000 // 设置第 3 位
	}

	// B 位（Bit 2）：Close Object
	if closeObject {
		flags |= 0b0000_0100 // 设置第 2 位
	}

	// A 位（Bit 1）：Close Session
	if closeSession {
		flags |= 0b0000_0010 // 设置第 1 位
	}

	// Bit 0（H）固定为 0（忽略 Half-word 模式）
	return flags
}
