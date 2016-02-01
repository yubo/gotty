package rec

import (
	"encoding/gob"
	"io"
	"io/ioutil"
	"os"
)

type RecData struct {
	Time int64
	Data []byte
}

type Recorder struct {
	FileName string
	f        *os.File
	enc      *gob.Encoder
}

const (
	Input          = '0'
	Ping           = '1'
	ResizeTerminal = '2'
)

const (
	Output         = '0'
	Pong           = '1'
	SetWindowTitle = '2'
	SetPreferences = '3'
	SetReconnect   = '4'
)

type argResizeTerminal struct {
	Columns float64
	Rows    float64
}

func NewRecorder(dir string) (*Recorder, error) {
	var err error
	rec := &Recorder{}
	if rec.f, err = ioutil.TempFile(dir, ""); err != nil {
		return nil, err
	}
	rec.enc = gob.NewEncoder(rec.f)
	rec.FileName = rec.f.Name()
	return rec, nil
}

func (r *Recorder) Read(d []byte) (n int, err error) {
	return 0, io.EOF
}

func (r *Recorder) Write(d []byte) (n int, err error) {
	if err := r.enc.Encode(RecData{Time: Nanotime(), Data: d}); err != nil {
		return 0, err
	}
	return len(d), nil
}

func (r *Recorder) Close() error {
	return r.f.Close()
}
