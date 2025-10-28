// package main

// import (
// 	// alc "FluteTest/pkg/alc"
// 	"encoding/binary"
// 	"fmt"
// 	"net"
// 	"os"
// 	"path/filepath"
// 	"sort"
// )

// func main() {
// 	listen, err := net.ListenUDP("udp", &net.UDPAddr{
// 		IP:   net.IPv4(192, 168, 1, 102),
// 		Port: 3400,
// 	})
// 	if err != nil {
// 		fmt.Println("Listen failed, err:", err)
// 		return
// 	}
// 	defer listen.Close()

// 	fileSaveDir := "received_files"
// 	recvFileName := "test_1024mb.bin"
// 	savePath := filepath.Join(fileSaveDir, recvFileName)

// 	// Ensure the directory exists
// 	if err := os.MkdirAll(fileSaveDir, 0755); err != nil {
// 		fmt.Printf("Failed to create directory %s: %v\n", fileSaveDir, err)
// 		return
// 	}

// 	// Open file for writing (truncate if exists)
// 	file, err := os.Create(savePath)
// 	if err != nil {
// 		fmt.Printf("Failed to create file %s: %v\n", savePath, err)
// 		return
// 	}
// 	defer file.Close()

// 	chunks := make(map[int32][]byte)
// 	var totalChunks int32 = -1

// 	for {
// 		var data [65507]byte

// 		n, addr, err := listen.ReadFromUDP(data[:])
// 		if err != nil {
// 			fmt.Println("Read UDP failed, err:", err)
// 			continue
// 		}

// 		// pkt, err := alc.ParseAlcPkt(data[:n])
// 		// if err != nil {
// 		// 	fmt.Printf("Parse ALC packet failed: %v\n", err)
// 		// 	continue
// 		// }

// 		// seqNum := int32(pkt.SourceBlockNb)
// 		// total := int32(pkt.EncodingSymbol)
// 		// chunkData := pkt.EncodingSymbols
// 		seqNum := int32(binary.BigEndian.Uint32(data[0:4]))
// 		total := int32(binary.BigEndian.Uint32(data[4:8]))
// 		chunkData := data[8:n]
// 		if total <= 0 {
// 			fmt.Println("Invalid total chunk count")
// 			continue
// 		}

// 		if totalChunks == -1 {
// 			totalChunks = total
// 		} else if totalChunks != total {
// 			fmt.Println("Inconsistent total chunks")
// 			continue
// 		}

// 		chunks[seqNum] = make([]byte, len(chunkData))
// 		copy(chunks[seqNum], chunkData)

// 		fmt.Printf("Received chunk %d/%d from %v\n", seqNum+1, totalChunks, addr)

// 		// Check if all chunks received
// 		if len(chunks) == int(totalChunks) {
// 			// Sort chunks by seqNum
// 			var keys []int
// 			for k := range chunks {
// 				keys = append(keys, int(k))
// 			}
// 			sort.Ints(keys)

// 			// Write to file
// 			for _, k := range keys {
// 				if _, err := file.Write(chunks[int32(k)]); err != nil {
// 					fmt.Printf("Failed to write to file: %v\n", err)
// 					break
// 				}
// 			}
// 			fmt.Printf("File assembled and saved to: %s\n", savePath)

// 			// Reset for next file
// 			chunks = make(map[int32][]byte)
// 			totalChunks = -1
// 			file.Seek(0, 0) // Reset file for next write, or close and reopen
// 		}

// 		// _, err = listen.WriteToUDP([]byte("ACK"), addr)
// 		// if err != nil {
// 		// 	fmt.Println("Write to UDP failed, err:", err)
// 		// 	continue
// 		// }
// 	}
// }

package main

import (
	alc "FluteTest/pkg/alc"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"

	raptorq "github.com/xssnick/raptorq"
)

const (
	ServerIP    = "192.168.1.103"
	ServerPort  = 3400
	FileSaveDir = "./received_files"
)

func main() {
	// Setup UDP listener
	listenAddr := &net.UDPAddr{
		IP:   net.ParseIP(ServerIP),
		Port: ServerPort,
	}
	listen, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		fmt.Println("Listen failed:", err)
		return
	}
	defer listen.Close()

	// Prepare file storage
	if err := os.MkdirAll(FileSaveDir, 0755); err != nil {
		fmt.Printf("Failed to create directory: %v\n", err)
		return
	}
	savePath := filepath.Join(FileSaveDir, "a.jpg")

	// Receive chunks
	var totalChunks uint32

	chunks := make(map[uint32][]byte)
	var rq *raptorq.RaptorQ
	var symbolSize uint32
	buf := make([]byte, 65507)

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
			break
		}

		if totalChunks == 0 {
			totalChunks = pkt.TotalChunks
		}

		if totalChunks == 0 {
			fmt.Println("Total chunk count unknown, skip packet")
			continue
		}

		if len(pkt.EncodingSymbols) == 0 {
			fmt.Printf("Skip empty payload packet from %v\n", addr)
			continue
		}

		chunkIndex := pkt.SourceBlockNb
		// if _, exists := chunks[chunkIndex]; exists {
		// 	fmt.Printf("Duplicate chunk %d ignored\n", chunkIndex)
		// 	continue
		// }

		payloadLen := pkt.PayloadLength
		if payloadLen == 0 {
			payloadLen = uint32(len(pkt.EncodingSymbols))
		}

		var dataToStore []byte

		if pkt.LCTHeader.CodePoint == 1 {
			if rq == nil {
				symbolSize = uint32(pkt.OTI.EncodingSymbolLength)
				if symbolSize == 0 {
					symbolSize = uint32(len(pkt.EncodingSymbols))
				}
				if symbolSize == 0 {
					fmt.Println("Unable to determine symbol size for RaptorQ decoding, skip chunk")
					continue
				}
				rq = raptorq.NewRaptorQ(symbolSize)
			}

			decoder, err := rq.CreateDecoder(payloadLen)
			if err != nil {
				fmt.Println("Failed to create decoder", err)
				continue
			}
			if _, err = decoder.AddSymbol(chunkIndex, pkt.EncodingSymbols); err != nil {
				fmt.Println("Failed to add symbol to decoder", err)
				continue
			}

			complete, decoded, err := decoder.Decode()
			if err != nil {
				fmt.Println("Decode error", err)
			}
			if !complete || len(decoded) < int(payloadLen) {
				fmt.Printf("Decoder not satisfied for chunk %d (len=%d, want=%d)\n", chunkIndex, len(decoded), payloadLen)
				continue
			}

			dataToStore = make([]byte, payloadLen)
			copy(dataToStore, decoded[:payloadLen])
		} else {
			if payloadLen > uint32(len(pkt.EncodingSymbols)) {
				payloadLen = uint32(len(pkt.EncodingSymbols))
			}
			dataToStore = make([]byte, payloadLen)
			copy(dataToStore, pkt.EncodingSymbols[:payloadLen])
		}

		chunks[chunkIndex] = dataToStore

		fmt.Printf("Received chunk %d/%d from %v (stored=%d bytes)\n", pkt.SourceBlockNb+1, totalChunks, addr, len(dataToStore))

		if len(chunks) == int(totalChunks) {
			var keys []int
			for k := range chunks {
				keys = append(keys, int(k))
			}
			sort.Ints(keys)

			totalSize := 0
			for _, k := range keys {
				totalSize += len(chunks[uint32(k)])
			}

			reconstructed := make([]byte, 0, totalSize)
			for _, k := range keys {
				reconstructed = append(reconstructed, chunks[uint32(k)]...)
			}

			if err := os.WriteFile(savePath, reconstructed, 0644); err != nil {
				fmt.Printf("Failed to write reconstructed file: %v\n", err)
				continue
			}

			fmt.Printf("File saved to: %s\n", savePath)
		}
		fmt.Printf("CloseObject: %v, Chunks received: %d/%d\n", pkt.LCTHeader.CloseObject, len(chunks), totalChunks)
	}

	// for {
	// 	n, addr, err := listen.ReadFromUDP(buf)
	// 	if err != nil {
	// 		fmt.Println("Read error:", err)
	// 		continue
	// 	}

	// 	// Parse ALC packet
	// 	pkt, err := alc.ParseAlcPkt(buf[:n])
	// 	if err != nil {
	// 		fmt.Printf("Parse error: %v\n", err)
	// 		continue
	// 	}

	// 	if pkt.LCTHeader.CloseSession {
	// 		fmt.Println("Received close session packet")
	// 		break
	// 	}

	// 	if totalChunks == 0 {
	// 		totalChunks = pkt.TotalChunks
	// 	}

	// 	if totalChunks == 0 {
	// 		fmt.Println("Total chunk count unknown, skip packet")
	// 		continue
	// 	}

	// 	if len(pkt.EncodingSymbols) == 0 {
	// 		fmt.Printf("Skip empty payload packet from %v\n", addr)
	// 		continue
	// 	}

	// 	chunkIndex := pkt.SourceBlockNb
	// 	if _, exists := chunks[chunkIndex]; exists {
	// 		fmt.Printf("Duplicate chunk %d ignored\n", chunkIndex)
	// 		continue
	// 	}

	// 	dataToStore := make([]byte, len(pkt.EncodingSymbols))
	// 	copy(dataToStore, pkt.EncodingSymbols)

	// 	if pkt.LCTHeader.CodePoint == 1 {
	// 		if rq == nil {
	// 			symbolSize = uint32(pkt.OTI.EncodingSymbolLength)
	// 			if symbolSize == 0 {
	// 				symbolSize = uint32(len(pkt.EncodingSymbols))
	// 			}
	// 			if symbolSize == 0 {
	// 				fmt.Println("Unable to determine symbol size for RaptorQ decoding")
	// 				goto storeChunk
	// 			}
	// 			rq = raptorq.NewRaptorQ(symbolSize)
	// 		}

	// 		// decoded, err := decodeRaptorQSymbol(rq, pkt.EncodingSymbols, symbolSize, chunkIndex)
	// 		// if err != nil {
	// 		// 	fmt.Printf("RaptorQ decode failed for chunk %d: %v\n", chunkIndex, err)
	// 		// } else {
	// 		// 	dataToStore = decoded
	// 		// }
	// 		decoder, err := rq.CreateDecoder(uint32(len(pkt.EncodingSymbols)))
	// 		if err != nil {
	// 			fmt.Println("Failed to create decoder", err)
	// 		}
	// 		if _, err = decoder.AddSymbol(chunkIndex, pkt.EncodingSymbols); err != nil {
	// 			fmt.Println("Failed to add symbol to decoder", err)
	// 		}
	// 		complete, data, err := decoder.Decode()
	// 		if err != nil {
	// 			fmt.Println("Decode error", err)
	// 		}
	// 		if complete && len(data) > 0 {
	// 			dataToStore = data
	// 		} else {
	// 			fmt.Printf("Decoder not satisfied for chunk %d\n", chunkIndex)
	// 		}

	// 	}

	// storeChunk:
	// 	chunks[chunkIndex] = dataToStore
	// 	fmt.Printf("Received chunk %d/%d from %v\n", pkt.SourceBlockNb+1, totalChunks, addr)

	// 	// Check if all chunks received
	// 	if len(chunks) == int(totalChunks) {
	// 		// Sort and write to file
	// 		var keys []int
	// 		for k := range chunks {
	// 			keys = append(keys, int(k))
	// 		}
	// 		sort.Ints(keys)

	// 		for _, k := range keys {
	// 			if _, err := file.Write(chunks[uint32(k)]); err != nil {
	// 				fmt.Printf("Write error: %v\n", err)
	// 				break
	// 			}
	// 		}

	// 		fmt.Printf("File saved to: %s\n", savePath)
	// 		// return
	// 	}
	// }
}

// func decodeRaptorQSymbol(rq *raptorq.RaptorQ, symbol []byte, dataLen, chunkIndex uint32) ([]byte, error) {
// 	if rq == nil {
// 		return nil, fmt.Errorf("raptorq instance not initialized")
// 	}
// 	if dataLen == 0 {
// 		dataLen = uint32(len(symbol))
// 	}
// 	if dataLen == 0 {
// 		return nil, fmt.Errorf("invalid data length for decoding")
// 	}

// 	dec, err := rq.CreateDecoder(dataLen)
// 	if err != nil {
// 		return nil, fmt.Errorf("create decoder: %w", err)
// 	}

// 	if _, err = dec.AddSymbol(chunkIndex, symbol); err != nil {
// 		return nil, fmt.Errorf("add symbol: %w", err)
// 	}

// 	complete, data, err := dec.Decode()
// 	if err != nil {
// 		return nil, fmt.Errorf("decode: %w", err)
// 	}
// 	if !complete || len(data) == 0 {
// 		return nil, fmt.Errorf("decoder not satisfied with single symbol")
// 	}

// 	decoded := make([]byte, len(data))
// 	copy(decoded, data)
// 	return decoded, nil
// }
