package main

//后续优化可将[]byte 从池中创建与归还

import "net"
import "fmt"
import "bufio"
import "time"
import "strings"
import "net/http"
import "bytes"
import "david/util"
import "sync/atomic"

var browserConnCounter int32

var channelChunkChanMap = make(map[uint64](chan *ChannelDataChunk))

var ServerConnOpened bool

//StartClient start client
//port: listening port
//server: the server's ip:port
//proxy: the http proxy server if necessary
func StartClient(port int, server string, proxy string) error {
	//indicate to exit
	chanClose := make(chan int)
	queue := make(chan *ChannelDataChunk, 1024)
	//open conn to server
	go func() {
		for {
			if err := OpenConn2Server(server, proxy, queue, chanClose); err != nil {
				ServerConnOpened = false
				util.ColorPrinter.Red("[%s]: OPEN CONN TO SERVER, SOMETHING FAILED!%v\n", util.NowTime(), err)
				util.ColorPrinter.Cyan("[%s]: SLEEP 5 SECONDS TO RETRY OPEN CONN TO SERVER\n", util.NowTime())
			}
		}
	}()

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	defer ln.Close()
	util.ColorPrinter.Cyan("[%s]: client side start listening @%d\n", util.NowTime(), port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go HandleBrowserConn(conn, queue)
	}

	return nil

}

//OpenConn2Server
//open connection to server
//maybe through a proxy
func OpenConn2Server(server, proxy string, queue chan *ChannelDataChunk, chanClose chan int) (err error) {
	var pConn net.Conn
	if proxy == "" {
		if pConn, err = net.DialTimeout("tcp", server, time.Second); err != nil {
			return
		}
		defer pConn.Close()
	} else {
		if pConn, err = net.DialTimeout("tcp", proxy, time.Second); err != nil {
			return
		}
		defer pConn.Close()
		//dial the proxy server
		if _, err = fmt.Fprintf(pConn, "CONNECT %s HTTP/1.1\r\nHOST: %s\r\nUser-Agent: gurl.dw/0.1\r\nProxy-Connection: Keep-Alive\r\n\r\n", server, server); err != nil {
			return err
		}
		if resp, err := http.ReadResponse(bufio.NewReader(pConn), nil); err != nil {
			return err
		} else if resp.StatusCode != http.StatusOK {
			util.ColorPrinter.Red("[%s]: open proxy failed! statusCode=%d\n", util.NowTime(), resp.StatusCode)
			return fmt.Errorf("OPEN PROXY FAILED! StatusCode=%d", resp.StatusCode)
		}
	}

	util.ColorPrinter.Green("[%s]: CONNECT to remote server [%s] SUCCEED!\n", util.NowTime(), server)
	ServerConnOpened = true

	//try read & dispatch to channelChunkChanMap
	go func() {
		for {
			cdc := &ChannelDataChunk{}
			if _, err := cdc.Unpack(pConn); err != nil {
				util.ColorPrinter.Red("[%s]: read from server failed! %v\n", util.NowTime(), err)
				return
			}
			util.ColorPrinter.Cyan("[%s]: Got chunk [channelId=%d,type=%d,len=%d]\n", util.NowTime(), cdc.ChannelId, cdc.Type, len(cdc.Content))
			if channelChunkChanMap[cdc.ChannelId] == nil {
				if cdc.Type != 1 {
					util.ColorPrinter.Yellow("[%s]: receive chunk for channel %d, [type=%d,len=%d] but not found. Maybe it's closed\n", util.NowTime(), cdc.ChannelId, cdc.Type, len(cdc.Content))
				}
			} else {
				channelChunkChanMap[cdc.ChannelId] <- cdc
			}
		}

	}()

	//Now we got the connection
	//we can wrap it to tls if necessary
	//tls.Client(pConn,nil)
	//but now, we keep it simple, plain tcp socket

for_out:
	for {
		select {
		case chunk := <-queue:
			if chunk == nil {
				util.ColorPrinter.Red("[%s]: Receive inappropriate chunk %v, we close\n", util.NowTime(), chunk)
				break for_out
			}
			_, err := chunk.Write(pConn)
			if err != nil {
				return err
			}
		case <-chanClose:
			util.ColorPrinter.Red("[%s]: Receive close chan signal\n", util.NowTime())
			break for_out
		}
	}
	return nil

}

//HandleBrowserConn
//handle broswer http proxy connection
//wrap the request to channel chunk data
func HandleBrowserConn(conn net.Conn, queue chan *ChannelDataChunk) error {
	atomic.AddInt32(&browserConnCounter, 1)

	//each browser connection remap to one channel [channelId]
	var myChannelId uint64 = NextChannelId()
	util.ColorPrinter.Green("[%s]: GOT CONN %s [channelId=%d] [active=%d]\n", util.NowTime(), conn.RemoteAddr(), myChannelId, browserConnCounter)
	defer func() {
		atomic.AddInt32(&browserConnCounter, -1)
		util.ColorPrinter.Yellow("[%s]: LOST CONN %s [channelId=%d] [active=%d]\n", util.NowTime(), conn.RemoteAddr(), myChannelId, browserConnCounter)
	}()

	defer conn.Close()

	var myChunkChan = make(chan *ChannelDataChunk, 1<<7) //128 pool size
	channelChunkChanMap[myChannelId] = myChunkChan
	defer func() {
		close(myChunkChan)
		delete(channelChunkChanMap, myChannelId)
	}()

	//send close channel to server side
	defer func() {
		ck := &ChannelDataChunk{
			myChannelId, 1, "*", ([]byte)("CLOSE"),
		}
		queue <- ck
	}()

	br := bufio.NewReader(conn)

	go func() {
		//read the chan, then write it out to client
		for chunk := range myChunkChan {
			if chunk == nil {
				util.ColorPrinter.Yellow("[%s]: channel [%d] has no more chunk data\n", util.NowTime(), myChannelId)
				break
			}

			if chunk.Type == 1 {
				if (string)(chunk.Content) == "CLOSE" {
					conn.Close()
					break
				}
				continue
			}

			if _, err := conn.Write(chunk.Content); err != nil {
				util.ColorPrinter.Yellow("[%s]: write channel [%d] chunk to browser,got error %v\n", util.NowTime(), myChannelId, err)
				break
			}
		}
	}()

	for {
		req, err := http.ReadRequest(br)
		if err != nil {
			util.ColorPrinter.Red("[%s]: Read Request Error!%v\n", util.NowTime(), err)
			return err
		}
		if req.RemoteAddr == "" {
			req.RemoteAddr = conn.RemoteAddr().String()
		}
		if req.Method == "CONNECT" {
			return handleHttpsConn(myChannelId, req.Host, conn, queue)
		}
		if err = handleHttpRequest(myChannelId, req, queue); err != nil {
			return err
		}
	}
}

//handleHttpsConn handle https conneciton
func handleHttpsConn(channelId uint64, host string, conn net.Conn, queue chan *ChannelDataChunk) error {
	var err error
	var n int
	//写 established
	if _, err = conn.Write(([]byte)("HTTP/1.1 200 Connection Established\r\nProxy-Agent: GoProxy-D.W.\r\n\r\n")); err != nil {
		return err
	}
	//read data chan it to queue
	for {
		if !ServerConnOpened {
			return fmt.Errorf("SERVER CONN LOST")
		}
		buff := GetBackend(1 << 16) //make([]byte, 1<<16) //64Kb buffer size
		n, err = conn.Read(buff)
		if err != nil {
			Back2Backend(buff)
			return err
		}
		chunk := &ChannelDataChunk{
			channelId, 0, host, buff[:n],
		}
		util.ColorPrinter.Green("[%s]: POOL Https chunk [%s] [channelId=%d] [type=%d,len=%d]to queue\n", util.NowTime(), host, channelId, chunk.Type, len(chunk.Content))
		queue <- chunk
	}

}

//handleHttpRequest handle plain http proxy request
func handleHttpRequest(channelId uint64, req *http.Request, queue chan *ChannelDataChunk) error {
	if !ServerConnOpened {
		return fmt.Errorf("SERVER CONN LOST")
	}
	var host = req.Host
	if strings.Index(host, ":") == -1 {
		host += ":80"
	}
	for k, v := range req.Header {
		if strings.Index(k, "Proxy") == 0 {
			util.ColorPrinter.Cyan("[%s]: Remove unnecessary proxy header %s: %s\n", util.NowTime(), k, v)
			req.Header.Del(k)
		}
	}
	content := bytes.NewBuffer(nil)
	if err := req.Write(content); err != nil {
		return err
	}
	chunk := &ChannelDataChunk{
		channelId, 0, host, content.Bytes(),
	}
	util.ColorPrinter.Green("[%s]: POOL Http Request [%s%s] [channelId=%d] to queue\n", util.NowTime(), host, req.URL.Path, channelId)
	queue <- chunk

	return nil
}
