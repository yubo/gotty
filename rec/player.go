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
	FileName  string
	f         *os.File
	dec       *gob.Decoder
	d         RecData
	speed     float64
	repeat    bool
	init      bool
	fileStart int64
	start     int64
	offset    int64
	maxWait   int64
	window    struct {
		row uint16
		col uint16
	}
}

func NewPlayer(filename string, speed float64, repeat bool, wait int64) (*Player, error) {
	var err error

	p := &Player{FileName: filename,
		speed:   speed,
		repeat:  repeat,
		maxWait: wait * 1000000000,
	}
	if p.f, err = os.OpenFile(filename, os.O_RDONLY, 0); err != nil {
		return nil, err
	}
	p.dec = gob.NewDecoder(p.f)
	return p, nil
}

func (p *Player) Read(d []byte) (n int, err error) {
	for {
		if err = p.dec.Decode(&p.d); err != nil {
			if p.init && p.repeat && err == io.EOF {
				p.start = Nanotime()
				p.f.Seek(0, 0)
				p.dec = gob.NewDecoder(p.f)
				glog.V(2).Infof("read %s EOF, replay again", p.FileName)
				continue
			} else {
				p.Close()
				return 0, err
			}
		}

		switch p.d.Data[0] {
		case ResizeTerminal:
			var args ArgResizeTerminal
			err = json.Unmarshal(p.d.Data[1:], &args)
			if err != nil {
				glog.Errorln("Malformed remote command")
				continue
			}
			p.window.row = uint16(args.Rows)
			p.window.col = uint16(args.Columns)
			continue
		case Output:
			if !p.init {
				p.fileStart = p.d.Time
				p.start = Nanotime()
				p.init = true
			}
			delta := (p.d.Time - p.fileStart) - p.offset -
				int64(float64(Nanotime()-p.start)*p.speed)
			if p.maxWait > 0 && delta > p.maxWait {
				p.offset += delta - p.maxWait
				time.Sleep(time.Duration(p.maxWait) * time.Nanosecond)
			} else if delta > 0 {
				time.Sleep(time.Duration(delta) * time.Nanosecond)
			}
			//glog.Infof("time:%d %d len:%d", Nanotime(), p.d.Time, n)

			n = copy(d, p.d.Data[1:])
			return
		case SysEnv:
			continue
		default:
			glog.Errorf("unknow type(%d) context(%s)",
				p.d.Data[0], string(p.d.Data[1:]))
			continue
		}
	}
}

func (p *Player) Write(d []byte) (n int, err error) {
	return len(d), nil
}

func (p *Player) Close() error {
	return p.f.Close()
}
