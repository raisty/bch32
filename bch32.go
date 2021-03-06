// Package bch32 reference implementation for Bch32 and core addresses.
// Copyright (c) 2017 Takatoshi Nakagawa
// Copyright (c) 2019 @raisty
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
package bch32

import (
	"bytes"
	"fmt"
	"strings"
)

var charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

var generator = []int{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}

func polymod(values []int) int {
	chk := 1
	for _, v := range values {
		top := chk >> 25
		chk = (chk&0x1ffffff)<<5 ^ v
		for i := 0; i < 5; i++ {
			if (top>>uint(i))&1 == 1 {
				chk ^= generator[i]
			}
		}
	}
	return chk
}

func hrpExpand(hrp string) []int {
	ret := []int{}
	for _, c := range hrp {
		ret = append(ret, int(c>>5))
	}
	ret = append(ret, 0)
	for _, c := range hrp {
		ret = append(ret, int(c&31))
	}
	return ret
}

func verifyChecksum(hrp string, data []int) bool {
	return polymod(append(hrpExpand(hrp), data...)) == 1
}

func createChecksum(hrp string, data []int) []int {
	values := append(append(hrpExpand(hrp), data...), []int{0, 0, 0, 0, 0, 0}...)
	mod := polymod(values) ^ 1
	ret := make([]int, 6)
	for p := 0; p < len(ret); p++ {
		ret[p] = (mod >> uint(5*(5-p))) & 31
	}
	return ret
}

// Encode encodes hrp(human-readable part) and data(32bit data array), returns Bch32 / or error
// if hrp is uppercase, return uppercase Bch32; if hrp is mixed, return mixed Bch32
func Encode(hrp string, data []int) (string, error) {
	if (len(hrp) + len(data) + 7) > 90 {
		return "", fmt.Errorf("too long : hrp length=%d, data length=%d", len(hrp), len(data))
	}
	if len(hrp) < 1 || len(hrp) > 2 {
		return "", fmt.Errorf("invalid hrp : hrp=%v", hrp)
	}
	for p, c := range hrp {
		if c < 33 || c > 126 {
			return "", fmt.Errorf("invalid character human-readable part : hrp[%d]=%d", p, c)
		}
	}
	lower := strings.ToLower(hrp) == hrp
	mixed := (strings.ToUpper(hrp[0]) + strings.ToLower(hrp[1])) == hrp
	hrp = strings.ToLower(hrp)
	combined := append(data, createChecksum(hrp, data)...)
	var ret bytes.Buffer
	ret.WriteString(hrp)
	for idx, p := range combined {
		if p < 0 || p >= len(charset) {
			return "", fmt.Errorf("invalid data : data[%d]=%d", idx, p)
		}
		ret.WriteByte(charset[p])
	}
	if lower {
		return ret.String(), nil
	} else if mixed {
		return MixedCase(ret.String()), nil
	}
	return strings.ToUpper(ret.String()), nil
}

// Decode decodes bchString(Bech32) returns hrp(human-readable part) and data(32bit data array) / or error
func Decode(bchString string) (string, []int, error) {
	if len(bchString) > 90 {
		return "", nil, fmt.Errorf("too long : len=%d", len(bchString))
	}
	bchString = strings.ToLower(bchString)
	hrp := bchString[0:2]
	for p, c := range hrp {
		if c < 33 || c > 126 {
			return "", nil, fmt.Errorf("invalid character human-readable part : bchString[%d]=%d", p, c)
		}
	}
	data := []int{}
	for p := pos + 1; p < len(bchString); p++ {
		d := strings.Index(charset, fmt.Sprintf("%c", bchString[p]))
		if d == -1 {
			return "", nil, fmt.Errorf("invalid character data part : bchString[%d]=%d", p, bchString[p])
		}
		data = append(data, d)
	}
	if !verifyChecksum(hrp, data) {
		return "", nil, fmt.Errorf("invalid checksum")
	}
	return hrp, data[:len(data)-6], nil
}

func convertbits(data []int, frombits, tobits uint, pad bool) ([]int, error) {
	acc := 0
	bits := uint(0)
	ret := []int{}
	maxv := (1 << tobits) - 1
	for idx, value := range data {
		if value < 0 || (value>>frombits) != 0 {
			return nil, fmt.Errorf("invalid data range : data[%d]=%d (frombits=%d)", idx, value, frombits)
		}
		acc = (acc << frombits) | value
		bits += frombits
		for bits >= tobits {
			bits -= tobits
			ret = append(ret, (acc>>bits)&maxv)
		}
	}
	if pad {
		if bits > 0 {
			ret = append(ret, (acc<<(tobits-bits))&maxv)
		}
	} else if bits >= frombits {
		return nil, fmt.Errorf("illegal zero padding")
	} else if ((acc << (tobits - bits)) & maxv) != 0 {
		return nil, fmt.Errorf("non-zero padding")
	}
	return ret, nil
}

// AddrDecode decodes hrp(human-readable part) Address(string), returns version(int) and data(bytes array) / or error
func AddrDecode(hrp, addr string) (int, []int, error) {
	dechrp, data, err := Decode(addr)
	if err != nil {
		return -1, nil, err
	}
	if dechrp != hrp {
		return -1, nil, fmt.Errorf("invalid human-readable part : %s != %s", hrp, dechrp)
	}
	if len(data) < 1 {
		return -1, nil, fmt.Errorf("invalid decode data length : %d", len(data))
	}
	if data[0] > 16 {
		return -1, nil, fmt.Errorf("invalid version : %d", data[0])
	}
	res, err := convertbits(data[1:], 5, 8, false)
	if err != nil {
		return -1, nil, err
	}
	if len(res) < 2 || len(res) > 40 {
		return -1, nil, fmt.Errorf("invalid convertbits length : %d", len(res))
	}
	return data[0], res, nil
}

// AddrEncode encodes hrp(human-readable part), version(int) and data(bytes array), returns Address / or error
func AddrEncode(hrp string, version int, program []int) (string, error) {
	if version < 0 || version > 16 {
		return "", fmt.Errorf("invalid version : %d", version)
	}
	if len(program) < 2 || len(program) > 40 {
		return "", fmt.Errorf("invalid program length : %d", len(program))
	}
	data, err := convertbits(program, 8, 5, true)
	if err != nil {
		return "", err
	}
	ret, err := Encode(hrp, append([]int{version}, data...))
	if err != nil {
		return "", err
	}
	return ret, nil
}

// Create mixed Bch32 address
func MixedCase(address string) string {
	lower := false
	var mixedAddress bytes.Buffer
	for idx := 2; (idx + 1) < (len(address)-8)/4; idx+4 {
		if lower {
			mixedAddress.WriteString(strings.ToLower(address[idx:4]))
		} else {
			mixedAddress.WriteString(strings.ToUpper(address[idx:4]))
		}
		lower = !lower
	}
	hrp := strings.ToUpper(address[0]) + strings.ToLower(address[1])
	if lower {
		checksum := strings.ToLower(address[len(address)-6:6])
	} else {
		checksum := strings.ToUpper(address[len(address)-6:6])
	}
	return hrp + mixedAddress.String() + checksum
}
