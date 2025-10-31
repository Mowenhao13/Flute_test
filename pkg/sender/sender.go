package sender

import (
	alc "FluteTest/pkg/alc"
	fdt "FluteTest/pkg/fdt"
	fd "FluteTest/pkg/filedesc"
	lct "FluteTest/pkg/lct"
	oti "FluteTest/pkg/oti"
	"fmt"
	"math"
	"net"
	"time"

	raptorq "github.com/xssnick/raptorq"
)

type SenderConfig struct {
	FdtDuration time.Duration
	SymbolSize  uint32
	FdtStartID  uint32
}

type FileConfig struct {
	ContentType string
	FileName    string
	FilePath    string
}

type Sender struct {
	Conn         *net.UDPConn
	Fdt          fdt.ExtFDT
	TSI          uint32
	OTI          oti.Oti
	SenderConfig SenderConfig
	FileConfig   FileConfig
	RQ           raptorq.RaptorQ
	nextFdtID    uint32
	nextTSI      uint32
}

func NewSender(conn *net.UDPConn, fdtMeta *fdt.ExtFDT, TSI uint32, oti oti.Oti, fileCfg *FileConfig, sendCfg SenderConfig, rq *raptorq.RaptorQ) *Sender {
	var meta fdt.ExtFDT
	if fdtMeta != nil {
		meta = *fdtMeta
	}

	var cfg FileConfig
	if fileCfg != nil {
		cfg = *fileCfg
	}

	var encoder raptorq.RaptorQ
	if rq != nil {
		encoder = *rq
	}

	startID := sendCfg.FdtStartID
	if startID == 0 {
		startID = 1
	}

	return &Sender{
		Conn:         conn,
		Fdt:          meta,
		TSI:          TSI,
		OTI:          oti,
		SenderConfig: sendCfg,
		FileConfig:   cfg,
		RQ:           encoder,
		nextFdtID:    startID,
		nextTSI:      TSI,
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

func AddFile(s *Sender, filedesc *fd.FileDesc) {
	// 设置 senderCfg symbolSize
	s.SenderConfig.SymbolSize = uint32(s.OTI.EncodingSymbolLength)

	if filedesc.FdtID == 0 {
		filedesc.FdtID = s.nextFdtID
		s.nextFdtID++
	} else if filedesc.FdtID >= s.nextFdtID {
		s.nextFdtID = filedesc.FdtID + 1
	}

	s.TSI = s.nextTSI
	s.nextTSI++

	// 设置 sender fileCfg
	fileCfg := s.FileConfig
	fileCfg.FileName = filedesc.Name
	fileCfg.FilePath = filedesc.Path
	fileCfg.ContentType = filedesc.ContentType
	s.FileConfig = fileCfg

	// 设置 FDT
	s.Fdt.FileName = filedesc.Name
	s.Fdt.ContentType = filedesc.ContentType
	s.Fdt.FDTInstanceID = filedesc.FdtID
}

func (s *Sender) Send(fileData *[]byte) error {

	if s.Conn == nil {
		return fmt.Errorf("sender UDP connection is nil")
	}

	startTime := time.Now()
	lastTime := time.Now()

	oti := s.OTI

	// 是否进行 FEC 编码
	shouldEncode := (oti.FECEncodingID != 0)

	var cci uint8 = 0
	var tsi uint32 = s.TSI
	var toi uint32 = s.Fdt.FDTInstanceID

	fdtDur := s.SenderConfig.FdtDuration

	ChunkSize := int(s.SenderConfig.SymbolSize)
	if ChunkSize <= 0 {
		return fmt.Errorf("invalid symbol size: %d", ChunkSize)
	}
	totalChunks := uint32(math.Ceil(float64(len(*fileData)) / float64(ChunkSize)))

	sendCloseSession := false

	// Send chunks
	for i := 0; i < len(*fileData); i += ChunkSize {
		serverTime := time.Now()
		end := i + ChunkSize
		if end > len(*fileData) {
			end = len(*fileData)
		}
		chunkData := (*fileData)[i:end]
		chunkLen := uint32(len(chunkData))

		isLastChunk := (end == len(*fileData))
		closeObject := isLastChunk
		closeSession := false

		data := chunkData
		cp := uint8(0) // 0: 原始数据 No-code
		if shouldEncode {
			encodedSymbol, err := s.Encode(chunkData)
			if err != nil {
				return fmt.Errorf("encode chunk data failed: %w", err)
			}
			data = encodedSymbol
			cp = 1 // 1: 编码数据 RaptorQ
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
		packetFDT := s.Fdt

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
			FDT:             packetFDT,
		}

		// 日志输出
		fmt.Printf("Pkt param: SourceBlockNb(%d), EncodingSymbol(%d), TotalChunks(%d), TransferLength(%d), ServerTime(%v)\n",
			pkt.SourceBlockNb, pkt.EncodingSymbol, pkt.TotalChunks, pkt.TransferLength, pkt.ServerTime)

		packet := pkt.Serialize()
		if len(packet) > 65507 {
			fmt.Printf("Packet size %d exceeds UDP limit\n", len(packet))
			continue
		}

		_, err := s.Conn.Write(packet)
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
		_, err := s.Conn.Write(closePkt)
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
	if _, err := s.Conn.Write(packet); err != nil {
		return fmt.Errorf("send FDT packet failed: %w", err)
	}

	return nil
}
