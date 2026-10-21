package rfid

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
)

// EncodeSGTIN96 builds a 96-bit EPC Gen2 binary representation (Filter 1: POS Item, Partition 5)
func EncodeSGTIN96(companyPrefix string, itemRef int, serialNumber int64) (string, error) {
	if len(companyPrefix) != 7 {
		return "", fmt.Errorf("ERR_INVALID_PREFIX: company prefix must be exactly 7 digits")
	}
	prefixInt, err := strconv.ParseUint(companyPrefix, 10, 64)
	if err != nil || prefixInt > 9999999 {
		return "", fmt.Errorf("ERR_INVALID_PREFIX: invalid numeric company prefix: %w", err)
	}
	if itemRef < 0 || itemRef > 999999 {
		return "", fmt.Errorf("ERR_INVALID_ITEM_REF: Partition 5 item reference must be 0..999999 (got %d)", itemRef)
	}
	if serialNumber < 0 || serialNumber > 0x3FFFFFFFFF {
		return "", fmt.Errorf("ERR_INVALID_SERIAL: serial number must fit in 38 unsigned bits (got %d)", serialNumber)
	}

	var upper64 uint64 = 0x30
	upper64 = (upper64 << 3) | 0x01
	upper64 = (upper64 << 3) | 0x05
	upper64 = (upper64 << 24) | (prefixInt & 0xFFFFFF)
	upper64 = (upper64 << 20) | (uint64(itemRef) & 0xFFFFF)
	upper64 = (upper64 << 6) | ((uint64(serialNumber) >> 32) & 0x3F)

	lower32 := uint32(uint64(serialNumber) & 0xFFFFFFFF)

	var buf [12]byte
	binary.BigEndian.PutUint64(buf[0:8], upper64)
	binary.BigEndian.PutUint32(buf[8:12], lower32)

	return fmt.Sprintf("%024X", buf[:]), nil
}

func ValidatePermanentInlayRead(epcHex string) bool {
	raw, err := hex.DecodeString(epcHex)
	if err != nil || len(raw) != 12 {
		return false
	}
	return raw[0] == 0x30
}
