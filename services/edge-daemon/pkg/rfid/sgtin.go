package rfid

import (
	"encoding/hex"
	"fmt"
	"strconv"
)

// EncodeSGTIN96 builds a 96-bit EPC Gen2 binary representation (Filter 3: Item Level, Partition 5)
func EncodeSGTIN96(companyPrefix string, itemRef int, serialNumber int64) (string, error) {
	if len(companyPrefix) != 7 {
		return "", fmt.Errorf("company prefix must be exactly 7 digits")
	}

	prefixInt, err := strconv.ParseUint(companyPrefix, 10, 64)
	if err != nil {
		return "", err
	}

	// SGTIN-96 Bit Layout:
	// Header: 8 bits (0x30 = 00110000)
	// Filter: 3 bits (001 = POS Item)
	// Partition: 3 bits (101 = Partition 5)
	// Company Prefix: 24 bits
	// Item Reference: 20 bits
	// Serial Number: 38 bits
	var bits uint64 = 0x30
	bits = (bits << 3) | 0x01
	bits = (bits << 3) | 0x05
	bits = (bits << 24) | (prefixInt & 0xFFFFFF)
	bits = (bits << 20) | (uint64(itemRef) & 0xFFFFF)

	upperHex := fmt.Sprintf("%015X", bits)
	lowerHex := fmt.Sprintf("%010X", uint64(serialNumber)&0x3FFFFFFFFF)

	epcHex := upperHex[1:] + lowerHex
	return epcHex, nil
}

func ValidatePermanentInlayRead(epcHex string) bool {
	raw, err := hex.DecodeString(epcHex)
	if err != nil || len(raw) != 12 {
		return false
	}
	return raw[0] == 0x30 // Standard EPC Header for SGTIN-96
}