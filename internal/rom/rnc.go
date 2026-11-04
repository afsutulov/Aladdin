// Package rom imports data from the supplied Final Cut image at build time.
// It contains no CPU interpreter or emulated hardware.
package rom

import (
	"encoding/binary"
	"fmt"
)

func CRC(data []byte) uint16 {
	var c uint16
	for _, b := range data {
		c ^= uint16(b)
		for i := 0; i < 8; i++ {
			if c&1 != 0 {
				c = c>>1 ^ 0xa001
			} else {
				c >>= 1
			}
		}
	}
	return c
}

type bitReader struct {
	data       []byte
	pos, count int
	bits       uint32
}

func (r *bitReader) peek(n int) uint32 {
	if n < 0 || n >= len(r.data) {
		return 0
	}
	return uint32(r.data[n])
}
func (r *bitReader) read(n int) uint32 {
	var result uint32
	for i := 0; i < n; i++ {
		if r.count == 0 {
			if r.pos >= len(r.data) {
				panic("truncated bit stream")
			}
			r.bits = r.peek(r.pos) | r.peek(r.pos+1)<<8 | r.peek(r.pos+2)<<16 | r.peek(r.pos+3)<<24
			r.pos += 2
			r.count = 16
		}
		result |= (r.bits & 1) << i
		r.bits >>= 1
		r.count--
	}
	return result
}

type leaf struct {
	depth int
	code  uint32
}

func (r *bitReader) table() []leaf {
	n := int(r.read(5))
	if n > 16 {
		panic("invalid Huffman table")
	}
	t := make([]leaf, n)
	for i := range t {
		t[i].depth = int(r.read(4))
	}
	var code uint32
	for depth := 1; depth <= 16; depth++ {
		for i := range t {
			if t[i].depth != depth {
				continue
			}
			v := code >> (32 - depth)
			var reversed uint32
			for j := 0; j < depth; j++ {
				reversed = reversed<<1 | v&1
				v >>= 1
			}
			t[i].code = reversed
			code += 1 << (32 - depth)
		}
	}
	return t
}
func (r *bitReader) symbol(t []leaf) int {
	for i, l := range t {
		if l.depth == 0 || r.bits&((1<<l.depth)-1) != l.code {
			continue
		}
		r.read(l.depth)
		if i < 2 {
			return i
		}
		return 1<<(i-1) | int(r.read(i-1))
	}
	panic("invalid Huffman symbol")
}

// UnpackRNC1 checks both archive CRCs and expands an RNC method 1 block.
func UnpackRNC1(src []byte) (dst []byte, err error) {
	defer func() {
		if p := recover(); p != nil {
			dst = nil
			err = fmt.Errorf("RNC: %v", p)
		}
	}()
	if len(src) < 18 || string(src[:4]) != "RNC\x01" {
		return nil, fmt.Errorf("not an RNC1 block")
	}
	n, p := int(binary.BigEndian.Uint32(src[4:])), int(binary.BigEndian.Uint32(src[8:]))
	if n > 16<<20 || p > len(src)-18 || n <= 0 {
		return nil, fmt.Errorf("invalid RNC sizes")
	}
	packed := src[18 : 18+p]
	if CRC(packed) != binary.BigEndian.Uint16(src[14:]) {
		return nil, fmt.Errorf("packed CRC mismatch")
	}
	r := bitReader{data: packed}
	dst = make([]byte, 0, n)
	if r.read(2) != 0 {
		return nil, fmt.Errorf("protected RNC data is unsupported")
	}
	for len(dst) < n {
		raw, offset, length := r.table(), r.table(), r.table()
		chunks := int(r.read(16))
		if chunks == 0 {
			panic("empty chunk")
		}
		for j := 0; j < chunks; j++ {
			count := r.symbol(raw)
			if count > 0 {
				if r.pos+count > len(packed) || len(dst)+count > n {
					panic("literal out of bounds")
				}
				dst = append(dst, packed[r.pos:r.pos+count]...)
				r.pos += count
				r.bits = (r.peek(r.pos)|r.peek(r.pos+1)<<8|r.peek(r.pos+2)<<16)<<r.count | r.bits&((1<<r.count)-1)
			}
			if j+1 < chunks {
				distance, size := r.symbol(offset)+1, r.symbol(length)+2
				if distance > len(dst) || len(dst)+size > n {
					panic("back reference out of bounds")
				}
				for k := 0; k < size; k++ {
					dst = append(dst, dst[len(dst)-distance])
				}
			}
		}
	}
	if CRC(dst) != binary.BigEndian.Uint16(src[12:]) {
		return nil, fmt.Errorf("unpacked CRC mismatch")
	}
	return dst, nil
}
