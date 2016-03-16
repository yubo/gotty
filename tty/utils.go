package tty

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
)

func parseAddr(addrs string) *[]*net.IPNet {
	var nets []*net.IPNet
	for _, addr := range strings.Split(addrs, ",") {
		if _, net, err := net.ParseCIDR(addr); err != nil {
			glog.V(2).Info(err.Error())
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

func environment() map[string]string {
	env := map[string]string{}

	for _, keyval := range os.Environ() {
		pair := strings.SplitN(keyval, "=", 2)
		env[pair[0]] = pair[1]
	}

	return env
}

func parseFuncFiles(funcs template.FuncMap, filenames ...string) (*template.Template, error) {
	var t *template.Template

	if len(filenames) == 0 {
		// Not really a problem, but be consistent.
		return nil, fmt.Errorf("html/template: no files named in call to ParseFiles")
	}
	for _, filename := range filenames {
		b, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		s := string(b)
		name := filepath.Base(filename)
		// First template becomes return value if not already defined,
		// and we use that one for subsequent New calls to associate
		// all the templates together. Also, if this file has the same name
		// as t, this file becomes the contents of t, so
		//  t, err := New(name).Funcs(xxx).ParseFiles(name)
		// works. Otherwise we create a new template associated with t.
		var tmpl *template.Template
		if t == nil {
			t = template.New(name).Funcs(funcs)
		}
		if name == t.Name() {
			tmpl = t
		} else {
			tmpl = t.New(name).Funcs(funcs)
		}
		_, err = tmpl.Parse(s)
		if err != nil {
			return nil, err
		}
	}
	return t, nil
}

func RenderJson(w http.ResponseWriter, v interface{}) {
	bs, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Write(bs)
}

func SubSec(a, b int64) string {
	d := a - b
	if d < 60 {
		return fmt.Sprintf("%ds", d)
	}

	f := float32(d) / 60
	if f < 60 {
		return fmt.Sprintf("%.2fm", f)
	}

	return fmt.Sprintf("%.2fh", f/24)
}
