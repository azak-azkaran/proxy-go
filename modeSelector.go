package main

import (
	"github.com/azak-azkaran/cascade/utils"
	"github.com/azak-azkaran/goproxy"
	"strings"
	"time"
)

type config struct {
	CascadeMode       bool
	Username          string
	Password          string
	ProxyURL          string
	LocalPort         string
	Verbose           bool
	CascadeFunction   func()
	DirectFunction    func()
	ShutdownFunction  func()
	CheckAddress      string
	Health            time.Duration
	ProxyRedirectList []string
}

var CONFIG config

func switchMode(server *goproxy.ProxyHttpServer, mode string) {
	utils.Info.Println("Shutdown of current Server")
	ShutdownCurrentServer()
	utils.Info.Println("Creating Server")
	CreateServer(server, "localhost", CONFIG.LocalPort)
	utils.Info.Println("Starting Server in: ", mode)
	RunServer()
}

func CreateConfig(localPort string, proxyUrl string, username string, password string, checkAddress string, healthTime int, skipHosts string) {
	CONFIG.LocalPort = localPort
	CONFIG.ProxyURL = proxyUrl
	CONFIG.Username = username
	CONFIG.Password = password
	CONFIG.Verbose = true
	CONFIG.ProxyRedirectList = strings.Split(skipHosts, ",")

	CONFIG.DirectFunction = func() {
		switchMode(DIRECT.Run(CONFIG.Verbose), "Direct Mode")
	}
	CONFIG.CascadeFunction = func() {
		server := CASCADE.Run(CONFIG.Verbose, CONFIG.ProxyURL, CONFIG.Username, CONFIG.Password)
		switchMode(server, "Cascade Mode")
		HandleCustomProxies(CONFIG.ProxyRedirectList)
	}
	CONFIG.CheckAddress = checkAddress
	CONFIG.Health = time.Duration(healthTime) * time.Second
}

func HandleCustomProxies(list []string) {
	if len(list) > 0 {
		return
	}

	for i := 0; i < len(list); i++ {
		rule := list[i]
		if strings.Contains(rule, "->") {
			split := strings.Split(rule, "->")
			AddDifferentProxyConnection(split[0], split[1])
		} else {
			AddDirectConnection(rule)
		}
	}
}

func ModeSelection(checkAddress string) {
	var success bool
	utils.Info.Println("Running check on: ", checkAddress)
	rep, err := utils.GetResponse("", checkAddress)
	if err != nil {
		utils.Error.Println("Error while checking,", checkAddress, " , ", err)
		success = false
	} else {
		if rep.StatusCode == 200 {
			success = true
		} else {
			utils.Info.Println("Response was: ", rep.Status)
			success = false
		}
	}

	if CONFIG.Verbose {
		utils.Info.Println("Check returns: ", success)
		if CONFIG.CascadeMode {
			utils.Info.Println("Current Mode: CascadeMode")
		} else {
			utils.Info.Println("Current Mode: DirectMode")
		}
	}
	ChangeMode(success)
}

func ChangeMode(selector bool) {
	if (selector && CONFIG.CascadeMode) || (selector && CurrentServer == nil) || len(CONFIG.ProxyURL) == 0 {
		if len(CONFIG.ProxyURL) == 0 && !selector {
			utils.Error.Println("ProxyURL was not set so staying in DirectMode")
		}
		// switch to direct mode
		utils.Info.Println("switch to: DirectMode")
		CONFIG.CascadeMode = false
		go CONFIG.DirectFunction()
	} else if (!selector && !CONFIG.CascadeMode) || (!selector && CurrentServer == nil) {
		// switch to cascade mode
		utils.Info.Println("switch to: CascadeMode")
		CONFIG.CascadeMode = true
		go CONFIG.CascadeFunction()
	}
}
