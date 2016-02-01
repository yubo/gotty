package rec

import (
	"encoding/gob"
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/golang/glog"
)

type Player struct {
	FileName   string
	f          *os.File
	dec        *gob.Decoder
	d          RecData
	speed      float64
	repeat     bool
	init       bool
	file_start int64
	start      int64
	window     struct {
		row uint16
		col uint16
		x   uint16
		y   uint16
	}
}

func NewPlayer(filename string, speed float64, repeat bool) (*Player, error) {
	var err error

	p := &Player{FileName: filename, speed: speed, repeat: repeat}
	if p.f, err = os.OpenFile(filename, os.O_RDONLY, 0); err != nil {
		return nil, err
	}
	p.dec = gob.NewDecoder(p.f)
	return p, nil
}

func (p *Player) Read(d []byte) (n int, err error) {
retry:
	if err = p.dec.Decode(&p.d); err != nil {
		if p.init && p.repeat && err == io.EOF {
			p.start = Nanotime()
			p.f.Seek(0, 0)
			p.dec = gob.NewDecoder(p.f)
			glog.Infof("read %s EOF, replay again", p.FileName)
			goto retry
		} else {
			p.Close()
			return 0, err
		}
	}

	if p.d.Data[0] == ResizeTerminal {
		var args argResizeTerminal
		err = json.Unmarshal(p.d.Data[1:], &args)
		if err != nil {
			glog.Errorln("Malformed remote command")
			goto retry
		}
		p.window.row = uint16(args.Rows)
		p.window.col = uint16(args.Columns)
		goto retry
	}

	if !p.init {
		p.file_start = p.d.Time
		p.start = Nanotime()
		p.init = true
	}

	offset := (p.d.Time - p.file_start) - int64(float64(Nanotime()-p.start)*p.speed)
	if offset > 0 {
		time.Sleep(time.Duration(offset) * time.Nanosecond)
	}
	n = copy(d, p.d.Data[1:])
	return
}

func (p *Player) Write(d []byte) (n int, err error) {
	return len(d), nil
}

func (p *Player) Close() error {
	return p.f.Close()
}
