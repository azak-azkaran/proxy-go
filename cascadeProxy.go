package main

import (
	"encoding/base64"
	"github.com/azak-azkaran/cascade/utils"
	"github.com/azak-azkaran/goproxy"
	"github.com/orcaman/concurrent-map"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type cascadeProxy struct {
}

type HostConfig struct {
	addr      string
	reg       *regexp.Regexp
	proxyAddr string
	regString string
}

var CASCADE cascadeProxy
var LoginRequired bool
var HostList cmap.ConcurrentMap = cmap.New()

const (
	httpPrefix      = "http://"
	ProxyAuthHeader = "Proxy-Authorization"
)

func SetBasicAuth(username, password string, req *http.Request) {
	req.Header.Set(ProxyAuthHeader, "Basic "+basicAuth(username, password))
}

func ClearHostList() {
	for content := range HostList.IterBuffered() {
		HostList.Remove(content.Key)
	}
}

func basicAuth(username, password string) string {
	var builder strings.Builder
	builder.WriteString(username)
	builder.WriteString(":")
	builder.WriteString(password)
	return base64.StdEncoding.EncodeToString([]byte(builder.String()))
}

func HandleDirectHttpRequest(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	var resp *http.Response
	var err error
	if req.URL.Scheme == "" {
		resp, err = utils.GetResponse("", req.Host)
	} else {
		resp, err = utils.GetResponse("", req.URL.Scheme+"://"+req.Host)
	}
	if err != nil {
		utils.Error.Println("Problem while trying direct connection: ", err)
	}
	if resp != nil {
		return req, resp
	} else {
		return req, nil
	}
}

func AddDifferentProxyConnection(host string, proxyAddr string) {
	var value HostConfig
	val, ok := HostList.Get(proxyAddr)
	if ok {
		value = val.(HostConfig)
		value.regString += "|"
	}

	value.regString += ".*" + host + ".*"
	value.reg = regexp.MustCompile(value.regString)
	value.addr = host
	if !strings.HasPrefix(proxyAddr, httpPrefix) && len(proxyAddr) > 0 {
		value.proxyAddr = httpPrefix + proxyAddr
	} else {
		value.proxyAddr = proxyAddr
	}
	utils.Info.Println("Adding Redirect: ", proxyAddr, " for: ", host, " Regex:", value.regString)
	HostList.Set(proxyAddr, value)
}

func AddDirectConnection(host string) {
	AddDifferentProxyConnection(host, "")
}

func CustomConnectDial(proxyURL string, connectReqHandler func(req *http.Request), server *goproxy.ProxyHttpServer) func(network string, addr string) (net.Conn, error) {
	return func(network string, addr string) (conn net.Conn, e error) {

		for content := range HostList.IterBuffered() {
			val := content.Val.(HostConfig)
			if val.reg.MatchString(addr) {
				utils.Info.Println("Matching Host found: ", addr, " for: ", val.regString)
				utils.Info.Println("Regex: ", val.reg)

				if len(val.proxyAddr) != 0 {
					utils.Info.Println("Redirect to: ", val.proxyAddr)
					f := server.NewConnectDialToProxyWithHandler(val.proxyAddr, connectReqHandler)
					return f(network, addr)
				}
				utils.Info.Println("Using direct connection")
				return net.DialTimeout(network, addr, 5*time.Second)
			}
		}
		f := server.NewConnectDialToProxyWithHandler(proxyURL, connectReqHandler)
		return f(network, addr)
	}
}

func parseProxyUrl(proxyURL string) (*url.URL, error) {
	if strings.HasPrefix(proxyURL, httpPrefix) {
		return url.Parse(proxyURL)
	} else {
		return url.Parse(httpPrefix + proxyURL)
	}
}

func CustomProxy(proxyURL string) func(req *http.Request) (*url.URL, error) {
	return func(reg *http.Request) (*url.URL, error) {
		for content := range HostList.IterBuffered() {
			val := content.Val.(HostConfig)
			if val.reg.MatchString(reg.Host) {
				utils.Info.Println("Matching Host found: ", reg.Host, " for: ", val.regString)
				utils.Info.Println("Regex: ", val.reg)

				if len(val.proxyAddr) != 0 {
					utils.Info.Println("with redirect to: ", val.proxyAddr)
					f, err := parseProxyUrl(val.proxyAddr)
					return f, err
				} else {
					utils.Info.Println("Using direct connection")
					return nil, nil
				}
			}
		}

		f, err := parseProxyUrl(proxyURL)
		return f, err
	}
}

func (cascadeProxy) Run(verbose bool, proxyURL string, username string, password string) *goproxy.ProxyHttpServer {
	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = verbose
	proxy.Logger = utils.Info
	proxy.KeepHeader = true

	proxy.Tr.Proxy = CustomProxy(proxyURL)
	var connectReqHandler func(req *http.Request)

	if len(username) > 0 {
		LoginRequired = true
		connectReqHandler = func(req *http.Request) {
			SetBasicAuth(username, password, req)
		}
	} else {
		LoginRequired = false
		connectReqHandler = nil
	}

	proxy.ConnectDial = CustomConnectDial(proxyURL, connectReqHandler, proxy)

	proxy.OnRequest().Do(goproxy.FuncReqHandler(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		if LoginRequired {
			SetBasicAuth(username, password, req)
		}
		return req, nil
	}))
	return proxy
}
