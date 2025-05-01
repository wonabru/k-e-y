package tcpip

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/okuralabs/okura-node/common"
	"golang.org/x/exp/rand"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	peersConnected      = map[[6]byte][2]byte{}
	validPeersConnected = map[[4]byte]int{}
	nodePeersConnected  = map[[4]byte]int{}
	oldPeers            = map[[6]byte][2]byte{}
	PeersCount          = 0
	waitChan            = make(chan []byte)
	tcpConnections      = make(map[[2]byte]map[[4]byte]*net.TCPConn)
	PeersMutex          = &sync.RWMutex{}
	Quit                chan os.Signal
	TransactionTopic    = [2]byte{'T', 'T'}
	NonceTopic          = [2]byte{'N', 'N'}
	SelfNonceTopic      = [2]byte{'S', 'S'}
	SyncTopic           = [2]byte{'B', 'B'}
	RPCTopic            = [2]byte{'R', 'P'}
)

var Ports = map[[2]byte]int{
	TransactionTopic: 9091,
	NonceTopic:       8091,
	SelfNonceTopic:   7091,
	SyncTopic:        6091,
	RPCTopic:         9009,
}

var MyIP [4]byte

func init() {
	Quit = make(chan os.Signal)
	signal.Notify(Quit, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
	MyIP = GetIp()
	log.Println("Discover MyIP: ", MyIP)
	for k := range Ports {
		tcpConnections[k] = map[[4]byte]*net.TCPConn{}
	}
	// Get NODE_IP environment variable
	ips := os.Getenv("NODE_IP")
	if ips == "" {
		log.Println("Warning: NODE_IP environment variable is not set")
		return
	}

	// Parse the IP address
	ip := net.ParseIP(ips)
	if ip == nil {
		log.Fatalf("Failed to parse NODE_IP '%s' as an IP address", ips)
	}

	ip4 := ip.To4()
	if ip4 == nil {
		log.Fatalf("Failed to parse NODE_IP '%s' as 4 byte format", ips)
	}
	// Assign the parsed IP to tcpip.MyIP
	MyIP = [4]byte(ip4)

	// Rest of your application logic here...
	log.Printf("Successfully set NODE_IP to %d.%d.%d.%d", int(MyIP[0]), int(MyIP[1]), int(MyIP[2]), int(MyIP[3]))
	validPeersConnected[MyIP] = 100
}

func GetIp() [4]byte {
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Println("Can not obtain net interface")
		return [4]byte{}
	}
	ipInternal := [4]byte{}
	zeros := [4]byte{}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			log.Println("Can not get net addresses")
			return [4]byte{}
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			ip4 := ip.To4()
			if ip4 == nil {
				continue
			}
			if ip.IsLoopback() {
				continue
			}
			if !ip.IsPrivate() {
				return [4]byte(ip.To4())
			} else if bytes.Equal(ipInternal[:], zeros[:]) {
				ipInternal = [4]byte(ip.To4())
			}
		}
	}
	return ipInternal
}
func Listen(ip [4]byte, port int) (*net.TCPListener, error) {
	ipport := fmt.Sprintf("%d.%d.%d.%d:%d", ip[0], ip[1], ip[2], ip[3], port)
	protocol := "tcp"
	addr, err := net.ResolveTCPAddr(protocol, ipport)
	if err != nil {
		log.Println("Wrong Address", err)
		return nil, err
	}
	conn, err := net.ListenTCP(protocol, addr)
	if err != nil {
		log.Printf("Some error %v\n", err)
		return nil, err
	}
	return conn, nil
}

func Accept(topic [2]byte, conn *net.TCPListener) (*net.TCPConn, error) {
	tcpConn, err := conn.AcceptTCP()
	if err != nil {
		return nil, fmt.Errorf("error accepting connection: %w", err)
	}

	RegisterPeer(topic, tcpConn)
	return tcpConn, nil
}

func Send(conn *net.TCPConn, message []byte) error {

	message = append(common.MessageInitialization[:], message...)
	message = append(message, []byte("<-END->")...)
	_, err := conn.Write(message)
	if err != nil {
		log.Printf("Can't send response: %v", err)
		return err
	}
	return nil
}

// Receive reads data from the connection and handles errors
func Receive(topic [2]byte, conn *net.TCPConn) []byte {
	const bufSize = 1024 //1048576

	if conn == nil {
		return []byte("<-CLS->")
	}

	buf := make([]byte, bufSize)
	n, err := conn.Read(buf)

	if err != nil {
		//handleConnectionError(err, topic, conn)
		conn.Close()
		return []byte("<-CLS->")
	}

	return buf[:n]
}

// handleConnectionError logs different connection errors and tries to reconnect if necessary
func handleConnectionError(err error, topic [2]byte, conn *net.TCPConn) {
	switch {
	case errors.Is(err, syscall.EPIPE), errors.Is(err, syscall.ECONNRESET), errors.Is(err, syscall.ECONNABORTED):
		log.Print("This is a broken pipe error. Attempting to reconnect...")
	case err == io.EOF:
		log.Print("Connection closed by peer. Attempting to reconnect...")
	default:
		log.Printf("Unexpected error: %v", err)
	}
	// Close the current connection
	conn.Close()
}

// ValidRegisterPeer Confirm that ip is valid node
func ValidRegisterPeer(ip [4]byte) {
	PeersMutex.Lock()
	defer PeersMutex.Unlock()
	if _, ok := validPeersConnected[ip]; ok {
		return
	}
	validPeersConnected[ip] = common.ConnectionMaxTries
}

// NodeRegisterPeer Confirm that ip is valid node IP
func NodeRegisterPeer(ip [4]byte) {
	PeersMutex.Lock()
	defer PeersMutex.Unlock()
	if _, ok := nodePeersConnected[ip]; ok {
		return
	}
	nodePeersConnected[ip] = common.ConnectionMaxTries
}

// ReduceTrustRegisterPeer limit connections attempts needs to be peer lock
func ReduceTrustRegisterPeer(ip [4]byte) {
	if bytes.Equal(ip[:], MyIP[:]) || bytes.Equal(ip[:], []byte{0, 0, 0, 0}) {
		return
	}
	if _, ok := validPeersConnected[ip]; !ok {
		return
	}
	// for testing no reduction
	validPeersConnected[ip]--
	if validPeersConnected[ip] <= 0 {
		delete(validPeersConnected, ip)
	}
}

// RegisterPeer registers a new peer connection
func RegisterPeer(topic [2]byte, tcpConn *net.TCPConn) {

	raddr := tcpConn.RemoteAddr().String()
	ra := strings.Split(raddr, ":")
	ips := strings.Split(ra[0], ".")
	var ip [4]byte
	for i := 0; i < 4; i++ {
		num, err := strconv.Atoi(ips[i])
		if err != nil {
			fmt.Println("Invalid IP address segment:", ips[i])
			return
		}
		ip[i] = byte(num)
	}
	if IsIPBanned(ip) {
		return
	}
	ValidRegisterPeer(ip)
	var topicipBytes [6]byte
	copy(topicipBytes[:], append(topic[:], ip[:]...))

	PeersMutex.Lock()
	defer PeersMutex.Unlock()

	// Check if we already have a connection for this peer
	if existingConn, ok := tcpConnections[topic][ip]; ok {
		// Try to close the existing connection if it's still open
		if existingConn != nil {
			log.Printf("Closing existing connection for peer %v on topic %v", ip, topic)
			existingConn.Close()
		}
		// Remove the old connection from our maps
		delete(tcpConnections[topic], ip)
		delete(peersConnected, topicipBytes)
	}

	log.Printf("Registering new connection from address %s on topic %v", ra[0], topic)

	// Initialize the map for the topic if it doesn't exist
	if _, ok := tcpConnections[topic]; !ok {
		tcpConnections[topic] = make(map[[4]byte]*net.TCPConn)
	}

	// Register the new connection
	tcpConnections[topic][ip] = tcpConn
	peersConnected[topicipBytes] = topic
}

func GetPeersConnected(topic [2]byte) map[[6]byte][2]byte {
	PeersMutex.RLock()
	defer PeersMutex.RUnlock()

	copyOfPeers := make(map[[6]byte][2]byte, len(peersConnected))
	for key, value := range peersConnected {
		if value == topic {
			copyOfPeers[key] = value
		}
	}

	return copyOfPeers
}

func GetIPsConnected() [][]byte {
	PeersMutex.Lock()
	defer PeersMutex.Unlock()
	uniqueIPs := make(map[[4]byte]struct{})
	for key, value := range nodePeersConnected {
		if value > 1 {
			if bytes.Equal(key[:], MyIP[:]) {
				continue
			}
			uniqueIPs[key] = struct{}{}
		}
	}
	var ips [][]byte
	for ip := range uniqueIPs {
		ips = append(ips, ip[:])
	}
	PeersCount = len(ips)
	// return one random peer only
	if PeersCount > 0 {
		rn := rand.Intn(PeersCount)
		return [][]byte{ips[rn]}
	} else {
		return [][]byte{}
	}
}

func GetPeersCount() int {
	PeersMutex.RLock()
	defer PeersMutex.RUnlock()
	return PeersCount
}

func LookUpForNewPeersToConnect(chanPeer chan []byte) {
	for {
		PeersMutex.Lock()
		for topicip, topic := range peersConnected {
			_, ok := oldPeers[topicip]
			if ok == false {
				log.Println("Found new peer with ip", topicip)
				oldPeers[topicip] = topic
				chanPeer <- topicip[:]
			}
		}
		for topicip := range oldPeers {
			_, ok := peersConnected[topicip]
			if ok == false {
				log.Println("New peer is deleted with ip", topicip)
				delete(oldPeers, topicip)
			}
		}
		PeersMutex.Unlock()
		time.Sleep(time.Second * 10)
	}
}
