package rec

import "time"

func Nanotime() int64 {
	return time.Now().UnixNano()
}
