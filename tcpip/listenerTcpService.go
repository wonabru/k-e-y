package tcpip

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/okuralabs/okura-node/common"
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
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)

			PeersMutex.Lock()
			select {
			case <-ctx.Done():
				// Handle timeout
				PeersMutex.Unlock()
				log.Println("timeout in sending")
				cancel()
				continue
			default:

				if bytes.Equal(ipr[:], []byte{0, 0, 0, 0}) {

					tmpConn := tcpConnections[topic]
					for k, tcpConn0 := range tmpConn {
						if _, ok := validPeersConnected[k]; ok {
							ReduceTrustRegisterPeer(k)
						}
						if _, ok := validPeersConnected[k]; !ok {
							log.Println("when send to all, remove connection")
							CloseAndRemoveConnection(tcpConn0)
						} else if !bytes.Equal(k[:], MyIP[:]) {
							//log.Println("send to ipr", k)
							err := Send(tcpConn0, s[4:])
							if errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNABORTED) {
								log.Println("error in sending to all ", err)
								CloseAndRemoveConnection(tcpConn0)
							}
						}
					}
				} else {
					tcpConns := tcpConnections[topic]
					tcpConn, ok := tcpConns[ipr]

					if _, ok2 := validPeersConnected[ipr]; ok2 {
						ReduceTrustRegisterPeer(ipr)
					}
					if _, ok2 := validPeersConnected[ipr]; !ok2 {
						log.Println("when send to ", ipr, " remove connection")
						CloseAndRemoveConnection(tcpConn)
					} else if ok {
						//log.Println("send to ip", ipr)
						err := Send(tcpConn, s[4:])
						if errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNABORTED) {
							log.Println("error in sending to ", ipr, err)
							CloseAndRemoveConnection(tcpConn)
						}
					} else {
						//fmt.Println("no connection to given ip", ipr, topic)
						//BanIP(ipr, topic)
					}

				}
				PeersMutex.Unlock()
				cancel()
			}
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
			PeersMutex.Lock()
			ReduceTrustRegisterPeer(ip)
			trust, ok := validPeersConnected[ip]
			PeersMutex.Unlock()
			if ok && trust <= 0 {
				BanIP(ip)
			} else if i == maxRetries-1 {
				BanIP(ip)
			}
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
	resetNumber := 0

	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in connection to %v: %v", ip, r)
			receiveChan <- []byte("EXIT")
			PeersMutex.Lock()
			defer PeersMutex.Unlock()
			CloseAndRemoveConnection(tcpConn)
		}
	}()

	log.Printf("Starting message processing loop for connection to %v", ip)

	rTopic := map[[2]byte][]byte{}

	for {
		resetNumber++
		if resetNumber%100 == 0 {
			reconnectionTries = 0
		}

		select {
		case <-Quit:
			log.Printf("Received quit signal for connection to %v", ip)
			receiveChan <- []byte("EXIT")
			PeersMutex.Lock()
			defer PeersMutex.Unlock()
			CloseAndRemoveConnection(tcpConn)
			return
		default:
			r := Receive(topic, tcpConn)
			if r == nil {
				continue
			}
			if bytes.Equal(r, []byte("<-ERR->")) {
				if reconnectionTries > common.ConnectionMaxTries {
					log.Println("error in read. Closing connection", ip, string(r))
					tcpConn.Close()
					tcpConn, err = net.DialTCP("tcp", nil, tcpAddr)
					if err != nil {
						log.Printf("Connection attempt %d to %s failed: %v", ipport, err.Error())
					}
					reconnectionTries = 0
					continue
				}
				reconnectionTries++
				time.Sleep(time.Millisecond * 10)
				continue
			}
			if bytes.Equal(r, []byte("<-CLS->")) || bytes.Equal(r, []byte("QUITFOR")) {

				log.Println("Closing connection", ip, r)
				receiveChan <- []byte("EXIT")
				PeersMutex.Lock()
				defer PeersMutex.Unlock()
				CloseAndRemoveConnection(tcpConn)
				return
			}
			if bytes.Equal(r, []byte("WAIT")) {
				waitChan <- topic[:]
				continue
			}

			rt, ok := rTopic[topic]
			if ok {
				r = append(rt, r...)
			}
			if !bytes.Equal(r[len(r)-7:], []byte("<-END->")) {
				rTopic[topic] = r
			} else {
				rTopic[topic] = []byte{}
			}

			if int32(len(r)) > common.MaxMessageSizeBytes {
				log.Println("error: too long message received: ", len(r))
				PeersMutex.Lock()
				ReduceTrustRegisterPeer(ip)
				PeersMutex.Unlock()
				rTopic[topic] = []byte{}
				if trust, ok := validPeersConnected[ip]; ok && trust <= 0 {
					BanIP(ip)
					receiveChan <- []byte("EXIT")
					return
				}
				continue
			}
			if bytes.Equal(r[len(r)-7:], []byte("<-END->")) {
				if len(r) > 4 {
					if bytes.Equal(r[:4], common.MessageInitialization[:]) {
						receiveChan <- append(ip[:], r[4:]...)
					} else {
						log.Println("wrong MessageInitialization", r[:4], "should be", common.MessageInitialization[:])
						PeersMutex.Lock()
						ReduceTrustRegisterPeer(ip)
						PeersMutex.Unlock()
						if trust, ok := validPeersConnected[ip]; ok && trust <= 0 {
							BanIP(ip)
							receiveChan <- []byte("EXIT")
							return
						}
					}
				}
			}
		}
	}
}

func CloseAndRemoveConnection(tcpConn *net.TCPConn) {
	if tcpConn == nil {
		return
	}
	//PeersMutex.Lock()
	//defer PeersMutex.Unlock()
	topicipBytes := [6]byte{}
	for topic, c := range tcpConnections {
		for k, v := range c {
			if tcpConn.RemoteAddr().String() == v.RemoteAddr().String() {

				fmt.Println("Closing connection (send)", topic, k)
				tcpConnections[topic][k].Close()
				copy(topicipBytes[:], append(topic[:], k[:]...))
				delete(tcpConnections[topic], k)
				delete(peersConnected, topicipBytes)
				delete(validPeersConnected, k)
				delete(nodePeersConnected, k)
			}
		}
	}
}
