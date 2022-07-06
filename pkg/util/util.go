package util

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"strconv"
	"time"
)

type Serializer struct {
	buf *bytes.Buffer
	in  interface{}
	enc *gob.Encoder
	dec *gob.Decoder
}

func NewSerializer(i interface{}, b *bytes.Buffer) *Serializer {
	s := &Serializer{
		buf: b,
		in:  i,
	}
	s.enc = gob.NewEncoder(s.buf)
	s.dec = gob.NewDecoder(s.buf)
	return s
}

func (s *Serializer) Bytes() ([]byte, error) {
	err := s.enc.Encode(s.in)
	if err != nil {
		return nil, err
	}
	return s.buf.Bytes(), nil
}

func (s *Serializer) Hash() ([32]byte, error) {
	b, err := s.Bytes()
	if err != nil {
		return [32]byte{}, err
	}
	sum := sha256.Sum256(b)
	return sum, nil
}

func (s *Serializer) HashString() (string, error) {
	h, err := s.Hash()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h), nil
}

func PtrString(s string) *string {
	return &s
}

func PtrInt(i int) *int {
	return &i
}

func Uint64ToString(u uint64) string {
	return strconv.FormatUint(u, 10)
}

func StringToUint64(s string) uint64 {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func StringToTimestamp(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
