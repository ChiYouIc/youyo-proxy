package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	youyoproxy "proxy"
	"strings"
)

var (
	port   string
	config string
)

func main() {
	parseFlag()

	proxy := youyoproxy.NewHttpProxy()

	list := readFilterResponseConfig(config)
	hs := make([]youyoproxy.RespHandler, len(list))
	for i, l := range list {
		p := l.Path
		r := l.Response
		hs[i] = youyoproxy.FuncRespHandler(func(resp *http.Response) *http.Response {
			path := resp.Request.URL.Path
			if strings.EqualFold(path, p) {
				proxy.Info("overwrite response: %s", p)
				marshal, _ := json.Marshal(r)
				buffer := bytes.NewBuffer(marshal)
				resp.Body = io.NopCloser(buffer)
				return resp
			}

			return resp
		})
	}
	proxy.RespHandlers = hs

	proxy.Info("Start youyo http proxy in %s", port)
	log.Fatal(http.ListenAndServe(":"+port, proxy))
}

func parseFlag() {
	flag.StringVar(&port, "p", "8888", "proxy server port")
	flag.StringVar(&config, "c", "./config.json", "proxy server config file")
	flag.Parse()
}

type FilterResponseConfig struct {
	Path     string      `json:"path"`
	Response interface{} `json:"response"`
}

func readFilterResponseConfig(path string) []FilterResponseConfig {
	file, err := os.OpenFile(path, os.O_RDONLY, 0755)
	if err != nil {
		log.Panicln("read config file error,", err)
	}

	all, err := io.ReadAll(file)
	if err != nil {
		log.Fatal("read config file error,", err)
	}

	var config []FilterResponseConfig
	err = json.Unmarshal(all, &config)
	if err != nil {
		log.Fatal("parse json error,", err)
	}

	return config
}
