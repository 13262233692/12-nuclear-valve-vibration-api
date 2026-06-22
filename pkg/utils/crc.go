package utils

import "hash/crc32"

func CRC32(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}

func CRC32Verify(data []byte, expected uint32) bool {
	return CRC32(data) == expected
}
