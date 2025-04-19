package tcpip

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net"
	"syscall"
	"time"
)

func StartNewListener(sendChan <-chan []byte, topic [2]byte) {

	conn, err := Listen([4]byte{0, 0, 0, 0}, Ports[topic])
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	defer func() {
		PeersMutex.Lock()
		defer PeersMutex.Unlock()
		for _, tcpConn := range tcpConnections[topic] {
			tcpConn.Close()
		}
	}()
	go LoopSend(sendChan, topic)
	for {
		select {
		case <-Quit:
			return
		default:
			_, err := Accept(topic, conn)
			if err != nil {
				log.Println(err)
				continue
			}
		}
	}
}

//func worker(sendChan <-chan []byte, topic [2]byte, wg *sync.WaitGroup) {
//	defer wg.Done()
//	var ipr [4]byte
//	for s := range sendChan {
//		if len(s) > 4 {
//			copy(ipr[:], s[:4])
//		} else {
//			log.Println("wrong message")
//			continue
//		}
//		PeersMutex.RLock()
//		if bytes.Equal(ipr[:], []byte{0, 0, 0, 0}) {
//			tmpConn := tcpConnections[topic]
//			for k, tcpConn0 := range tmpConn {
//				if !bytes.Equal(k[:], MyIP[:]) {
//					Send(tcpConn0, s[4:])
//				}
//			}
//		} else {
//			tcpConns := tcpConnections[topic]
//			tcpConn, ok := tcpConns[ipr]
//			if ok {
//				Send(tcpConn, s[4:])
//			} else {
//				// Handle no connection case
//			}
//		}
//		PeersMutex.RUnlock()
//	}
//}
//func LoopSend(sendChan <-chan []byte, topic [2]byte, numWorkers int) {
//	var wg sync.WaitGroup
//	// Start worker goroutines
//	for i := 0; i < numWorkers; i++ {
//		wg.Add(1)
//		go worker(sendChan, topic, &wg)
//	}
//	for {
//		select {
//		case b := <-waitChan:
//			if bytes.Equal(b, topic[:]) {
//				time.Sleep(time.Millisecond * 10)
//			}
//		case <-Quit:
//			//close(sendChan)
//			wg.Wait()
//			return
//		default:
//		}
//	}
//}

func LoopSend(sendChan <-chan []byte, topic [2]byte) {
	var ipr [4]byte
	for {
		select {
		case s := <-sendChan:
			if len(s) > 4 {
				copy(ipr[:], s[:4])
			} else {
				log.Println("wrong message")
				continue
			}
			PeersMutex.RLock()
			if bytes.Equal(ipr[:], []byte{0, 0, 0, 0}) {

				tmpConn := tcpConnections[topic]
				for k, tcpConn0 := range tmpConn {
					if !bytes.Equal(k[:], MyIP[:]) {
						//log.Println("send to ipr", k)
						err := Send(tcpConn0, s[4:])
						if errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNABORTED) {
							PeersMutex.RUnlock()
							CloseAndRemoveConnection(tcpConn0)
							PeersMutex.RLock()
						}
					}
				}
			} else {
				tcpConns := tcpConnections[topic]
				tcpConn, ok := tcpConns[ipr]
				if ok {
					//log.Println("send to ip", ipr)
					err := Send(tcpConn, s[4:])
					if errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNABORTED) {
						PeersMutex.RUnlock()
						CloseAndRemoveConnection(tcpConn)
						PeersMutex.RLock()
					}
				} else {
					//fmt.Println("no connection to given ip", ipr, topic)
					//BanIP(ipr, topic)
				}

			}
			PeersMutex.RUnlock()
		case b := <-waitChan:
			if bytes.Equal(b, topic[:]) {
				time.Sleep(time.Millisecond * 10)
			}
		case <-Quit:
			return
		default:
		}
	}
}

func StartNewConnection(ip [4]byte, receiveChan chan []byte, topic [2]byte) {
	ipport := fmt.Sprintf("%d.%d.%d.%d:%d", ip[0], ip[1], ip[2], ip[3], Ports[topic])
	if bytes.Equal(ip[:], []byte{127, 0, 0, 1}) {
		ipport = fmt.Sprintf(":%d", Ports[topic])
	}

	log.Printf("Attempting to connect to %s for topic %v", ipport, topic)

	tcpAddr, err := net.ResolveTCPAddr("tcp", ipport)
	if err != nil {
		log.Printf("Failed to resolve TCP address for %s: %v", ipport, err)
		return
	}

	var tcpConn *net.TCPConn
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		tcpConn, err = net.DialTCP("tcp", nil, tcpAddr)
		if err == nil {
			break
		}
		log.Printf("Connection attempt %d to %s failed: %v", i+1, ipport, err)
		if i < maxRetries-1 {
			time.Sleep(time.Second * 2)
		}
	}

	if err != nil {
		log.Printf("Failed to establish connection to %s after %d attempts: %v", ipport, maxRetries, err)
		return
	}

	if topic == TransactionTopic {
		log.Printf("Successfully connected for TRANSACTIONS TOPIC with %v", ip)
	}

	reconnectionTries := 0
	lastBytes := []byte{}
	resetNumber := 0

	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in connection to %v: %v", ip, r)
			PeersMutex.Lock()
			defer PeersMutex.Unlock()
			receiveChan <- []byte("EXIT")
			CloseAndRemoveConnection(tcpConn)
		}
	}()

	log.Printf("Starting message processing loop for connection to %v", ip)

	for {
		resetNumber++
		if resetNumber%100 == 0 {
			reconnectionTries = 0
		}

		select {
		case <-Quit:
			log.Printf("Received quit signal for connection to %v", ip)
			receiveChan <- []byte("EXIT")
			CloseAndRemoveConnection(tcpConn)
			return
		default:
			r := Receive(topic, tcpConn)
			if r == nil {
				continue
			}

			if bytes.Equal(r, []byte("<-CLS->")) {
				if reconnectionTries > 5 {
					log.Printf("Too many reconnection attempts for %v, closing connection", ip)
					CloseAndRemoveConnection(tcpConn)
					return
				}
				reconnectionTries++
				log.Printf("Connection lost to %v, attempting reconnection (attempt %d)", ip, reconnectionTries)
				tcpConn, err = net.DialTCP("tcp", nil, tcpAddr)
				if err != nil {
					log.Printf("Reconnection attempt %d to %v failed: %v", reconnectionTries, ip, err)
					time.Sleep(time.Second * 2)
					continue
				}
				log.Printf("Successfully reconnected to %v", ip)
				continue
			}

			if bytes.Equal(r, []byte("QUITFOR")) {
				log.Printf("Received QUITFOR signal from %v", ip)
				receiveChan <- []byte("EXIT")
				CloseAndRemoveConnection(tcpConn)
				return
			}

			if bytes.Equal(r, []byte("WAIT")) {
				waitChan <- topic[:]
				continue
			}

			r = append(lastBytes, r...)
			rs := bytes.Split(r, []byte("<-END->"))
			if !bytes.Equal(r[len(r)-7:], []byte("<-END->")) {
				lastBytes = rs[len(rs)-1]
			} else {
				lastBytes = []byte{}
			}

			for _, e := range rs[:len(rs)-1] {
				if len(e) > 0 {
					receiveChan <- append(ip[:], e...)
				}
			}
		}
	}
}

func CloseAndRemoveConnection(tcpConn *net.TCPConn) {
	if tcpConn == nil {
		return
	}
	PeersMutex.Lock()
	defer PeersMutex.Unlock()
	topicipBytes := [6]byte{}
	for topic, c := range tcpConnections {
		for k, v := range c {
			if tcpConn.RemoteAddr().String() == v.RemoteAddr().String() {

				fmt.Println("Closing connection (send)", topic, k)
				tcpConnections[topic][k].Close()
				copy(topicipBytes[:], append(topic[:], k[:]...))
				delete(tcpConnections[topic], k)
				delete(peersConnected, topicipBytes)
			}
		}
	}
}
