// +build !cgo plan9 android windows

package utils

import (
	"fmt"
	"runtime"
)

func getugroups(username string) ([]uint32, error) {
	return []uint32{}, fmt.Errorf("utils: getugroups not implemented on %s/%s", runtime.GOOS, runtime.GOARCH)
}
