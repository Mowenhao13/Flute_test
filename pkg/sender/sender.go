package sender

import (
	alc "FluteTest/pkg/alc"
	fdt "FluteTest/pkg/fdt"
	lct "FluteTest/pkg/lct"
	oti "FluteTest/pkg/oti"
	endpoint "FluteTest/pkg/udpendpoint"
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	"time"

	raptorq "github.com/xssnick/raptorq"
)

type SenderConfig struct {
	FdtDuration     time.Duration
	FdtCarouselMode uint8
	FdtStartID      uint32
	ToiMaxLength    uint64
	SymbolSize      uint32
}

type FileConfig struct {
	ContentType string
	FileName    string
	FilePath    string
}

type Sender struct {
	EndPoint     endpoint.Endpoint
	Fdt          fdt.ExtFDT
	TSI          uint32
	OTI          oti.Oti
	SenderConfig SenderConfig
	FileConfig   FileConfig
	RQ           raptorq.RaptorQ
}

func NewSender(ep endpoint.Endpoint, fdt fdt.ExtFDT, TSI uint32, oti oti.Oti, fileCfg FileConfig, sendCfg SenderConfig, rq *raptorq.RaptorQ) *Sender {
	return &Sender{
		EndPoint:     ep,
		Fdt:          fdt,
		TSI:          TSI,
		OTI:          oti,
		RQ:           *rq,
		FileConfig:   fileCfg,
		SenderConfig: sendCfg,
	}
}

func (s *Sender) Encode(data []byte) ([]byte, error) {
	blockEncoder, err := s.RQ.CreateEncoder(data)
	if err != nil {
		return nil, fmt.Errorf("create RaptorQ encoder failed: %w", err)
	}

	raw := blockEncoder.GenSymbol(0)

	return raw, nil
}

func (s *Sender) Send() error {
	destIP := net.ParseIP(s.EndPoint.DestAddr)
	if destIP == nil {
		return fmt.Errorf("invalid destination IP: %s", s.EndPoint.DestAddr)
	}
	remoteAddr := &net.UDPAddr{IP: destIP, Port: s.EndPoint.Port}

	var localAddr *net.UDPAddr
	if s.EndPoint.SourceAddr != "" {
		ip := net.ParseIP(s.EndPoint.SourceAddr)
		if ip == nil {
			return fmt.Errorf("invalid source IP: %s", s.EndPoint.SourceAddr)
		}
		localAddr = &net.UDPAddr{IP: ip}
	}

	conn, err := net.DialUDP("udp", localAddr, remoteAddr)
	if err != nil {
		return fmt.Errorf("dial UDP failed: %w", err)
	}
	defer conn.Close()

	fileData, err := os.ReadFile(s.FileConfig.FilePath)
	if err != nil {
		fmt.Println("Read file failed:", err)
		return err
	}

	startTime := time.Now()
	lastTime := time.Now()

	oti := s.OTI

	shouldEncode := (oti.FECEncodingID != 0)

	// 生成 TSI
	rand.Seed(time.Now().UnixNano())
	var cci uint8 = 0
	var tsi uint32 = s.TSI
	var toi uint32 = 1

	fdtDur := s.SenderConfig.FdtDuration

	ChunkSize := int(oti.EncodingSymbolLength)
	totalChunks := uint32(math.Ceil(float64(len(fileData)) / float64(ChunkSize)))

	sendCloseSession := false

	// Send chunks
	for i := 0; i < len(fileData); i += ChunkSize {
		serverTime := time.Now()
		end := i + ChunkSize
		if end > len(fileData) {
			end = len(fileData)
		}
		chunkData := fileData[i:end]
		chunkLen := uint32(len(chunkData))

		isLastChunk := (end == len(fileData))
		closeObject := isLastChunk
		closeSession := false

		data := chunkData
		cp := uint8(0)
		if shouldEncode {
			encodedSymbol, err := s.Encode(chunkData)
			if err != nil {
				return fmt.Errorf("encode chunk data failed: %w", err)
			}
			data = encodedSymbol
			cp = 1
		}

		lcth := lct.LCTHeader{
			Version:      1,
			Flags:        alc.CalculateFlags(closeObject, closeSession, true),
			CCI:          cci, // 无速率控制
			TSI:          tsi,
			TOI:          toi,
			CloseObject:  closeObject,
			CloseSession: closeSession,
			CodePoint:    cp, // 直接传输原始数据或编码数据
		}

		fmt.Printf("LCT Header param: Flags(%d), CCI(%d), TSI(%d), TOI(%d)\n", lcth.Flags, lcth.CCI, lcth.TSI, lcth.TOI)

		// FDT
		fdt := fdt.ExtFDT{
			FDTInstanceID: 1,
			ContentType:   s.FileConfig.ContentType,
			FileName:      s.FileConfig.FileName,
		}

		// Create ALC packet
		pkt := &alc.AlcPkt{
			LCTHeader:       lcth,
			OTI:             oti,
			SourceBlockNb:   uint32(i / ChunkSize),
			EncodingSymbol:  uint32(totalChunks),
			EncodingSymbols: data,
			TotalChunks:     uint32(totalChunks),
			PayloadLength:   chunkLen,
			TransferLength:  uint64(len(data)),
			ServerTime:      serverTime,
			FDT:             fdt,
		}

		// 日志输出
		fmt.Printf("Pkt param: SourceBlockNb(%d), EncodingSymbol(%d), TotalChunks(%d), TransferLength(%d), ServerTime(%v)\n",
			pkt.SourceBlockNb, pkt.EncodingSymbol, pkt.TotalChunks, pkt.TransferLength, pkt.ServerTime)

		packet := pkt.Serialize()
		if len(packet) > 65507 {
			fmt.Printf("Packet size %d exceeds UDP limit\n", len(packet))
			continue
		}

		_, err := conn.Write(packet)
		if err != nil {
			fmt.Println("Write to UDP failed:", err)
			return err
		}

		if fdtDur > 0 && serverTime.Sub(lastTime) >= fdtDur {
			s.SendFDT()
		}

		lastTime = time.Now()
	}

	if sendCloseSession {
		closePkt := alc.NewAlcPktCloseSession(oti, cci, tsi, toi)
		_, err := conn.Write(closePkt)
		if err != nil {
			fmt.Println("Write close packet to UDP failed:", err)
			return err
		}
	}

	timeSpent := time.Since(startTime)
	fmt.Printf("File %s sent in %v\n", s.FileConfig.FilePath, timeSpent)
	return nil
}

func (s *Sender) SendFDT() error {
	payload, err := s.Fdt.Marshal()
	if err != nil {
		return fmt.Errorf("marshal FDT failed: %w", err)
	}

	destIP := net.ParseIP(s.EndPoint.DestAddr)
	if destIP == nil {
		return fmt.Errorf("invalid destination IP: %s", s.EndPoint.DestAddr)
	}
	remoteAddr := &net.UDPAddr{IP: destIP, Port: s.EndPoint.Port}

	var localAddr *net.UDPAddr
	if s.EndPoint.SourceAddr != "" {
		ip := net.ParseIP(s.EndPoint.SourceAddr)
		if ip == nil {
			return fmt.Errorf("invalid source IP: %s", s.EndPoint.SourceAddr)
		}
		localAddr = &net.UDPAddr{IP: ip}
	}

	conn, err := net.DialUDP("udp", localAddr, remoteAddr)
	if err != nil {
		return fmt.Errorf("dial UDP failed: %w", err)
	}
	defer conn.Close()

	fdtAlcPkt := alc.AlcPkt{
		LCTHeader: lct.LCTHeader{
			Version:      1,
			Flags:        alc.CalculateFlags(true, false, true),
			CCI:          0,
			TSI:          s.TSI,
			TOI:          0,
			CloseObject:  true,
			CloseSession: false,
			CodePoint:    s.OTI.FECEncodingID,
		},
		OTI:             s.OTI,
		SourceBlockNb:   0,
		EncodingSymbol:  1,
		TotalChunks:     1,
		TransferLength:  uint64(len(payload)),
		EncodingSymbols: payload,
		ServerTime:      time.Now(),
		FDT:             s.Fdt,
	}

	packet := fdtAlcPkt.Serialize()
	if _, err := conn.Write(packet); err != nil {
		return fmt.Errorf("send FDT packet failed: %w", err)
	}

	return nil
}
