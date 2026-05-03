package watermark

// crc16 computes CRC-16-CCITT (poly 0x1021, init 0xFFFF).
func crc16(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// encodeRaw builds the rawBits-long bit array: sync || payload || crc.
// Repetition is handled at the block-assignment layer (each block's bit
// is order_index % rawBits), not by expanding the bit stream.
func encodeRaw(payload []byte) []int {
	out := make([]int, 0, rawBits)
	for i := 15; i >= 0; i-- {
		out = append(out, int((syncHeader>>uint(i))&1))
	}
	for _, b := range payload {
		for i := 7; i >= 0; i-- {
			out = append(out, int((b>>uint(i))&1))
		}
	}
	c := crc16(payload)
	for i := 15; i >= 0; i-- {
		out = append(out, int((c>>uint(i))&1))
	}
	return out
}

// decodeRaw validates sync header and CRC against a rawBits-long bit
// array and returns the payload bytes on success.
func decodeRaw(raw []int) ([]byte, bool) {
	if len(raw) != rawBits {
		return nil, false
	}
	var sync uint16
	for i := 0; i < 16; i++ {
		sync = sync<<1 | uint16(raw[i])
	}
	if sync != syncHeader {
		return nil, false
	}
	payload := make([]byte, payloadBits/8)
	for i := 0; i < payloadBits/8; i++ {
		var b byte
		for j := 0; j < 8; j++ {
			b = b<<1 | byte(raw[16+i*8+j])
		}
		payload[i] = b
	}
	var crc uint16
	for i := 0; i < 16; i++ {
		crc = crc<<1 | uint16(raw[16+payloadBits+i])
	}
	if crc != crc16(payload) {
		return nil, false
	}
	return payload, true
}
