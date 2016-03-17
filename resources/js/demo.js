$("#execBtn").on("click", function(){
	var json = {
		action: "exec",
		name: $("#execName").val(),
	    cmd: $("#execCmd").val(),
//	    addr: $("#execAddr").val(),
	    write: $("#writeCkb").prop("checked"),
	    rec: $("#recCkb").prop("checked"),
	    share: $("#shareCkb").prop("checked"),
	    sharew: $("#sharewCkb").prop("checked")
	};

	fetch("/cmd", {
		method: 'post',
		body: JSON.stringify(json)
	}).then(function(response){
		return response.json()
	}).then(function(json){
		window.open("/?name="+json.Key.Name+"&addr="+json.Key.Addr)
	})
	
});

$("#sessions").on("click", "button", function(e){
	var name = $(e.currentTarget).data("name");
	var addr = $(e.currentTarget).data("addr")
	var action = $(e.currentTarget).data("action")

	if(action == "attach"){
		window.open("/?name="+name+"&addr="+addr)
	}else if(action == "close"){
		fetch("/cmd", {
			method: 'post',
			body: JSON.stringify({ action: action, name: name, addr: addr })
		}).then(function(response){
			return response.json()
		}).then(function(json){
			console.log('parsed json', json)
			alert("delete success")
			window.location.reload();
		}).catch(function(ex) {
			console.log('parsing failed', action, ex)
		})
	}
});

$("#recPlay").on("click", function(){
	var json = {
		action: "play",
	    recid: $("#recid").val(),
		repeat: true,
		maxwait: 1,
	    speed: parseFloat($("#recSpeed").val())
	};
	fetch("/cmd", {
		method: 'post',
		body: JSON.stringify(json)
	}).then(function(response){
		return response.json()
	}).then(function(json){
		window.open("/?name="+json.Key.Name+"&addr="+json.Key.Addr)
	}).catch(function(ex) {
		//console.log('parsing failed', action, ex)
		alert(action + ex)
	})
});

$("#recDelete").on("click", function(){
	var json = {
		action: "delete",
	    recid: $("#recid").val(),
	    speed: parseFloat($("#recSpeed").val())
	};
	fetch("/cmd", {
		method: 'post',
		body: JSON.stringify(json)
	}).then(function(response){
		return response.json()
	}).then(function(json){
		console.log('parsed json', json)
		alert("delete success")
		window.location.reload();
	}).catch(function(ex) {
		console.log('parsing failed', action, ex)
	})
});

