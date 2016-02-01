package rec

import (
	"encoding/gob"
	"encoding/json"
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
	SysEnv         = '3'
)

const (
	Output         = '0'
	Pong           = '1'
	SetWindowTitle = '2'
	SetPreferences = '3'
	SetReconnect   = '4'
)

type ArgEnvTerminal struct {
	Term    string `json:"TERM"`
	Shell   string `json:"SHELL"`
	Command string `json:"COMMAND"`
}

type ArgResizeTerminal struct {
	Columns float64
	Rows    float64
}

func NewRecorder(term, shell, command, dir string) (*Recorder, error) {
	var err error
	var buf []byte

	r := &Recorder{}
	if r.f, err = ioutil.TempFile(dir, ""); err != nil {
		return nil, err
	}
	r.enc = gob.NewEncoder(r.f)
	r.FileName = r.f.Name()

	buf, err = json.Marshal(ArgEnvTerminal{Term: term,
		Shell: shell, Command: command})
	if err != nil {
		return nil, err
	}

	r.Write(append([]byte{SysEnv}, buf...))

	return r, nil
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
