package tty

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/golang/glog"
)

var demoTpl *template.Template

type demoIndex struct {
	RemoteAddr string
	Sessions   *Session_infos
	Recs       []string
}

func Key2Str(args ...interface{}) string {
	ok := false
	var key ConnKey
	if len(args) == 1 {
		key, ok = args[0].(ConnKey)
	}

	if !ok {
		return fmt.Sprint(args...)
	}

	return fmt.Sprintf("%s/%s", key.Name, key.Addr)
}

func StringJoinSpace(args ...interface{}) string {
	ok := false
	var ss []string
	if len(args) == 1 {
		ss, ok = args[0].([]string)
	}

	if !ok {
		return fmt.Sprint(args...)
	}

	return strings.Join(ss, " ")
}

func demoHandler(w http.ResponseWriter, r *http.Request) {
	opt := &CallOptions{}
	ss := Session_infos{}
	data := demoIndex{}
	now := time.Now().Unix()

	data.RemoteAddr = GlobalOpt.DemoAddr

	//sessions
	if err := Call("Cmd.Ps", opt, &ss); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	} else {
		if len(ss) > 0 {
			for k, _ := range ss {
				if ss[k].ConnTime > 0 {
					ss[k].Time = SubSec(now, ss[k].ConnTime)
				}
			}
		}
	}
	sort.Sort(ss)
	data.Sessions = &ss

	//recs
	if f, err := os.Open(GlobalOpt.RecFileDir); err == nil {
		names, _ := f.Readdirnames(-1)
		f.Close()
		sort.Strings(names)
		if len(names) > 10 {
			data.Recs = names[:10]
		} else {
			data.Recs = names[:]
		}
	}

	demoTpl.Execute(w, data)
}

func demoStaticHandler(w http.ResponseWriter, r *http.Request) {
	path := fmt.Sprintf("%s%s", GlobalOpt.DemoDir, r.RequestURI)
	glog.V(3).Infof("demoStaticHandler file %s,  r:%s", path, r.URL.Path)
	http.ServeFile(w, r, path)
}

func demoExecHandler(w http.ResponseWriter, r *http.Request) {
	var info Session_info
	opt := &CallOptions{}

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&opt.Opt)
	if err != nil {
		fmt.Println(err)
		return
	}

	if opt.Opt.Cmd == "" {
		opt.Opt.Cmd = "/bin/bash"
	}
	opt.Opt.Addr = GlobalOpt.DemoAddr
	opt.Args = strings.Fields(opt.Opt.Cmd)

	if opt.Opt.Action == "exec" {
		if err := Call("Cmd.Exec", opt, &info); err != nil {
			glog.Errorf("exec %v \n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else if opt.Opt.Action == "close" {
		if err := Call("Cmd.Close", opt, nil); err != nil {
			glog.Errorf("close %v \n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		//todo: return successful
	} else if opt.Opt.Action == "play" {
		if err := Call("Cmd.Play", opt, &info); err != nil {
			glog.Errorf("play %v \n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else if opt.Opt.Action == "delete" {
		if err := os.Remove(fmt.Sprintf("%s/%s", GlobalOpt.RecFileDir, opt.Opt.RecId)); err != nil {
			glog.Errorf("delete %v \n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		//todo
	}

	fmt.Printf("%v\n", info)
	RenderJson(w, info)
}

func init() {
	var err error
	demoTpl, err = template.New("gotty demo").Funcs(template.FuncMap{"join": strings.Join}).Parse(`
<!doctype html>
<html>
<head>
	<meta charset="utf-8">
	<meta http-equiv="X-UA-Compatible" content="IE=edge">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>Gotty by yubo</title>
	
	<!-- Latest compiled and minified CSS -->
	<link rel="stylesheet" href="/css/bootstrap.min.css">
	
	<!-- Optional theme -->
	<link rel="stylesheet" href="/css/bootstrap-theme.min.css">
	
	<!-- HTML5 shim and Respond.js for IE8 support of HTML5 elements and media queries -->
	<!--[if lt IE 9]>
	<script src="https://oss.maxcdn.com/html5shiv/3.7.2/html5shiv.min.js"></script>
	<script src="https://oss.maxcdn.com/respond/1.4.2/respond.min.js"></script>
	<![endif]-->
	
	<script src="/js/fetch.js"></script>
	<script src="/js/jquery-1.12.1.js"></script>
</head>

<body>
	<!-- Main jumbotron for a primary marketing message or call to action -->
	<div class="jumbotron">
		<div class="container">
			<h1>Gotty</h1>
			<p>GoTTY is a simple command line tool that turns your CLI tools into web applications.</p>
			<p>
				<a class="btn btn-primary btn-lg" href="https://github.com/yubo/gotty" role="button">View on GitHub</a>
				<a class="btn btn-primary btn-lg" href="https://github.com/yubo/gotty/zipball/master" role="button">Download .zip</a>
				<a class="btn btn-primary btn-lg" href="https://github.com/yubo/gotty/tarball/master" role="button">Download .tar.gz</a>
			</p>
		</div>
	</div>
	
	<div class="container">
		<h2> create </h2>
		<div> <table class="table table-striped">
			<thead><tr>
				<th>Name</th>
				<th>Command</th>
				<th>Method</th>
				<th>Address</th>
			</tr></thead>
			<tbody><tr>
				<td><input class="form-control" type="text" id="execName" placeholder="random" /></td>
				<td><input class="form-control" type="text" id="execCmd" placeholder="bash" /></td>
				<td><input class="form-control" type="text" id="execAddr" placeholder="{{.RemoteAddr}}" /></td>
				<td>
					<input type="checkbox" id="writeCkb"/> writeable
					<input type="checkbox" id="recCkb" /> rec 
					<input type="checkbox" id="shareCkb" /> share
					<input type="checkbox" id="sharewCkb" /> share-write
					<input class="btn btn-default" type="button" value="Submit" id="execBtn" />
				</td>
			</tr><tbody>
		</table></div>
		
		<h2> sessions </h2>
		<div><table id="sessions" class="table table-striped">
			<thead><tr>
				<th>Name</th>
				<th>PName</th>
				<th>LinkNb</th>
				<th>Method</th>
				<th>Status</th>
				<th>Command</th>
				<th>RemoteAddr</th>
				<th>Conntime</th>
				<th>action</th>
			</tr><thead>
			<tbody>
{{range .Sessions}}
			<tr>
				<td>{{.Key.Name}}/{{.Key.Addr}}</td>
				<td>{{.PKey.Name}}/{{.PKey.Addr}}</td>
				<td>{{.LinkNb}}</td>
				<td>{{.Method}}</td>
				<td>{{.Status}}</td>
				<td>{{join .Command " "}}</td>
				<td>{{.RemoteAddr}}</td>
				<td>{{.Time}}</td>
				<td>
					<button class="btn btn-default" value="attach" data-name="{{.Key.Name}}"  data-addr="{{.Key.Addr}}" data-action="attach" {{if not .Share}} disabled="disabled" {{end}}> attach </button>
					<button class="btn btn-default" value="close" data-name="{{.Key.Name}}"  data-addr="{{.Key.Addr}}" data-action="close"> close </button>
				</td>
			</tr>
{{end}}
			</tbody>
		</table> </div>
		
		
		<h2> recorder </h2>
		<div><table id="recs" class="table table-striped">
			<thead><tr><th>RedId</th><th>speed</th><th>action</th></tr></thead>
			<tbody> <tr>
				<td><select class="form-contorl" id="recid"> {{range .Recs}} <option value="{{.}}">{{.}}</option> {{end}} </select></td>
				<td><select class="form-contorl" id="recSpeed"> <option value="1">1x</option><option value="2">2x</option><option falue="4">4x</option> </select></td>
				<td>
					<button class="btn btn-default" value="play" id="recPlay">play</button>
					<button class="btn btn-default" value="delete" id="recDelete">delete</button>
				</td>
			</tr> </tbody>
		</table></div>
		
		<hr />

		<footer>
			<p><a href="https://github.com/yubo/gotty">Gotty</a> is maintained by <a href="https://github.com/yubo">yubo</a>.</p>
		</footer>
	</div> <!-- /container -->
	
	
	
	<script src="/js/demo.js"></script>
	<!-- Latest compiled and minified JavaScript -->
	<script src="/js/bootstrap.min.js"></script>
</body>
</html>`)
	if err != nil {
		glog.Errorln(err)
		os.Exit(1)
	}
}
