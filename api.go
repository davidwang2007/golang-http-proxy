package main

//define some struct & interface

import (
	"david/util"
	"encoding/binary"
	"fmt"
	"io"
	"sync/atomic"
)

var channelIdCounter uint64

//ChannelDataChunk  represent the chunk data
//has two type: CTRL & DATA
//bytes structure:
// channel ChannelId: 8 bytes
// type: 1 byte
// remoteAddr: 2 + n bytes
// contentLength: 4 bytes
// content: n bytes
type ChannelDataChunk struct {
	ChannelId  uint64 //channelId
	Type       byte   // DATA 0, CTRL 1
	TargetAddr string //remote http service's ip address & port
	Content    []byte //datas
}

//pack pack the ChannelDataChunk to bytes
func (cdc *ChannelDataChunk) Write(writer io.Writer) (int, error) {
	addr := cdc.TargetAddr
	addrArr := ([]byte)(addr)
	buff := GetBackend(8) //make([]byte, 8) //make([]byte, 8+1+2+len(addrArr)+4+len(cdc.Content))
	defer Back2Backend(buff)
	defer Back2Backend(cdc.Content)
	var count int
	var n int
	var err error
	//ChannelId
	binary.BigEndian.PutUint64(buff[:8], cdc.ChannelId)
	if n, err = writer.Write(buff[:8]); err != nil {
		return count, err
	}
	count += n
	//type
	buff[0] = cdc.Type
	if n, err = writer.Write(buff[:1]); err != nil {
		return count, err
	}
	count += n
	var addrLen uint16 = uint16(len(addrArr))
	//address string length
	binary.BigEndian.PutUint16(buff[:2], addrLen)
	if n, err = writer.Write(buff[:2]); err != nil {
		return count, err
	}
	count += n
	//address string content
	if n, err = writer.Write(addrArr); err != nil {
		return count, err
	}
	count += n
	//content length
	var contentLen uint32 = uint32(len(cdc.Content))
	binary.BigEndian.PutUint32(buff[:4], contentLen)
	if n, err = writer.Write(buff[:4]); err != nil {
		return count, err
	}
	count += n
	//content
	if n, err = writer.Write(cdc.Content); err != nil {
		return count, err
	}
	count += n

	return count, nil
}

//Unpack unpack the bytes to ChannelDataChunk
func (chunk *ChannelDataChunk) Unpack(reader io.Reader) (count int, err error) {
	buff := GetBackend(8)
	defer Back2Backend(buff)
	var n int
	if n, err = reader.Read(buff[:8]); err != nil {
		return
	}
	if n != 8 {
		util.ColorPrinter.Red("[Unpack] should read 8 bytes A\n")
		return count, fmt.Errorf("Expected %d len bytes", 8)
	}
	count += n
	//ChannelId
	chunk.ChannelId = binary.BigEndian.Uint64(buff)
	//type
	if n, err = reader.Read(buff[:1]); err != nil {
		return
	}
	if n != 1 {
		util.ColorPrinter.Red("[Unpack] should read 1 bytes B\n")
		return count, fmt.Errorf("Expected %d len bytes", 1)
	}
	count += n
	chunk.Type = buff[0]
	//target addr
	if n, err = reader.Read(buff[:2]); err != nil {
		return
	}
	if n != 2 {
		util.ColorPrinter.Red("[Unpack] should read 2 bytes C\n")
		return count, fmt.Errorf("Expected %d len bytes", 2)
	}
	count += n
	var addrLen int = int(binary.BigEndian.Uint16(buff[:2]))
	tmp := GetBackend(addrLen) //make([]byte, addrLen)
	defer Back2Backend(tmp)
	if n, err = reader.Read(tmp); err != nil {
		return
	}
	if n != addrLen {
		util.ColorPrinter.Red("[Unpack] should read %d bytes D\n", addrLen)
		return count, fmt.Errorf("Expected %d len bytes", addrLen)
	}
	count += n
	chunk.TargetAddr = string(tmp)
	//content
	if n, err = reader.Read(buff[:4]); err != nil {
		return
	}
	if n != 4 {
		util.ColorPrinter.Red("[Unpack] should read 4 bytes E\n")
		return count, fmt.Errorf("Expected %d len bytes", 5)
	}
	count += n
	var contentLen int = int(binary.BigEndian.Uint32(buff[:4]))
	chunk.Content = GetBackend(contentLen) //make([]byte, contentLen)
	defer func() {
		if err != nil {
			Back2Backend(chunk.Content)
		}
	}()
	if n, err = reader.Read(chunk.Content); err != nil {
		return
	}
	if n != contentLen {
		util.ColorPrinter.Red("[Unpack] should read %d bytes F\n", contentLen)
		return count, fmt.Errorf("Expected %d len bytes", contentLen)
	}
	count += n
	return
}

//NextChannelId get next channelId
func NextChannelId() uint64 {
	atomic.AddUint64(&channelIdCounter, 1)
	return channelIdCounter
}

func CloseChunk(from *ChannelDataChunk) *ChannelDataChunk {
	cc := &ChannelDataChunk{
		from.ChannelId, 1, from.TargetAddr, ([]byte)("CLOSE"),
	}
	return cc
}

func EmptyRecover() {
	if r := recover(); r != nil {
		util.ColorPrinter.Cyan("[%s]: got panic %v\n", util.NowTime(), r)
	}
}
