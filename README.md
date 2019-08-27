
##two parts
* client
* server

*client & server share only 1 connection with multiple channels*

*A new request means a new channel*

*plain http proxy & https CONNECT request*

---

one channel from client remap to only one channel in server side

	fmt.Println("#start as client side")
	fmt.Println("c.exe -c -server ip:port [-port 9898] [-proxy ip:port]")
	fmt.Println("")
	fmt.Println("#start as server side")
	fmt.Println("c.exe -s [-port 9898]")

---

2018-02-12 15:53:27 
davidwang2006@aliyun.com 
