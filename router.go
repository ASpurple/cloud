package main

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type dirInfo struct {
	Name string
	Size int64
	Type int
}

type dirs struct {
	List []dirInfo
}

type response struct {
	Code int
	Info string
	Data interface{}
}

func res(code int, info string, data interface{}) []byte {
	r := response{Code: code, Info: info, Data: data}
	js, err := json.Marshal(r)
	if err != nil {
		jsb, _ := json.Marshal(response{Code: -2, Info: "数据异常！"})
		return jsb
	}
	return js
}

// 读取文件夹并返回文件列表
func returnDirs(resp http.ResponseWriter, req *http.Request, path string) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		resp.Write(res(-1, "路径不存在！", nil))
		return
	}
	res := new(response)
	ds := new(dirs)
	ds.List = make([]dirInfo, 0)
	for _, f := range files {
		info := dirInfo{Name: f.Name(), Size: f.Size()}
		if f.IsDir() {
			info.Type = 0
		} else {
			info.Type = 1
		}
		ds.List = append(ds.List, info)
	}
	res.Code = 0
	res.Info = "success"
	res.Data = ds
	js, _ := json.Marshal(res)
	resp.WriteHeader(200)
	resp.Write(js)
}

// 构建绝对路径
func absPath(p string) string {
	arr := strings.Split(p, "^")
	if len(arr) == 1 {
		return cloudRoot
	}
	path := cloudRoot + separator + strings.Join(arr, separator)
	return path
}

// 判断路径是否存在
func exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

// 接口

// 权限验证
func auth(req *http.Request) bool {
	a := req.Header.Get("Authorization")
	h := md5.New()
	_, err := h.Write([]byte("zzz"))
	if err != nil {
		return false
	}
	bs := h.Sum(nil)
	str := hex.EncodeToString(bs)
	if str == a {
		return true
	}
	return false
}

// 根据传入的路径读取目录下的所有文件
func readDir(resp http.ResponseWriter, req *http.Request) {
	path := req.URL.Query().Get("path")
	path = absPath(path)
	returnDirs(resp, req, path)
}

// 重命名
func reName(resp http.ResponseWriter, req *http.Request) {
	if !auth(req) {
		resp.WriteHeader(401)
		resp.Write(res(-1, "没有操作权限！", nil))
		return
	}
	vals := req.URL.Query()
	filePath := vals.Get("path")
	name := vals.Get("name")
	arr := strings.Split(filePath, "^")
	var newPath string
	if len(arr) > 1 {
		arr = arr[0 : len(arr)-1]
		newPath = cloudRoot + strings.Join(arr, separator) + separator + strings.TrimSpace(name)
	} else {
		newPath = cloudRoot + separator + strings.TrimSpace(name)
	}
	err := os.Rename(absPath(filePath), newPath)
	if err != nil {
		resp.WriteHeader(200)
		js, _ := json.Marshal(response{Code: -1, Info: "修改失败！"})
		resp.Write(js)
		return
	}
	js, _ := json.Marshal(response{Code: 0, Info: "success"})
	resp.Write(js)
}

// 新建目录
func createDir(resp http.ResponseWriter, req *http.Request) {
	if !auth(req) {
		resp.WriteHeader(401)
		resp.Write(res(-1, "没有操作权限！", nil))
		return
	}
	vals := req.URL.Query()
	path := vals.Get("path")
	name := vals.Get("name")
	path = absPath(path)
	path += separator + name
	if exists(path) {
		js, _ := json.Marshal(response{Code: -1, Info: "路径已存在！"})
		resp.Write(js)
		return
	}
	err := os.Mkdir(path, os.ModePerm)
	if err != nil {
		js, _ := json.Marshal(response{Code: -1, Info: "创建失败！"})
		resp.Write(js)
		return
	}
	js, _ := json.Marshal(response{Code: 0, Info: "创建成功！"})
	resp.Write(js)
}

// 删除文件
func del(resp http.ResponseWriter, req *http.Request) {
	if !auth(req) {
		resp.WriteHeader(401)
		resp.Write(res(-1, "没有操作权限！", nil))
		return
	}
	path := req.URL.Query().Get("path")
	path = absPath(path)
	info, err := os.Stat(path)
	if err != nil {
		resp.Write(res(-1, "路径不存在！", nil))
		return
	}
	if info.IsDir() {
		err = os.RemoveAll(path)
	} else {
		err = os.Remove(path)
	}
	if err != nil {
		resp.Write(res(-1, "删除失败！", nil))
		return
	}
	resp.Write(res(0, "success", nil))
}

// 移动文件/文件夹
func move(resp http.ResponseWriter, req *http.Request) {
	if !auth(req) {
		resp.WriteHeader(401)
		resp.Write(res(-1, "没有操作权限！", nil))
		return
	}
	values := req.URL.Query()
	curP := absPath(values.Get("curPath"))
	newP := absPath(values.Get("newPath"))
	if !exists(curP) {
		resp.Write(res(-1, "路径不存在！", nil))
		return
	}
	err := os.Rename(curP, newP)
	if err != nil {
		resp.Write(res(-1, "移动失败！", nil))
		return
	}
	resp.Write(res(0, "移动成功！", nil))
}

// 上传文件
func uploadFile(resp http.ResponseWriter, req *http.Request) {
	if !auth(req) {
		resp.WriteHeader(401)
		resp.Write(res(-1, "没有操作权限！", nil))
		return
	}
	path := absPath(req.URL.Query().Get("path"))
	if !exists(path) {
		resp.Write(res(-1, "路径不存在！", nil))
		return
	}
	tp := req.Header.Get("Content-Type")
	length, _ := strconv.Atoi(req.Header.Get("Content-Length"))
	end := len([]byte("----" + strings.Split(tp, "=")[1] + "--"))
	// 换行符长度
	et := len([]byte("\r\n"))
	var buf []byte
	bs := make([]byte, 256)
	for {
		n, err := req.Body.Read(bs)
		if err != nil || len(buf) >= length {
			buf = append(buf, bs[0:n]...)
			break
		}
		if n > 0 {
			buf = append(buf, bs[0:n]...)
		}
	}
	buf = buf[0:(len(buf) - end - et)]
	info := strings.Split(string(buf[0:256]), "\r\n")
	infoLen := len([]byte(info[0]+info[1]+info[2])) + 4*et
	bin := buf[infoLen:]
	fileName := strings.Split(info[1], "=")[2]
	fileName = fileName[1:(len(fileName) - 1)]
	fileName = path + separator + fileName
	for exists(fileName) {
		if idx := strings.LastIndex(fileName, "."); idx != -1 {
			fileName = fileName[0:idx] + "0" + fileName[idx:]
		} else {
			fileName += "0"
		}
	}
	newFile, err := os.Create(fileName)
	if err != nil {
		resp.Write(res(-1, "创建文件失败！", nil))
		req.Body.Close()
		return
	}
	defer newFile.Close()
	newFile.Write(bin)
	resp.Write(res(0, "success", nil))
	req.Body.Close()
}

func login(resp http.ResponseWriter, req *http.Request) {
	buf, err := base64.StdEncoding.DecodeString(req.Header.Get("Login"))
	if err != nil {
		resp.Write(res(-1, "登陆失败！", nil))
		return
	}
	pw := string(buf)
	if pw == "zzz" {
		h := md5.New()
		_, err = h.Write([]byte(pw))
		if err != nil {
			resp.Write(res(-1, "登陆失败！", nil))
			return
		}
		bs := h.Sum(nil)
		str := hex.EncodeToString(bs)
		resp.Write(res(0, "success", str))
	} else {
		resp.Write(res(-1, "密码错误！", nil))
	}
}
