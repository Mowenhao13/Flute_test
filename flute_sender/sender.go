// package main

// import (
// 	// alc "FluteTest/pkg/alc"
// 	"encoding/binary"
// 	"fmt"
// 	"net"
// 	"os"
// 	"time"
// )

// const (
// 	maxUDPPayload = 65507
// 	alcHeaderSize = 12 + 13 // LCT(12) + FEC Payload ID(13)
// 	chunkSize     = 10240
// )

// func main() {
// 	socket, err := net.DialUDP("udp", nil, &net.UDPAddr{
// 		IP:   net.IPv4(192, 168, 1, 102),
// 		Port: 3400,
// 	})
// 	if err != nil {
// 		fmt.Println("连接UDP服务器失败，err:", err)
// 		return
// 	}
// 	defer socket.Close()

// 	filePath := "./send_files/test_1024mb.bin"
// 	fileData, err := os.ReadFile(filePath)
// 	if err != nil {
// 		fmt.Println("读取文件失败，err:", err)
// 		return
// 	}

// 	if chunkSize <= 0 {
// 		fmt.Println("配置的ALC头部超出UDP负载能力")
// 		return
// 	}

// 	totalChunks := (len(fileData) + chunkSize - 1) / chunkSize // 计算总块数
// 	start := time.Now()

// 	for i := 0; i < len(fileData); i += chunkSize {
// 		end := i + chunkSize
// 		if end > len(fileData) {
// 			end = len(fileData)
// 		}
// 		chunk := fileData[i:end]

// 		// 创建数据包： [序号 (4字节)] + [总块数 (4字节)] + [数据]
// 		seqNum := int32(i / chunkSize)
// 		packet := make([]byte, 8+len(chunk))
// 		binary.BigEndian.PutUint32(packet[0:4], uint32(seqNum))
// 		binary.BigEndian.PutUint32(packet[4:8], uint32(totalChunks))
// 		copy(packet[8:], chunk)

// 		_, err = socket.Write(packet)
// 		if err != nil {
// 			fmt.Println("发送文件块失败: ", err)
// 			return
// 		}
// 		fmt.Printf("发送块 %d/%d\n", seqNum+1, totalChunks)
// 	}

// 	// for i := 0; i < len(fileData); i += chunkSize {
// 	// 	end := i + chunkSize
// 	// 	if end > len(fileData) {
// 	// 		end = len(fileData)
// 	// 	}
// 	// 	chunk := fileData[i:end]

// 	// 	seqNum := uint32(i / chunkSize)
// 	// 	pkt := &alc.AlcPkt{
// 	// 		Version:         1,
// 	// 		Flags:           0,
// 	// 		CongestionCtrl:  0,
// 	// 		TSI:             12345,
// 	// 		TOI:             1,
// 	// 		FECEncodingID:   0,
// 	// 		FECInstanceID:   0,
// 	// 		SourceBlockNb:   seqNum,
// 	// 		EncodingSymbol:  uint32(totalChunks),
// 	// 		EncodingSymbols: chunk,
// 	// 	}

// 	// 	packet := pkt.Serialize()
// 	// 	if len(packet) > maxUDPPayload {
// 	// 		fmt.Printf("序号 %d 的ALC包过大: %d 字节\n", seqNum, len(packet))
// 	// 		return
// 	// 	}

// 	// 	if _, err := socket.Write(packet); err != nil {
// 	// 		fmt.Println("发送文件块失败: ", err)
// 	// 		return
// 	// 	}
// 	// 	fmt.Printf("发送块 %d/%d\n", seqNum+1, totalChunks)
// 	// }

// 	elapsed := time.Since(start)
// 	fmt.Printf("发送文件成功，耗时: %v\n", elapsed)
// }

package main

import (
	fdt "FluteTest/pkg/fdt"
	oti "FluteTest/pkg/oti"
	"FluteTest/pkg/sender"
	raptorq "github.com/xssnick/raptorq"
	ep "FluteTest/pkg/udpendpoint"
	"fmt"
	"math/rand"

	// "path/filepath"
	"time"
)

const (
	SourceAddr   = "192.168.1.102"
	DestAddr     = "192.168.1.103"
	Port         = 3400
	MaxRetries   = 3
)

const (
	EncodingSymbolLength    uint16 = 10240
	MaximumSourceBlockLength uint16 = 60
	ChunkSize                int    = int(EncodingSymbolLength) // 10KB per chunk
)

const (
	FilePath    = "./send_files/a.jpg"
	ContentType = "application/octet-stream"
	FileName    = "a.jpg"
)

const (
	FdtDuration     = 1 * time.Second // 发送FDT的时间间隔
	FdtCarouselMode  uint8 = 0 // 0: 单次发送, 1: 轮播发送
	FdtStartID       uint32 = 1
	ToiMaxLength     uint64 = 1 << 24 - 1
)


func main() {
	// // Setup UDP connection
	// targetAddr := &net.UDPAddr{
	// 	IP:   net.ParseIP(ServerIP),
	// 	Port: ServerPort,
	// }
	// conn, err := net.DialUDP("udp", nil, targetAddr)
	// if err != nil {
	// 	fmt.Println("Connection failed:", err)
	// 	return
	// }
	// defer conn.Close()

	// // Read file to send
	// filePath := "./send_files/test_1024mb.bin"
	// fileData, err := os.ReadFile(filePath)
	// if err != nil {
	// 	fmt.Println("Read file failed:", err)
	// 	return
	// }

	// totalChunks := uint32(math.Ceil(float64(len(fileData)) / float64(ChunkSize)))
	// startTime := time.Now()


	// o := oti.NewNoCode(EncodingSymbolLength, MaximumSourceBlockLength)

	// // 生成 TSI
	// rand.Seed(time.Now().UnixNano())

	// var cci uint8 = 0
	// var tsi uint32 = rand.Uint32()%0xFFFFFFFE + 1 // 确保 TSI 不为 0
	// var toi uint32 = 1


	
	// // Send chunks
	// for i := 0; i < len(fileData); i += ChunkSize {
	// 	end := i + ChunkSize
	// 	if end > len(fileData) {
	// 		end = len(fileData)
	// 	}
	// 	chunk := fileData[i:end]

	// 	isLastChunk := (end == len(fileData))
	// 	closeObject := isLastChunk
	// 	closeSession := isLastChunk

	// 	lcth := lct.LCTHeader{
	// 		Version:      1,
	// 		Flags:        alc.CalculateFlags(false, false, true),
	// 		CCI:          cci, // 无速率控制
	// 		TSI:          tsi, // 单文件传输
	// 		TOI:          toi, // 单对象
	// 		CloseObject:  closeObject,
	// 		CloseSession: closeSession,
	// 		CodePoint:    0, // 直接传输原始数据
	// 	}

	// 	// 日志输出
	// 	fmt.Printf("LCT Header param: Flags(%d), CCI(%d), TSI(%d), TOI(%d)\n", lcth.Flags, lcth.CCI, lcth.TSI, lcth.TOI)

	// 	serverTime := time.Now()

	// 	// FDT
	// 	fdt := fdt.ExtFDT{
	// 		FDTInstanceID: 1,
	// 		ContentType:   "application/octet-stream",
	// 		FileName:      "test_1024mb.bin",
	// 		// FecOtiMaximumSourceBlockLength: MaximumSourceBlockLength,
	// 		// FecOtiEncodingSymbolLength: EncodingSymbolLength,
	// 	}
	// 	// 日志输出
	// 	fmt.Printf("FDT param: FDTInstanceID(%d)\n", fdt.FDTInstanceID)

	// 	// Create ALC packet
	// 	pkt := &alc.AlcPkt{
	// 		LCTHeader:       lcth,
	// 		OTI:             o,
	// 		SourceBlockNb:   uint32(i / ChunkSize),
	// 		EncodingSymbol:  uint32(totalChunks),
	// 		EncodingSymbols: chunk,
	// 		TotalChunks:     uint32(totalChunks),
	// 		TransferLength:  uint64(len(chunk)),
	// 		ServerTime:      serverTime,
	// 		FDT:             fdt,
	// 	}
	// 	// 日志输出
	// 	fmt.Printf("Pkt param: SourceBlockNb(%d), EncodingSymbol(%d), TotalChunks(%d), TransferLength(%d), ServerTime(%v)\n",
	// 		pkt.SourceBlockNb, pkt.EncodingSymbol, pkt.TotalChunks, pkt.TransferLength, pkt.ServerTime)

	// 	packet := pkt.Serialize()
	// 	if len(packet) > 65507 {
	// 		fmt.Printf("Packet too large (%d bytes)\n", len(packet))
	// 		continue
	// 	}

	// 	_, err := conn.Write(packet)
	// 	if err != nil {
	// 		fmt.Printf("Send failed: %v\n", err)
	// 		return
	// 	}
	// 	// sent := false
	// 	// for retry := 0; retry < MaxRetries; retry++ {
	// 	// 	if _, err := conn.Write(packet); err != nil {
	// 	// 		fmt.Printf("Send failed (attempt %d): %v\n", retry+1, err)
	// 	// 		time.Sleep(50 * time.Millisecond)
	// 	// 		continue
	// 	// 	}
	// 	// 	sent = true
	// 	// 	break
	// 	// }

	// 	// if !sent {
	// 	// 	fmt.Printf("Failed to send chunk %d after %d attempts\n", i/ChunkSize, MaxRetries)
	// 	// 	return
	// 	// }

	// 	fmt.Printf("Sent chunk %d/%d (%d bytes)\n", i/ChunkSize+1, totalChunks, len(chunk))
	// }

	// // 结束会话
	// closePkt := alc.NewAlcPktCloseSession(o, cci, tsi, 0)
	// _, cerr := conn.Write(closePkt)
	// if cerr != nil {
	// 	fmt.Printf("Failed to send close packet: %v\n", cerr)
	// }

	// fmt.Printf("Transfer completed in %v\n", time.Since(startTime))

	ep := ep.Endpoint {
		SourceAddr: SourceAddr,
		DestAddr:   DestAddr,
		Port:       Port,
	}

	fileCfg := sender.FileConfig {
		FilePath:    FilePath,
		ContentType: ContentType,
		FileName:    FileName,
	}

	sendCfg := sender.SenderConfig {
		FdtDuration:    FdtDuration,
		FdtCarouselMode: FdtCarouselMode,
		FdtStartID:     FdtStartID,
		ToiMaxLength:   ToiMaxLength,
	}

	fdt := fdt.ExtFDT {
		FDTInstanceID: 1,
		ContentType:   fileCfg.ContentType,
		FileName:      fileCfg.FileName,
	}

	// oti := oti.NewNoCode(EncodingSymbolLength, MaximumSourceBlockLength)
	oti := oti.NewRaptorQ(EncodingSymbolLength)
	tsi := rand.Uint32()%0xFFFFFFFE + 1 // 确保 TSI 不为 0

	rq := raptorq.NewRaptorQ(uint32(EncodingSymbolLength))

	s := sender.NewSender(ep, fdt, tsi, oti, fileCfg, sendCfg, rq)

	err := s.Send()
	if err != nil {
		fmt.Println("Send file failed:", err)
		return
	}
}
