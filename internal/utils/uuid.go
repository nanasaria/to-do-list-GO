package utils

import (
	"crypto/rand"
	"encoding/hex"
)

func NewUUID() string {
	var raw [16]byte
	_, _ = rand.Read(raw[:])

	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80

	var formatted [36]byte
	hex.Encode(formatted[0:8], raw[0:4])
	formatted[8] = '-'
	hex.Encode(formatted[9:13], raw[4:6])
	formatted[13] = '-'
	hex.Encode(formatted[14:18], raw[6:8])
	formatted[18] = '-'
	hex.Encode(formatted[19:23], raw[8:10])
	formatted[23] = '-'
	hex.Encode(formatted[24:36], raw[10:16])

	return string(formatted[:])
}

func IsValidUUID(value string) bool {
	if len(value) != 36 {
		return false
	}

	for i := 0; i < len(value); i++ {
		switch i {
		case 8, 13, 18, 23:
			if value[i] != '-' {
				return false
			}
		default:
			if !isHexDigit(value[i]) {
				return false
			}
		}
	}

	return true
}

func isHexDigit(value byte) bool {
	return ('0' <= value && value <= '9') ||
		('a' <= value && value <= 'f') ||
		('A' <= value && value <= 'F')
}
