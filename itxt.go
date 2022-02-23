package pngitxt

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"strings"
	"sync"
)

const header = "\x89PNG\r\n\x1a\n"
const ender = "\x00\x00\x00\x00IEND\xAEB`\x82"

/*
header
len(data)+type+(key+data)+hash(type+(key+data))
len(data)+type+(key+data)+hash(type+(key+data))
ender
*/
var (
	ErrCRC32Mismatch = errors.New("crc32 mismatch")
	ErrNotPNG        = errors.New("not png")
	ErrBadLength     = errors.New("bad length")
)

type PNGiTXt struct {
	sync.Mutex
	Start    []byte
	End      []byte
	startBuf *bytes.Buffer
	iTXt     map[string][]byte
	endBuf   *bytes.Buffer
	r        io.Reader
}

// png + txt + end

func NewPNGiTXt(r io.Reader) (res *PNGiTXt, err error) {
	res = &PNGiTXt{
		startBuf: &bytes.Buffer{},
		iTXt:     map[string][]byte{},
		endBuf:   &bytes.Buffer{},
		r:        r}

	expectedHeader := make([]byte, len(header))
	if _, err = io.ReadFull(res.r, expectedHeader); err != nil {
		return nil, err
	}
	if string(expectedHeader) != header {
		return nil, ErrNotPNG
	}
	if _, err = io.WriteString(res.startBuf, header); err != nil {
		return nil, err
	}

	var end bool
	for !end {
		end, err = res.nextChunk()
		if errors.Is(err, io.EOF) {
			err = nil
			break
		}
		if err != nil {
			return nil, err
		}
	}

	res.Start = res.startBuf.Bytes()
	res.End = res.endBuf.Bytes()
	res.startBuf = nil
	res.endBuf = nil
	return res, nil
}

func (x *PNGiTXt) Del(key string) {
	x.Lock()
	defer x.Unlock()
	delete(x.iTXt, key)
	return
}

func (x *PNGiTXt) Set(key string, value []byte) {
	x.Lock()
	defer x.Unlock()
	x.iTXt[key] = value
	return
}

func (x *PNGiTXt) Get(key string) []byte {
	x.Lock()
	defer x.Unlock()
	return x.iTXt[key]
}
func (x *PNGiTXt) GetAll() map[string][]byte {
	x.Lock()
	defer x.Unlock()
	res := make(map[string][]byte, len(x.iTXt))
	for k, v := range x.iTXt {
		res[k] = v
	}
	return res
}

func (x *PNGiTXt) Write(w io.Writer) error {
	x.Lock()
	defer x.Unlock()
	body, err := x.iTxtBody()
	if err != nil {
		return err
	}

	_, err = w.Write(x.Start)
	if err != nil {
		return err
	}
	_, err = w.Write(body)
	if err != nil {
		return err
	}
	_, err = w.Write(x.End)
	if err != nil {
		return err
	}
	return err
}

func (x *PNGiTXt) iTxtBody() ([]byte, error) {
	w := &bytes.Buffer{}
	for key, value := range x.iTXt {
		body := []byte(fmt.Sprintf("%s\x00\x00\x00\x00\x00%s", key, value))

		if err := binary.Write(w, binary.BigEndian, int32(len(body))); err != nil {
			return nil, err
		}
		checksummer := crc32.NewIEEE()
		ww := io.MultiWriter(w, checksummer)
		_, _ = ww.Write(append([]byte("iTXt"), body...))

		if err := binary.Write(w, binary.BigEndian, checksummer.Sum32()); err != nil {
			return nil, err
		}
	}
	return io.ReadAll(w)
}

func (x *PNGiTXt) nextChunk() (bool, error) {

	var length int32
	if err := binary.Read(x.r, binary.BigEndian, &length); err != nil {
		return false, err
	}

	if length < 0 {
		return false, ErrBadLength
	}

	var rawTyp [4]byte
	if _, err := io.ReadFull(x.r, rawTyp[:]); err != nil {
		return false, err
	}
	body, err := io.ReadAll(io.LimitReader(x.r, int64(length)))
	if err != nil {
		return false, err
	}

	typ := string(rawTyp[:])
	isTxt := false
	switch typ {
	case "iTXt":
		split := bytes.Split(body, []byte("\x00\x00\x00\x00\x00"))
		if len(split) == 2 {
			x.iTXt[strings.TrimSpace(string(split[0]))] = split[1]
		}
		isTxt = true
	case "IEND":
		if err = binary.Write(x.endBuf, binary.BigEndian, length); err != nil {
			return false, err
		}
		if _, err = x.endBuf.Write(append([]byte(typ), body...)); err != nil {
			return false, err
		}
		if _, err = io.Copy(x.endBuf, x.r); err != nil {
			return false, err
		}
		return true, nil
	default:
		if err = binary.Write(x.startBuf, binary.BigEndian, length); err != nil {
			return false, err
		}
		if _, err = x.startBuf.Write(append([]byte(typ), body...)); err != nil {
			return false, err
		}
	}

	// CRC-32 check
	checksummer := crc32.NewIEEE()
	_, _ = checksummer.Write(append(rawTyp[:], body...))
	var crc32 uint32
	if err = binary.Read(x.r, binary.BigEndian, &crc32); err != nil {
		return false, err
	}
	if crc32 != checksummer.Sum32() {
		return false, ErrCRC32Mismatch
	}

	if !isTxt {
		if err = binary.Write(x.startBuf, binary.BigEndian, crc32); err != nil {
			return false, err
		}
	}

	return false, nil
}
