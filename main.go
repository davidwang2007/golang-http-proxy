package main

import "fmt"
import "david/util"
import "flag"

func main() {

	var isClient bool
	var isServer bool
	var port int
	var proxy string
	var server string

	flag.BoolVar(&isClient, "c", false, "client side")
	flag.BoolVar(&isServer, "s", false, "server side")

	flag.StringVar(&proxy, "proxy", "", "proxy server")
	flag.StringVar(&server, "server", "", "remote server side ip:port")
	flag.IntVar(&port, "port", 9898, "listening port")
	flag.Parse()

	if isClient {
		if server == "" {
			util.ColorPrinter.Red("[%s]: Not Enough Params\n", util.NowTime())
			usage()
			return
		}
		StartClient(port, server, proxy)
	} else if isServer {
		StartServer(port)
	} else {
		usage()
	}

}

func usage() {
	fmt.Println("#start as client side")
	fmt.Println("c.exe -c -server ip:port [-port 9898] [-proxy ip:port]")
	fmt.Println("")
	fmt.Println("#start as server side")
	fmt.Println("c.exe -s [-port 9898]")
}
