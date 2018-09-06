package main

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	m := initMux()
	http.ListenAndServe(":2018", m)
}

var separator = string(append(make([]byte, 1), byte(os.PathSeparator)))[1:]

var curDir = getCurrPath()

var cloudRoot string

func getCurrPath() string {
	file, _ := exec.LookPath(os.Args[0])
	path, _ := filepath.Abs(file)
	index := strings.LastIndex(path, string(os.PathSeparator))
	ret := path[:index]
	return ret
}

type mux struct {
	// map[path]map[method]handler
	muxMap     map[string]map[string]func(http.ResponseWriter, *http.Request)
	staticPath string
}

func (m *mux) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	if origin := req.Header.Get("Origin"); origin != "" {
		resp.Header().Set("Access-Control-Allow-Origin", "*")
		resp.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		resp.Header().Set("Access-Control-Allow-Headers", "Login, Authorization")
		if req.Method == "OPTIONS" {
			return
		}
	}
	path := req.URL.Path
	method := req.Method
	arr := strings.Split(path, "/")
	if arr[1] == m.staticPath {
		if req.URL.Query().Get("download") == "1" {
			resp.Header().Set("Content-Disposition", "attachment; filename="+"'"+arr[len(arr)-1]+"'")
		}
		arr = arr[2:]
		path = strings.Join(arr, separator)
		absPath := cloudRoot + separator + path
		sendFile(resp, req, absPath)
		return
	}
	if methodMap, ok := m.muxMap[path]; ok {
		if handler, hok := methodMap[method]; hok {
			handler(resp, req)
			return
		}
		resp.WriteHeader(400)
		resp.Write([]byte("Bad Method!"))
		return
	}
	resp.WriteHeader(404)
	resp.Write([]byte("404NotFound!"))
}

func (m *mux) get(p string, handler func(http.ResponseWriter, *http.Request)) {
	if mp, ok := m.muxMap[p]; ok {
		mp["GET"] = handler
		return
	}
	newMap := make(map[string]func(http.ResponseWriter, *http.Request))
	newMap["GET"] = handler
	m.muxMap[p] = newMap
}

func (m *mux) post(p string, handler func(http.ResponseWriter, *http.Request)) {
	if mp, ok := m.muxMap[p]; ok {
		mp["POST"] = handler
		return
	}
	newMap := make(map[string]func(http.ResponseWriter, *http.Request))
	newMap["POST"] = handler
	m.muxMap[p] = newMap
}

func (m *mux) put(p string, handler func(http.ResponseWriter, *http.Request)) {
	if mp, ok := m.muxMap[p]; ok {
		mp["PUT"] = handler
		return
	}
	newMap := make(map[string]func(http.ResponseWriter, *http.Request))
	newMap["PUT"] = handler
	m.muxMap[p] = newMap
}

func (m *mux) delete(p string, handler func(http.ResponseWriter, *http.Request)) {
	if mp, ok := m.muxMap[p]; ok {
		mp["DELETE"] = handler
		return
	}
	newMap := make(map[string]func(http.ResponseWriter, *http.Request))
	newMap["DELETE"] = handler
	m.muxMap[p] = newMap
}

func sendFile(resp http.ResponseWriter, req *http.Request, path string) {
	file, err := os.OpenFile(path, 0, os.ModePerm)
	if err != nil {
		resp.WriteHeader(404)
		req.Body.Close()
		file.Close()
		return
	}
	defer file.Close()
	info, _ := file.Stat()
	resp.Header().Set("Content-Length", strconv.Itoa(int(info.Size())))
	resp.WriteHeader(200)
	buf := make([]byte, 512)
	for {
		n, err := file.Read(buf)
		if err != nil {
			resp.Write(buf[0:n])
			break
		}
		resp.Write(buf[0:n])
	}
	req.Body.Close()
}

// 初始化路由
func initMux() *mux {
	m := new(mux)
	m.muxMap = make(map[string]map[string]func(http.ResponseWriter, *http.Request))
	cloudRoot = curDir + separator + "files"
	m.staticPath = "static"
	m.get("/list", readDir)           // 参数：?path=路径^子路径
	m.get("/rename", reName)          // 参数：?path=路径&name=名称
	m.get("/create", createDir)       // 参数：?path=路径&name=目录名
	m.get("/delete", del)             // 参数：?path=路径
	m.get("/move", move)              // 参数：?curPath=当前路径&newPath=新路径
	m.post("/uploadfile", uploadFile) // 参数：?path=路径
	m.get("/login", login)
	return m
}
