package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const timeout = 5

func main() {
	//程序的核心,中转站
	log.SetFlags(log.Ldate | log.Lmicroseconds)
	printWelcome()
	args := os.Args
	argc := len(os.Args)
	if argc <= 2 {
		printHelp()
		os.Exit(0)
	}
	switch args[1] {
	case "-listen":
		if argc < 3 {
			log.Fatalln(`-listen need two arguments, like "nb -listen 1997 2017".`)
		}
		port1 := checkPort(args[2])
		port2 := checkPort(args[3])
		log.Println("[√]", "start to listen port:", port1, "and port:", port2)
		port2port(port1, port2)
		break
	case "-slave":
		if argc < 3 {
			log.Fatalln(`-slave need two arguments, like "nb -slave 127.0.0.1:3389 8.8.8.8:1997".`)
		}
		var address1, address2 string
		//checkIp(args[2])
		if checkIp(args[2]) {
			address1 = args[2]
		}
		//checkIp(args[3])
		if checkIp(args[3]) {
			address2 = args[3]
		}
		log.Println("[√]", "start to connect address:", address1, "and address:", address2)
		host2host(address1, address2)
		break
	default:
		printHelp()
	}
}

func printWelcome() {
	fmt.Println("+----------------------------------------------------------------+")
	fmt.Println("| Welcome to use NATBypass Ver1.0.0 .                            |")
	fmt.Println("| Code by cw1997 at 2017-10-19 03:59:51                          |")
	fmt.Println("| If you have some problem when you use the tool,                |")
	fmt.Println("| please submit issue at : https://github.com/cw1997/NATBypass . |")
	fmt.Println("+----------------------------------------------------------------+")
	fmt.Println()
	// sleep one second because the fmt is not thread-safety.
	// if not to do this, fmt.Print will print after the log.Print.
	// time.Sleep(time.Second)
}
func printHelp() {
	fmt.Println(`usage: "-listen port1 port2" example: "nb -listen 1997 2017" `)
	fmt.Println(`       "-slave ip1:port1 ip2:port2" example: "nb -slave 127.0.0.1:3389 8.8.8.8:1997" `)
	fmt.Println(`============================================================`)
	fmt.Println(`if you want more help, please read "README.md". `)
}

func checkPort(port string) string {
	PortNum, err := strconv.Atoi(port)
	if err != nil {
		log.Fatalln("[x]", "port should be a number")
	}
	if PortNum < 1 || PortNum > 65535 {
		log.Fatalln("[x]", "port should be a number and the range is [1,65536)")
	}
	return port
}

func checkIp(address string) bool {
	ipAndPort := strings.Split(address, ":")
	if len(ipAndPort) != 2 {
		log.Fatalln("[x]", "address error. should be a string like [ip:port]. ")
	}
	ip := ipAndPort[0]
	port := ipAndPort[1]
	checkPort(port)
	pattern := `^(\d{1,2}|1\d\d|2[0-4]\d|25[0-5])\.(\d{1,2}|1\d\d|2[0-4]\d|25[0-5])\.(\d{1,2}|1\d\d|2[0-4]\d|25[0-5])\.(\d{1,2}|1\d\d|2[0-4]\d|25[0-5])$`
	ok, err := regexp.MatchString(pattern, ip)
	if err != nil || !ok {
		log.Fatalln("[x]", "ip error. ")
	}
	return ok
}

func port2port(port1 string, port2 string) {
	// 启动两台服务器
	listen1 := startServer("0.0.0.0:" + port1)
	listen2 := startServer("0.0.0.0:" + port2)
	log.Println("[√]", "listen port:", port1, "and", port2, "success. waiting for client...")
	for {
		conn1 := accept(listen1) //首先接受配置的第一个端口,conn1是公网ip与穿透内网主机之间的桥梁
		conn2 := accept(listen2) //接受配置的第二个端口,这是暴露在公网,提供访问的port
		if conn1 == nil || conn2 == nil {
			log.Println("[x]", "accept client faild. retry in ", timeout, " seconds. ")
			time.Sleep(timeout * time.Second)
			continue
		}
		forward(conn1, conn2) //转发拷贝
	}
}

func host2host(address1, address2 string) {
	for {
		log.Println("[+]", "try to connect host:["+address1+"] and ["+address2+"]")
		var host1, host2 net.Conn
		var err error
		for {
			// 第一层循环,连接第一个ip(本地)
			host1, err = net.Dial("tcp", address1) //使用tcp协议,连接address1
			if err == nil {
				log.Println("[→]", "connect ["+address1+"] success.")
				break
			} else {
				log.Println("[x]", "connect target address ["+address1+"] faild. retry in ", timeout, " seconds. ")
				time.Sleep(timeout * time.Second)
			}
		}
		for {
			// 第二层循环,连接第二个ip(公网ip),让其他内网主机通过公网穿透到本机,从而实现访问内网主机端口
			host2, err = net.Dial("tcp", address2)
			if err == nil {
				log.Println("[→]", "connect ["+address2+"] success.")
				break
			} else {
				log.Println("[x]", "connect target address ["+address2+"] faild. retry in ", timeout, " seconds. ")
				time.Sleep(timeout * time.Second)
			}
		}
		//持续转发
		forward(host1, host2)
	}
}

func startServer(address string) net.Listener {
	log.Println("[+]", "try to start server on:["+address+"]")
	server, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalln("[x]", "listen address ["+address+"] faild.")
	}
	log.Println("[√]", "start listen at address:["+address+"]")
	return server
}

func accept(listener net.Listener) net.Conn {
	conn, err := listener.Accept()
	if err != nil {
		log.Println("[x]", "accept connect ["+conn.RemoteAddr().String()+"] faild.", err.Error())
		return nil
	}
	log.Println("[√]", "accept a new client. remote address:["+conn.RemoteAddr().String()+"], local address:["+conn.LocalAddr().String()+"]")
	return conn
}

//方法会一直阻塞
func forward(conn1 net.Conn, conn2 net.Conn) {
	// 对conn1和conn2进行转发
	// conn1:与本地配置端口连接
	// conn2:与远程配置端口连接
	log.Printf("[+] start transmit. [%s],[%s] <-> [%s],[%s] \n", conn1.LocalAddr().String(), conn1.RemoteAddr().String(), conn2.LocalAddr().String(), conn2.RemoteAddr().String())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for true {
			_, _ = io.Copy(conn1, conn2)
		}
	}()
	go func() {
		for true {
			_, _ = io.Copy(conn2, conn1)
		}
	}()
	wg.Wait()
}
