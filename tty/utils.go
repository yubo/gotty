package tty

import (
	"net"
	"strings"

	"github.com/golang/glog"
)

func parseAddr(addrs string) *[]*net.IPNet {
	var nets []*net.IPNet
	for _, addr := range strings.Split(addrs, ",") {
		if _, net, err := net.ParseCIDR(addr); err != nil {
			glog.Info(err.Error())
		} else {
			nets = append(nets, net)
		}
	}
	return &nets
}

func ipFilter(addr string, nets *[]*net.IPNet) bool {
	if ip := net.ParseIP(addr); ip != nil {
		for _, net := range *nets {
			if net.Contains(ip) {
				return true
			}
		}
	}
	return false
}