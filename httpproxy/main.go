package main

import (
	"bytes"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"

	"github.com/unixpickle/essentials"
)

var TargetURL url.URL

func main() {
	var target string
	var addr string
	flag.StringVar(&target, "target", "https://api.gelighting.com", "target URL base")
	flag.StringVar(&addr, "addr", ":8080", "listen address")
	flag.Parse()

	targetURL, err := url.Parse(target)
	essentials.Must(err)
	TargetURL = *targetURL

	http.HandleFunc("/", ProxyRequest)
	essentials.Must(http.ListenAndServe(addr, nil))
}

func ProxyRequest(w http.ResponseWriter, r *http.Request) {
	var data []byte
	if r.Body != nil {
		data, _ = ioutil.ReadAll(r.Body)
	}

	log.Printf("%s <- %s", r.URL.String(), string(data))

	tu := *r.URL
	tu.Host = TargetURL.Host
	tu.Scheme = TargetURL.Scheme
	proxyReq, err := http.NewRequest(r.Method, tu.String(), bytes.NewReader(data))
	if err != nil {
		log.Println(err)
		return
	}
	proxyReq.Header.Set("content-type", r.Header.Get("content-type"))
	proxyReq.Header.Set("cookie", r.Header.Get("cookie"))
	resp, err := (&http.Client{}).Do(proxyReq)
	data, _ = ioutil.ReadAll(resp.Body)
	log.Printf("%s (%s) -> %s", r.URL.String(), resp.Status, string(data))
	w.Header().Set("set-cookie", resp.Header.Get("cookie"))
	w.Header().Set("content-type", resp.Header.Get("content-type"))
	w.WriteHeader(resp.StatusCode)
	w.Write(data)
}
