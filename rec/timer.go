package rec

/*
#include <stdio.h>
#include <stdint.h>
#include <time.h>

int64_t nanotime(void){
	struct timespec ts;
	clock_gettime(CLOCK_MONOTONIC, &ts);
	return ts.tv_sec*1000000000+ts.tv_nsec;
}

#cgo LDFLAGS: -lrt
*/
import "C"

func Nanotime() int64 {
	return int64(C.nanotime())
}
