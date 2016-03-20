// +build dragonfly freebsd !android,linux netbsd openbsd solaris darwin
// +build cgo

package utils

/*
#cgo CFLAGS: -D_BSD_SOURCE
#include <sys/types.h>
#include <grp.h>

void mysetgrent(void){
	return setgrent();
}

struct group *mygetgrent(void){
	return getgrent();
}

void myendgrent(void){
	return endgrent();
}

char **mynext(char **p){
	return ++p;
}

*/
import "C"
import "unsafe"

func getugroups(username string) ([]uint32, error) {
	var grp *C.struct_group
	var cp **C.char

	hgs := make(map[uint32]int)

	C.mysetgrent()

	grp = C.mygetgrent()
	for grp != nil {
		for cp = grp.gr_mem; unsafe.Pointer(*cp) != nil; cp = C.mynext(cp) {
			if username == C.GoString(*cp) {
				hgs[uint32(grp.gr_gid)] = 1
			}
		}
		grp = C.mygetgrent()
	}

	C.myendgrent()

	gs := make([]uint32, len(hgs))
	i := 0
	for gid, _ := range hgs {
		gs[i] = gid
		i += 1
	}
	return gs, nil
}
