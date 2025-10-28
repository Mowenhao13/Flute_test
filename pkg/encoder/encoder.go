package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	raptorq "github.com/xssnick/raptorq"
)

func calculateMD5(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

func main() {
	// Read file to send
	filePath := "/home/Halllo/Projects/flute_test/flute_sender/send_files/test_1mb.bin"
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Println("Read file failed:", err)
		return
	}

	// 示例：使用 RaptorQ 编码器对数据进行编码

	var symbolSize uint32 = 32 * 1024 // 每个符号的大小，控制在 UDP MTU 范围内
	r := raptorq.NewRaptorQ(symbolSize)

	encoder, err := r.CreateEncoder(fileData)
	if err != nil {
		panic(err)
	}

	baseSymbols := encoder.BaseSymbolsNum()
	println("Base Symbols Num:", baseSymbols)

	// var chunkSize uint32 = 1024 * 32
	// for i := uint32(0); i < uint32(len(fileData)); i += chunkSize {
	// 	end := i + chunkSize
	// 	if end > uint32(len(fileData)) {
	// 		end = uint32(len(fileData))
	// 	}
	// 	chunk := fileData[i:end]

	// 	fmt.Printf("Chunk %d md5: %s\n", i/chunkSize, calculateMD5(chunk))
	// }

	symbols := make([][]byte, 0, baseSymbols)
	for i := uint32(0); i < baseSymbols; i++ {
		raw := encoder.GenSymbol(i)
		dup := make([]byte, len(raw))
		copy(dup, raw)

		fmt.Printf("Symbol %d md5: %s\n", i, calculateMD5(dup))

		symbols = append(symbols, dup)
	}
	println("Generated Symbols:", len(symbols))

	fmt.Println("File encoding completed.")

	decoder, err := r.CreateDecoder(uint32(len(fileData)))
	if err != nil {
		panic(err)
	}

	for i := 0; i < len(symbols); i++ {
		sym := symbols[uint32(i)]
		if _, err := decoder.AddSymbol(uint32(i), sym); err != nil {
			panic(err)
		}
	}

	can, decodedData, derr := decoder.Decode()
	if derr != nil {
		panic(derr)
	}
	if !can || decodedData == nil {
		fmt.Println("Decoder failed to reconstruct data.")
		return
	}
	fmt.Println("File decoding completed.")

	// decodedSymbols := make([][]byte, len(symbols))
	// // 记录最后一个符号的实际大小
	// lastSymbolSize := len(fileData) % int(symbolSize)
	// if lastSymbolSize == 0 {
	// 	lastSymbolSize = int(symbolSize)
	// }

	// // 在解码时去除填充数据
	// for i := range symbols {
	// 	start := int(i) * int(symbolSize)
	// 	if start >= len(decodedData) {
	// 		break
	// 	}
	// 	end := start + int(symbolSize)
	// 	if end > len(decodedData) {
	// 		end = len(decodedData)
	// 	}
	// 	chunk := make([]byte, end-start)
	// 	copy(chunk, decodedData[start:end])
	// 	if i == len(symbols)-1 {
	// 		chunk = chunk[:lastSymbolSize] // 去除填充数据
	// 	}
	// 	decodedSymbols[i] = chunk
	// }

	// for i, sym := range symbols {
	// 	if decodedSymbols[i] == nil {
	// 		fmt.Printf("Symbol %d was not decoded.\n", i)
	// 		continue
	// 	}
	// 	encodedMD5 := calculateMD5(sym)
	// 	decodedMD5 := calculateMD5(decodedSymbols[i])
	// 	if encodedMD5 == decodedMD5 {
	// 		fmt.Printf("Symbol %d MD5 match: %s\n", i, encodedMD5)
	// 	} else {
	// 		fmt.Printf("Symbol %d MD5 mismatch! Encoded: %s, Decoded: %s\n", i, encodedMD5, decodedMD5)
	// 	}
	// }

	originalMD5 := calculateMD5(fileData)
	fmt.Printf("Original file MD5: %s\n", originalMD5)

	reconstructedData := make([]byte, len(decodedData))
	copy(reconstructedData, decodedData)

	reconstructedMD5 := calculateMD5(reconstructedData)
	fmt.Printf("Reconstructed file MD5: %s\n", reconstructedMD5)

	if originalMD5 == reconstructedMD5 {
		fmt.Println("MD5 match: The reconstructed file is identical to the original.")
	} else {
		fmt.Println("MD5 mismatch: The reconstructed file differs from the original.")
	}

	outputDir := "./received_files"
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fmt.Println("Failed to create output directory:", err)
		return
	}

	outputName := fmt.Sprintf("decoded_%s", filepath.Base(filePath))
	outputPath := filepath.Join(outputDir, outputName)

	if err := os.WriteFile(outputPath, reconstructedData, 0o644); err != nil {
		fmt.Println("Failed to write reconstructed file:", err)
		return
	}

	fmt.Printf("Reconstructed file written to %s\n", outputPath)

}

// func sendSymbols(remoteAddr string, symbols [][]byte) error {
// 	addr, err := net.ResolveUDPAddr("udp", remoteAddr)
// 	if err != nil {
// 		return fmt.Errorf("resolve remote addr: %w", err)
// 	}

// 	conn, err := net.DialUDP("udp", nil, addr)
// 	if err != nil {
// 		return fmt.Errorf("dial udp: %w", err)
// 	}
// 	defer conn.Close()

// 	for idx, sym := range symbols {
// 		packet := make([]byte, 8+len(sym))
// 		binary.BigEndian.PutUint32(packet[0:4], uint32(idx))
// 		binary.BigEndian.PutUint32(packet[4:8], uint32(len(sym)))
// 		copy(packet[8:], sym)

// 		if _, err := conn.Write(packet); err != nil {
// 			return fmt.Errorf("send symbol %d: %w", idx, err)
// 		}
// 	}

// 	return nil
// }

// func receiveSymbols(listenAddr string, expected int, maxSymbolSize uint32) (map[uint32][]byte, error) {
// 	addr, err := net.ResolveUDPAddr("udp", listenAddr)
// 	if err != nil {
// 		return nil, fmt.Errorf("resolve listen addr: %w", err)
// 	}

// 	conn, err := net.ListenUDP("udp", addr)
// 	if err != nil {
// 		return nil, fmt.Errorf("listen udp: %w", err)
// 	}
// 	defer conn.Close()

// 	buffer := make([]byte, int(maxSymbolSize)+8)
// 	received := make(map[uint32][]byte, expected)

// 	for len(received) < expected {
// 		n, _, err := conn.ReadFromUDP(buffer)
// 		if err != nil {
// 			return nil, fmt.Errorf("read udp: %w", err)
// 		}
// 		if n < 8 {
// 			continue
// 		}

// 		index := binary.BigEndian.Uint32(buffer[0:4])
// 		length := binary.BigEndian.Uint32(buffer[4:8])
// 		if int(length) > n-8 {
// 			continue
// 		}

// 		if _, exists := received[index]; exists {
// 			continue
// 		}

// 		data := make([]byte, length)
// 		copy(data, buffer[8:8+length])
// 		received[index] = data
// 	}

// 	return received, nil
// }
