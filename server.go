package main

import "net"
import "strings"
import "fmt"
import "net/http"
import "david/util"
import "sync/atomic"

var clientConnCounter int32

var FAILED_RESPONSE = &http.Response{
	Status:     "StatusServiceUnavailable d.w.",
	StatusCode: http.StatusServiceUnavailable,
	Request:    nil,
	ProtoMajor: 1,
	ProtoMinor: 1,
}

//StartServer start server side
func StartServer(port int) error {

	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	defer ln.Close()
	util.ColorPrinter.Cyan("[%s]: server side start listening @%d\n", util.NowTime(), port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go HandleClientConn(conn)
	}
	return nil
}

//HandleClientConn
//handle client side connection
//后续可考虑加密处理 username + password + tls
//now keep it simple
func HandleClientConn(conn net.Conn) error {

	defer EmptyRecover()

	atomic.AddInt32(&clientConnCounter, 1)
	util.ColorPrinter.Green("[%s]: GOT CLIENT CONN %s [active=%d]\n", util.NowTime(), conn.RemoteAddr(), clientConnCounter)
	defer func() {
		atomic.AddInt32(&clientConnCounter, -1)
		util.ColorPrinter.Yellow("[%s]: LOST CLIENT CONN %s [active=%d]\n", util.NowTime(), conn.RemoteAddr(), clientConnCounter)
		conn.Close()
	}()

	//this queue is for receive the [baidu] response
	queue := make(chan *ChannelDataChunk, 1<<7)
	//this map is for dispatch chunk, key is channelId-targetAddr
	conns := make(map[string](chan *ChannelDataChunk))

	go func() {
		for ck := range queue {
			if ck == nil {
				break
			}
			util.ColorPrinter.Green("[%s]: Write chunk back for %s [channelId=%d,type=%d,len=%d]\n", util.NowTime(), ck.TargetAddr, ck.ChannelId, ck.Type, len(ck.Content))
			_, err := ck.Write(conn)
			if err != nil {
				util.ColorPrinter.Red("[%s]: Write chunk to client side error!%v\n", util.NowTime(), err)
				break
			}
		}
		//close the connection
		conn.Close()
	}()

	for {
		chunk := &ChannelDataChunk{}
		if _, err := chunk.Unpack(conn); err != nil {
			util.ColorPrinter.Red("[%s]: Read chunk failed!%v\n", util.NowTime(), err)
			return err
		}
		//考虑CLOSE * chunk
		if chunk.TargetAddr == "*" && chunk.Type == 1 && (string)(chunk.Content) == "CLOSE" {
			cid := fmt.Sprintf("%d", chunk.ChannelId)
			for k, v := range conns {
				if strings.Index(k, cid) == 0 {
					util.ColorPrinter.Yellow("[%s]: Receive wildcard Close Chunk for %s\n", util.NowTime(), k)
					v <- chunk
					delete(conns, k)
				}
			}

			continue
		}

		k := fmt.Sprintf("%d-%s", chunk.ChannelId, chunk.TargetAddr)
		if conns[k] == nil {
			conns[k] = make(chan *ChannelDataChunk, 1<<7)
			go startConn(queue, conns[k])
		}
		util.ColorPrinter.Green("[%s]: Receieve chunk » %s [len=%d]\n", util.NowTime(), chunk.TargetAddr, len(chunk.Content))
		conns[k] <- chunk

	}

}

//start conn to baidu
//queue 写回去的，用于返回到client side
//outQueue 由总开头解析得到后放入，此方法中遍历读出，然后写出去
func startConn(queue chan *ChannelDataChunk, outQueue chan *ChannelDataChunk) (err error) {
	var chunk *ChannelDataChunk = <-outQueue
	if chunk == nil {
		return fmt.Errorf("Unexpected nil chunk")
	}
	defer func() {
		//返回一个关闭的chunk
		queue <- CloseChunk(chunk)
		close(outQueue)
		util.ColorPrinter.Yellow("[%s]: Close %s\n", util.NowTime(), chunk.TargetAddr)
	}()
	var conn net.Conn

	if conn, err = net.Dial("tcp", chunk.TargetAddr); err != nil {
		util.ColorPrinter.Red("[%s]: Dial %s failed! %v\n", util.NowTime(), chunk.TargetAddr, err)
		return
	}
	util.ColorPrinter.Green("[%s]: CONN » %s OPENED\n", util.NowTime(), chunk.TargetAddr)
	defer conn.Close()
	if chunk.Type == 0 {
		if _, err = conn.Write(chunk.Content); err != nil {
			return err
		}
	} else if chunk.Type == 1 && string(chunk.Content) == "CLOSE" {
		return nil
	}

	//read from the baidu socket response, wrap the content to ChannelDataChunk
	go func(chunk *ChannelDataChunk) {
		for {
			buff := GetBackend(1 << 16) //make([]byte, 1<<16) //64Kb
			n, err := conn.Read(buff)
			if err != nil {
				util.ColorPrinter.Yellow("[%s]: Read from %s failed! %v\n", util.NowTime(), chunk.TargetAddr, err)
				return
			}
			ck := &ChannelDataChunk{
				chunk.ChannelId, 0, chunk.TargetAddr, buff[:n],
			}
			queue <- ck
		}
	}(chunk)

	for chunk = range outQueue {
		if chunk == nil {
			return fmt.Errorf("Unexpected nil chunk")
		}
		if chunk.Type == 1 {
			if string(chunk.Content) == "CLOSE" {
				return nil
			}
		} else if _, err = conn.Write(chunk.Content); err != nil {
			return err
		}
		Back2Backend(chunk.Content)
	}

	return nil
}
