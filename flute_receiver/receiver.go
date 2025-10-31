package main

import (
	alc "FluteTest/pkg/alc"
	utils "FluteTest/pkg/utils"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"gopkg.in/yaml.v3"
)

// const (
// 	EnableStaticARP   = true
// 	ReceiverIP        = "192.168.1.103" // 接收端IP地址
// 	ReceiverInterface = "eth0"
// 	Port              = 3400
// 	FileSaveDir       = "./received_files"
// 	SenderIP          = "192.168.1.102"
// 	SenderMac         = "00:11:22:33:44:55" // 发送端实际MAC地址
// )

type receiverStaticARP struct {
	Enable    bool   `yaml:"enable"`
	Interface string `yaml:"interface"`
	PeerIP    string `yaml:"peer_ip"`
	PeerMAC   string `yaml:"peer_mac"`
}

type receiverNetwork struct {
	ListenIP string `yaml:"listen_ip"`
	Port     int    `yaml:"port"`
}

type storage struct {
	SaveDir string `yaml:"save_dir"`
}
type receiverAppConfig struct {
	StaticARP receiverStaticARP `yaml:"static_arp"`
	Network   receiverNetwork   `yaml:"network"`
	Storage   storage           `yaml:"storage"`
}

type fileBuffer struct {
	TOI         uint32
	TotalChunks uint32
	Chunks      map[uint32][]byte
	FileName    string
	ContentType string
	closeObject bool
}

func newFileBuffer(toi uint32) *fileBuffer {
	return &fileBuffer{
		TOI:    toi,
		Chunks: make(map[uint32][]byte),
	}
}

type receiveQueue struct {
	order []uint32
	files map[uint32]*fileBuffer
}

func newReceiveQueue() *receiveQueue {
	return &receiveQueue{
		order: make([]uint32, 0),
		files: make(map[uint32]*fileBuffer),
	}
}

func loadReceiverConfig(path string) (*receiverAppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg receiverAppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func main() {
	cfg, err := loadReceiverConfig("./config/receiverCfg.yaml")
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		return
	}
	if err := utils.EnsureStaticARP(cfg.StaticARP.Enable, cfg.StaticARP.PeerIP, cfg.StaticARP.PeerMAC, cfg.StaticARP.Interface, "receiver"); err != nil {
		fmt.Printf("static ARP setup failed: %v\n", err)
	}

	// Setup UDP listener
	listenAddr := &net.UDPAddr{
		IP:   net.ParseIP(cfg.StaticARP.PeerIP),
		Port: cfg.Network.Port,
	}
	listen, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		fmt.Println("Listen failed:", err)
		return
	}
	defer listen.Close()

	// Prepare file storage
	if err := os.MkdirAll(cfg.Storage.SaveDir, 0755); err != nil {
		fmt.Printf("Failed to create directory: %v\n", err)
		return
	}

	queue := newReceiveQueue()
	buf := make([]byte, 65507) // Max UDP packet size

	for {
		n, addr, err := listen.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("Read error:", err)
			continue
		}

		// Parse ALC packet
		pkt, err := alc.ParseAlcPkt(buf[:n])
		if err != nil {
			fmt.Printf("Parse error: %v\n", err)
			continue
		}

		if pkt.LCTHeader.CloseSession {
			fmt.Println("Received close session packet")
			queue.flushAll(cfg.Storage.SaveDir)
			break
		}

		if len(pkt.EncodingSymbols) == 0 {
			fmt.Printf("Skip empty payload packet from %v\n", addr)
			continue
		}

		fb := queue.getOrCreate(pkt)

		stored, storedLen, err := fb.storeChunk(pkt)
		if err != nil {
			fmt.Printf("Failed to store chunk: %v\n", err)
			continue
		}
		if !stored {
			fmt.Printf("Duplicate chunk %d for TOI %d from %v, ignoring\n", pkt.SourceBlockNb, pkt.LCTHeader.TOI, addr)
			queue.flushReady(cfg.Storage.SaveDir)
			continue
		}

		totalChunks := fb.TotalChunks
		if totalChunks == 0 {
			fmt.Printf("Received chunk %d for TOI %d from %v (stored=%d bytes, total unknown)\n", pkt.SourceBlockNb+1, pkt.LCTHeader.TOI, addr, storedLen)
		} else {
			fmt.Printf("Received chunk %d/%d for TOI %d from %v (stored=%d bytes)\n", pkt.SourceBlockNb+1, totalChunks, pkt.LCTHeader.TOI, addr, storedLen)
		}
		fmt.Printf("CloseObject: %v, chunks stored: %d/%d\n", pkt.LCTHeader.CloseObject, len(fb.Chunks), fb.TotalChunks)

		queue.flushReady(cfg.Storage.SaveDir)
	}

}

func (q *receiveQueue) getOrCreate(pkt *alc.AlcPkt) *fileBuffer {
	toi := pkt.LCTHeader.TOI
	if fb, ok := q.files[toi]; ok {
		fb.applyMetadata(pkt)
		return fb
	}

	fb := newFileBuffer(toi)
	q.files[toi] = fb
	q.order = append(q.order, toi)
	fb.applyMetadata(pkt)
	return fb
}

func (q *receiveQueue) flushReady(saveDir string) {
	for len(q.order) > 0 {
		toi := q.order[0]
		fb := q.files[toi]
		if fb == nil {
			q.order = q.order[1:]
			continue
		}

		if !fb.isComplete() {
			break
		}

		if err := fb.save(saveDir); err != nil {
			fmt.Printf("Failed to finalize file (TOI=%d): %v\n", fb.TOI, err)
		}

		delete(q.files, toi)
		q.order = q.order[1:]
	}
}

func (q *receiveQueue) flushAll(saveDir string) {
	q.flushReady(saveDir)

	for len(q.order) > 0 {
		toi := q.order[0]
		fb := q.files[toi]
		if fb != nil && len(fb.Chunks) > 0 {
			fmt.Printf("File (TOI=%d) incomplete: received %d/%d chunks\n", fb.TOI, len(fb.Chunks), fb.TotalChunks)
		}
		delete(q.files, toi)
		q.order = q.order[1:]
	}
}

func (fb *fileBuffer) applyMetadata(pkt *alc.AlcPkt) {
	if pkt.TotalChunks > fb.TotalChunks {
		fb.TotalChunks = pkt.TotalChunks
	}
	if fb.FileName == "" && pkt.FDT.FileName != "" {
		fb.FileName = pkt.FDT.FileName
	}
	if fb.ContentType == "" && pkt.FDT.ContentType != "" {
		fb.ContentType = pkt.FDT.ContentType
	}
	if pkt.LCTHeader.CloseObject {
		fb.closeObject = true
	}
}

func (fb *fileBuffer) storeChunk(pkt *alc.AlcPkt) (bool, int, error) {
	chunkIndex := pkt.SourceBlockNb
	if _, exists := fb.Chunks[chunkIndex]; exists {
		fb.applyMetadata(pkt)
		return false, 0, nil
	}

	payloadLen := pkt.PayloadLength
	if payloadLen == 0 || payloadLen > uint32(len(pkt.EncodingSymbols)) {
		payloadLen = uint32(len(pkt.EncodingSymbols))
	}
	if payloadLen == 0 {
		return false, 0, fmt.Errorf("empty payload for chunk %d", chunkIndex)
	}

	dataToStore := make([]byte, payloadLen)
	copy(dataToStore, pkt.EncodingSymbols[:payloadLen])
	fb.Chunks[chunkIndex] = dataToStore
	fb.applyMetadata(pkt)
	return true, len(dataToStore), nil
}

func (fb *fileBuffer) isComplete() bool {
	if fb.TotalChunks > 0 && len(fb.Chunks) >= int(fb.TotalChunks) {
		return true
	}
	return fb.closeObject && len(fb.Chunks) > 0
}

func (fb *fileBuffer) reconstruct() ([]byte, error) {
	if len(fb.Chunks) == 0 {
		return nil, fmt.Errorf("no chunks available for TOI %d", fb.TOI)
	}

	keys := make([]int, 0, len(fb.Chunks))
	for k := range fb.Chunks {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)

	totalSize := 0
	for _, k := range keys {
		totalSize += len(fb.Chunks[uint32(k)])
	}

	reconstructed := make([]byte, 0, totalSize)
	for _, k := range keys {
		reconstructed = append(reconstructed, fb.Chunks[uint32(k)]...)
	}

	return reconstructed, nil
}

func (fb *fileBuffer) save(saveDir string) error {
	data, err := fb.reconstruct()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		return fmt.Errorf("ensure save dir: %w", err)
	}

	name := fb.FileName
	if name == "" {
		name = fmt.Sprintf("toi_%d.bin", fb.TOI)
	}
	path := filepath.Join(saveDir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}

	md5sum := utils.CalculateMD5(data)
	fmt.Printf("File (TOI=%d) saved to: %s\n", fb.TOI, path)
	fmt.Printf("Reconstructed file MD5: %s\n", md5sum)
	return nil
}
