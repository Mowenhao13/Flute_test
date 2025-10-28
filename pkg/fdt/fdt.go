package fdt

import "encoding/json"

type ExtFDT struct {
	FDTInstanceID uint32
	ContentType   string
	FileName      string
	// Expires time.Time

	// FecOtiMaximumSourceBlockLength uint16
	// FecOtiEncodingSymbolLength uint16
}

// Marshal 将扩展 FDT 元数据编码为 JSON 字节流，便于通过 ALC 发送。
func (f ExtFDT) Marshal() ([]byte, error) {
	return json.Marshal(f)
}
