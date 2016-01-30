package tty

import (
	"net"
	"testing"
)

var addrs = "127.0.0.0/8,172.16.0.0/16"

func check(ip string, nets *[]*net.IPNet, b bool, t *testing.T) {
	if ipFilter(ip, nets) != b {
		t.Fatalf("ipFilter(%s, %s) != %v", ip, addrs, b)
	}
}
func TestIpFilter(t *testing.T) {
	nets := parseAddr(addrs)
	check("172.16.0.1", nets, true, t)
	check("127.0.0.1", nets, true, t)
	check("8.8.8.8", nets, false, t)
}
