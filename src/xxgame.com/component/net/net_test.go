package net

import (
	"fmt"
	"testing"
	"time"
)

import (
	"github.com/stretchr/testify/assert"
	"xxgame.com/component/log"
)

type msg struct {
	text string
}

func newMsg(text string) *msg {
	m := new(msg)
	m.text = text
	return m
}

func encode(m *msg) []byte {
	b := make([]byte, len(m.text))
	copy(b, []byte(m.text))
	return b
}

func decode(n *NetMsg) *msg {
	m := new(msg)
	m.text = string(n.Data)
	return m
}

func testTcpCommu(server, client *Commu) {
	var serverConn *Conn
	var clientConn *Conn
	serverOk := make(chan int, 1)
	clientOk := make(chan int, 1)
	serverExit := make(chan int, 1)
	clientExit := make(chan int, 1)

	server.Restart(func(conn *Conn) {
		serverConn = conn
		fmt.Printf("server establish connection %v\n", serverConn)
		serverOk <- 0
	})
	client.Restart(func(conn *Conn) {
		clientConn = conn
		fmt.Printf("client establish connection %v\n", clientConn)
		clientOk <- 0
	})
	<-serverOk
	<-clientOk

	go func() {
		c := time.After(time.Second * 5)
		t := time.Tick(time.Second)
	LOOP:
		for {
			select {
			case <-c:
				break LOOP
			case <-t:
			default:
				continue
			}

			msg := newMsg("hello @ " + time.Now().String())
			if clientConn.SendNormalMsg(encode(msg), 0) == nil {
				fmt.Println("client sent:", msg.text)
				count := 3
				for {
					b, e := clientConn.RecvMsg(time.Millisecond * 5)
					if e == nil {
						msg := decode(b)
						fmt.Println("client recv:", msg.text)
						continue LOOP
					} else if count > 0 {
						count--
						time.Sleep(time.Millisecond * 5)
					} else {
						continue LOOP
					}
				}
			}
		}
		fmt.Println("client quit")
		clientExit <- 0
	}()
	go func() {
		c := time.After(time.Second * 5)
		t := time.Tick(time.Second)
	LOOP:
		for {
			select {
			case <-c:
				break LOOP
			case <-t:
			default:
				continue
			}

			b, e := serverConn.RecvMsg(time.Millisecond * 5)
			if e == nil {
				msg := decode(b)
				fmt.Println("server recv:", msg.text)
				msg = newMsg("world @ " + time.Now().String())
				if serverConn.SendNormalMsg(encode(msg), 0) == nil {
					fmt.Println("server sent:", msg.text)
				}
			}
		}
		fmt.Println("server quit")
		serverExit <- 0
	}()
	<-serverExit
	<-clientExit
}

func testUdpCommu(server, client *Commu) {
	var serverConn *Conn
	var clientConn *Conn
	serverOk := make(chan int, 1)
	clientOk := make(chan int, 1)
	serverExit := make(chan int, 1)
	clientExit := make(chan int, 1)

	server.Restart(func(conn *Conn) {
		serverConn = conn
		fmt.Printf("server establish connection %v\n", serverConn)
		serverOk <- 0
	})
	client.Restart(func(conn *Conn) {
		clientConn = conn
		fmt.Printf("client establish connection %v\n", clientConn)
		clientOk <- 0
	})
	<-serverOk
	<-clientOk

	go func() {
		c := time.After(time.Second * 5)
		t := time.Tick(time.Second)
	LOOP:
		for {
			select {
			case <-c:
				break LOOP
			case <-t:
			default:
				continue
			}

			msg := newMsg("hello @ " + time.Now().String())
			if clientConn.SendNormalMsg(encode(msg), 0) == nil {
				fmt.Println("client sent:", msg.text)
			}
		}
		fmt.Println("client quit")
		clientExit <- 0
	}()
	go func() {
		c := time.After(time.Second * 5)
		t := time.Tick(time.Second)
	LOOP:
		for {
			select {
			case <-c:
				break LOOP
			case <-t:
			default:
				continue
			}

			b, e := serverConn.RecvMsg(time.Millisecond * 5)
			if e == nil {
				msg := decode(b)
				fmt.Println("server recv:", msg.text)
			}
		}
		fmt.Println("server quit")
		serverExit <- 0
	}()
	<-serverExit
	<-clientExit
}

func TestTCP(t *testing.T) {
	SetDebugLevel(log.DEBUG)
	SetLogDir("./logs")
	defer log.FlushAll()
	addrs := []string{"127.0.0.1:11000"}
	server := NewCommu(true, "tcp", addrs, 1000, 1000)
	if !assert.NotNil(t, server, "create server failed!") {
		return
	}
	client := NewCommu(false, "tcp", addrs, 1000, 1000)
	if !assert.NotNil(t, client, "create client failed!") {
		return
	}

	// 启动tcp服务器和客户端
	fmt.Printf("\n启动tcp服务器和客户端\n")
	var serverConn *Conn
	serverOk := make(chan int, 1)
	server.Start(func(conn *Conn) {
		serverConn = conn
		fmt.Printf("server get connection %v\n", serverConn)
		serverOk <- 0
	})
	var clientConn *Conn
	clientOk := make(chan int, 1)
	client.Start(func(conn *Conn) {
		clientConn = conn
		fmt.Printf("client get connection %v\n", clientConn)
		clientOk <- 0
	})
	<-serverOk
	<-clientOk
	clientIP, clientPort, _ := clientConn.LocalAddr()
	clientOtherIP, clientOtherPort, _ := clientConn.RemoteAddr()
	fmt.Printf("client info: %s:%d, %s:%d\n", clientIP, clientPort, clientOtherIP, clientOtherPort)
	serverIP, serverPort, _ := serverConn.LocalAddr()
	serverOtherIP, serverOtherPort, _ := serverConn.RemoteAddr()
	fmt.Printf("server info: %s:%d, %s:%d\n", serverIP, serverPort, serverOtherIP, serverOtherPort)

	// 重启客户端，服务器检测到客户端的断线
	fmt.Printf("\n重启tcp客户端，服务器检测到客户端的断线\n")
	detectDisconOk := make(chan int, 1)
	serverConn.RegisterDisconnCallback(func(arg interface{}) {
		fmt.Printf("detected client disconnection %s\n", arg.(*Conn))
		detectDisconOk <- 0
	}, serverConn)
	client.Stop()
	<-detectDisconOk
	if !assert.Equal(t, BY_OTHER, serverConn.CloseReason(), "should be closed by other end!") {
		return
	}
	client.Start(func(conn *Conn) {
		clientConn = conn
		fmt.Printf("client get connection again %v\n", clientConn)
		clientOk <- 0
	})
	<-clientOk

	// 关闭服务器，客户端检测到服务器的关闭
	fmt.Printf("\n关闭tcp服务器，客户端检测到服务器的关闭\n")
	clientConn.RegisterDisconnCallback(func(arg interface{}) {
		fmt.Printf("detected server disconnection %s\n", arg.(*Conn))
		detectDisconOk <- 0
	}, clientConn)
	server.Stop()
	<-detectDisconOk
	if !assert.Equal(t, BY_OTHER, clientConn.CloseReason(), "should be closed by other end!") {
		return
	}
	if !assert.Equal(t, BY_SELF, serverConn.CloseReason(), "should be closed by self!") {
		return
	}

	// 消息通信测试, 客户端向服务器发送hello，服务器向客户端返回world
	fmt.Printf("\ntcp消息通信测试, 客户端向服务器发送hello，服务器向客户端返回world\n")
	testTcpCommu(server, client)
	server.Stop()
	client.Stop()
}

func TestUDP(t *testing.T) {
	SetDebugLevel(log.DEBUG)
	SetLogDir("./logs")
	defer log.FlushAll()
	addrs := []string{"127.0.0.1:11000"}
	server := NewCommu(true, "udp", addrs, 1000, 1000)
	if !assert.NotNil(t, server, "create server failed!") {
		return
	}
	client := NewCommu(false, "udp", addrs, 1000, 1000)
	if !assert.NotNil(t, client, "create client failed!") {
		return
	}

	// 启动udp服务器和客户端
	fmt.Printf("\n启动udp服务器和客户端\n")
	var serverConn *Conn
	serverOk := make(chan int, 1)
	server.Start(func(conn *Conn) {
		serverConn = conn
		fmt.Printf("server get connection %v\n", serverConn)
		serverOk <- 0
	})
	var clientConn *Conn
	clientOk := make(chan int, 1)
	client.Start(func(conn *Conn) {
		clientConn = conn
		fmt.Printf("client get connection %v\n", clientConn)
		clientOk <- 0
	})
	<-serverOk
	<-clientOk
	clientIP, clientPort, _ := clientConn.LocalAddr()
	clientOtherIP, clientOtherPort, _ := clientConn.RemoteAddr()
	fmt.Printf("client info: %s:%d, %s:%d\n", clientIP, clientPort, clientOtherIP, clientOtherPort)
	serverIP, serverPort, _ := serverConn.LocalAddr()
	serverOtherIP, serverOtherPort, _ := serverConn.RemoteAddr()
	fmt.Printf("server info: %s:%d, %s:%d\n", serverIP, serverPort, serverOtherIP, serverOtherPort)

	// 消息通信测试, 客户端向服务器发送hello
	fmt.Printf("\nudp消息通信测试, 客户端向服务器发送hello\n")
	testUdpCommu(server, client)
	server.Stop()
	client.Stop()
}
