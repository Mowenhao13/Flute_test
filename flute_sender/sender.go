package main

import (
	"FluteTest/pkg/fdt"
	fd "FluteTest/pkg/filedesc"
	o "FluteTest/pkg/oti"
	sender "FluteTest/pkg/sender"
	ep "FluteTest/pkg/udpendpoint"
	utils "FluteTest/pkg/utils"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"time"

	raptorq "github.com/xssnick/raptorq"
	"gopkg.in/yaml.v3"
)

type senderStaticARP struct {
	Enable    bool   `yaml:"enable"`
	Interface string `yaml:"interface"`
	PeerIP    string `yaml:"peer_ip"`
	PeerMAC   string `yaml:"peer_mac"`
}

type senderNetwork struct {
	SourceIP string `yaml:"source_ip"`
	DestIP   string `yaml:"dest_ip"`
	Port     int    `yaml:"port"`
}

type senderTransmission struct {
	FdtDurationMs        int    `yaml:"fdt_duration_ms"`
	FdtStartID           uint32 `yaml:"fdt_start_id"`
}

type senderFile struct {
	Path        string `yaml:"path"`
	Name        string `yaml:"name"`
	ContentType string `yaml:"content_type"`
}

type fec struct {
	Type                  string `yaml:"type"`
	EncodingSymbolLength  uint16 `yaml:"encoding_symbol_length"`
}

type senderAppConfig struct {
	StaticARP    senderStaticARP    `yaml:"static_arp"`
	Network      senderNetwork      `yaml:"network"`
	Transmission senderTransmission `yaml:"transmission"`
	Files        []senderFile       `yaml:"files"`
	FEC          fec                 `yaml:"fec"`
}

func main() {
	cfg, err := loadSenderConfig("./config/senderCfg.yaml")
	if err != nil {
		fmt.Printf("failed to load sender config: %v\n", err)
		return
	}

	if err := utils.EnsureStaticARP(cfg.StaticARP.Enable, cfg.StaticARP.PeerIP, cfg.StaticARP.PeerMAC, cfg.StaticARP.Interface, "sender"); err != nil {
		fmt.Printf("static ARP setup failed: %v\n", err)
	}

	queue := make([]*fd.FileDesc, 0, len(cfg.Files))
	for _, entry := range cfg.Files {
		if entry.Path == "" {
			fmt.Println("skip file entry with empty path in config")
			continue
		}
		name := entry.Name
		if name == "" {
			name = filepath.Base(entry.Path)
		}
		contentType := entry.ContentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		queue = append(queue, &fd.FileDesc{
			Path:        entry.Path,
			Name:        name,
			ContentType: contentType,
		})
	}
	if len(queue) == 0 {
		fmt.Println("no valid files configured, nothing to send")
		return
	}

	endpointCfg := ep.Endpoint{
		SourceAddr: cfg.Network.SourceIP,
		DestAddr:   cfg.Network.DestIP,
		Port:       cfg.Network.Port,
	}

	sendCfg := sender.SenderConfig{
		FdtDuration: time.Duration(cfg.Transmission.FdtDurationMs) * time.Millisecond,
		FdtStartID:  cfg.Transmission.FdtStartID,
	}

	oti := o.NewNoCode(cfg.FEC.EncodingSymbolLength)
	if cfg.FEC.Type == "RaptorQ" {
		oti = o.NewRaptorQ(cfg.FEC.EncodingSymbolLength)
	}
	

	rq := raptorq.NewRaptorQ(uint32(cfg.FEC.EncodingSymbolLength))

	// 创建 UDP 连接
	destIP := net.ParseIP(endpointCfg.DestAddr)
	if destIP == nil {
		fmt.Printf("invalid destination IP: %s\n", endpointCfg.DestAddr)
		return
	}
	remoteAddr := &net.UDPAddr{IP: destIP, Port: endpointCfg.Port}

	var localAddr *net.UDPAddr
	if endpointCfg.SourceAddr != "" {
		ip := net.ParseIP(endpointCfg.SourceAddr)
		if ip == nil {
			fmt.Printf("invalid source IP: %s", endpointCfg.SourceAddr)
		}
		localAddr = &net.UDPAddr{IP: ip}
	}

	conn, err := net.DialUDP("udp", localAddr, remoteAddr)
	if err != nil {
		fmt.Printf("dial UDP failed: %v", err)
	}
	defer conn.Close()

	senderFileCfg := &sender.FileConfig{}
	senderFdt := &fdt.ExtFDT{}
	s := sender.NewSender(conn, senderFdt, 1, oti, senderFileCfg, sendCfg, rq)

	// 处理文件队列
	for _, filedesc := range queue {
		// 读取文件
		fileData, err := os.ReadFile(filedesc.Path)
		if err != nil {
			fmt.Println("Read file failed:", err)
			continue // 继续处理下一个文件
		}
		filedesc.Size = int64(len(fileData))
		filedesc.Md5 = utils.CalculateMD5(fileData)

		sender.AddFile(s, filedesc)
		fmt.Printf("Sending file %s with FDT ID %d\n", filedesc.Name, filedesc.FdtID)
		serr := s.Send(&fileData)
		if serr != nil {
			fmt.Println("Send file failed:", serr)
			continue // 继续处理下一个文件
		}
	}

	fmt.Println("All files sent.")
}

func loadSenderConfig(path string) (*senderAppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg senderAppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.FEC.EncodingSymbolLength == 0 {
		cfg.FEC.EncodingSymbolLength = 10240
		fmt.Printf("FEC EncodingSymbolLength not set, using default %d\n", cfg.FEC.EncodingSymbolLength)
	}
	if cfg.Transmission.FdtDurationMs <= 0 {
		cfg.Transmission.FdtDurationMs = 1000
	}
	if cfg.Transmission.FdtStartID == 0 {
		cfg.Transmission.FdtStartID = 1
		fmt.Printf("FEC FdtStartID not set, using default %d\n", cfg.Transmission.FdtStartID)
	}

	return &cfg, nil
}
