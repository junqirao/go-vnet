package main

import (
	"context"
	"io"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/quic-go/quic-go"

	"go-vnet/common/config"
	tt "go-vnet/common/tls"
)

func TestQuic(t *testing.T) {
	ctx := context.Background()
	tls := tt.GenerateTLSConfig(time.Hour*24*7, 1024)
	tls.InsecureSkipVerify = true

	// server - 只负责转发
	listener, err := quic.ListenAddr(":8080", tls, config.DefaultQuicConfig)
	if err != nil {
		panic(err)
	}

	var connA, connB *quic.Conn
	var wg sync.WaitGroup

	// 接收两个连接
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := listener.Accept(ctx)
		if err != nil {
			panic(err)
		}
		connA = conn
		log.Println("connA connected")
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := listener.Accept(ctx)
		if err != nil {
			panic(err)
		}
		connB = conn
		log.Println("connB connected")
	}()

	wg.Wait()
	log.Println("Both connA and connB connected, starting forward...")

	// 转发 A -> B
	go func() {
		for {
			stream, err := connA.AcceptStream(ctx)
			if err != nil {
				panic(err)
				return
			}
			go forwardStream(stream, connB, "a->b")
		}
	}()

	// 转发 B -> A
	go func() {
		for {
			stream, err := connB.AcceptStream(ctx)
			if err != nil {
				panic(err)
				return
			}
			go forwardStream(stream, connA, "b->a")
		}
	}()

	// client A - 发送端
	time.Sleep(time.Second)
	connA_client, err := quic.DialAddr(ctx, "localhost:8080", tls, config.DefaultQuicConfig)
	if err != nil {
		panic(err)
	}
	defer connA_client.CloseWithError(0, "")

	streamA, err := connA_client.OpenStreamSync(ctx)
	if err != nil {
		panic(err)
	}
	defer streamA.Close()

	// client B - 接收端
	connB_client, err := quic.DialAddr(ctx, "localhost:8080", tls, config.DefaultQuicConfig)
	if err != nil {
		panic(err)
	}
	defer connB_client.CloseWithError(0, "")

	// 接收流
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			stream, err := connB_client.AcceptStream(ctx)
			if err != nil {
				panic(err)
				return
			}
			go func(s *quic.Stream) {
				buf := make([]byte, 1400)
				for {
					n, err := s.Read(buf)
					if err != nil {
						if err != io.EOF {
							log.Printf("clientB read error: %v", err)
						}
						return
					}
					log.Printf("clientB received: %s", string(buf[:n]))
				}
			}(stream)
		}
	}()

	// 连续发送多个消息，测试是否会粘包
	messages := []string{
		"Message 1: Hello World",
		"Message 2: Test sticky packet",
		"Message 3: This is a longer message",
		"Message 4: Short",
		"Message 5: Another test message",
	}

	for i, msg := range messages {
		_, err = streamA.Write([]byte(msg))
		if err != nil {
			panic(err)
		}
		log.Printf("clientA sent message %d: %s", i+1, msg)
		time.Sleep(time.Millisecond * 100) // 短暂延迟
	}

	// 等待一段时间观察输出
	time.Sleep(time.Second * 2)
}

func forwardStream(stream *quic.Stream, dest *quic.Conn, name string) {
	defer stream.Close()

	destStream, err := dest.OpenStreamSync(stream.Context())
	if err != nil {
		log.Printf("%s OpenStreamSync error: %v", name, err)
		return
	}
	defer destStream.Close()

	buf := make([]byte, 1400)
	for {
		n, err := stream.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Printf("%s read error: %v", name, err)
			}
			return
		}
		log.Printf("%s forward %d bytes: %s", name, n, string(buf[:n]))
		_, err = destStream.Write(buf[:n])
		if err != nil {
			log.Printf("%s write error: %v", name, err)
			return
		}
	}
}
