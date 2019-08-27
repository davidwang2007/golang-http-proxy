package main

import "testing"
import "bytes"

//TestChannelDataChunk
func TestChannelDataChunk(t *testing.T) {

	/*
		addr, err := net.ResolveTCPAddr("tcp", "www.suning.com:80")
		if err != nil {
			t.Fatalf("resolve addr failed! %v\n", err)
		}
	*/
	var err error

	buff := bytes.NewBuffer(nil)
	cdc := &ChannelDataChunk{
		ChannelId:  11,
		Type:       1,
		TargetAddr: "abcd",
		Content:    []byte{1, 2, 3, 4, 5},
	}

	if _, err = cdc.Write(buff); err != nil {
		t.Fatalf("write got error %v\n", err)
	}

	t.Logf("Got packed bytes %v\n", buff.Bytes())
	t.Logf("---------------------------------")
	cdc2 := &ChannelDataChunk{}
	if _, err = cdc2.Unpack(buff); err != nil {
		t.Fatalf("Unpack failed %v\n", err)
	}
	t.Logf("Unpack result %v\n", cdc2)

}

func TestSlice(t *testing.T) {
	arr := make([]byte, 0, 9)
	arr = arr[:9]
	arr[0] = 3
	t.Logf("%v\n", arr)
	arr = arr[:9]
	t.Logf("%v\n", arr)
}
