package main

import "david/util"

//bytes pool backend

var bytesPool chan []byte

//init init bytes pool 8M
func init() {
	bytesPool = make(chan []byte, 128)
	for i := 0; i < 128; i += 1 { // total 128 * 64k = 8M
		bytesPool <- make([]byte, 1<<16) // 64k
	}
}

//GetBackend get back end bytes
func GetBackend(len int) []byte {
	select {
	case arr := <-bytesPool:
		if cap(arr) >= len {
			return arr[:len]
		} else {
			return make([]byte, len)
		}
	default:
		util.ColorPrinter.Yellow("[%s]: make new bytes for pool\n", util.NowTime())
		return make([]byte, len)
	}
}

//Back2Backend give the bytes back
func Back2Backend(arr []byte) {
	select {
	case bytesPool <- arr:
	default:
		util.ColorPrinter.Yellow("[%s]:  Pool is full, we just drop the bytes\n", util.NowTime())
		arr = nil
	}

}
