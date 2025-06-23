package tcpip

import (
	"bytes"
	"fmt"
	"github.com/okuralabs/okura-node/common"
	"github.com/okuralabs/okura-node/logger"
	"golang.org/x/exp/rand"
	"io"
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
	TransactionTopic: 19023,
	NonceTopic:       18023,
	SelfNonceTopic:   17023,
	SyncTopic:        16023,
	RPCTopic:         19009,
}

var MyIP [4]byte
var InternalIP [4]byte

func init() {
	Quit = make(chan os.Signal)
	signal.Notify(Quit, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)
	MyIP = GetIp()
	copy(InternalIP[:], MyIP[:])

	logger.GetLogger().Println("Discover MyIP: ", MyIP)
	for k := range Ports {
		tcpConnections[k] = map[[4]byte]*net.TCPConn{}
	}
	// Get NODE_IP environment variable
	ips := os.Getenv("NODE_IP")
	if ips == "" {
		logger.GetLogger().Println("Warning: NODE_IP environment variable is not set")
		return
	}

	// Parse the IP address
	ip := net.ParseIP(ips)
	if ip == nil {
		logger.GetLogger().Fatalf("Failed to parse NODE_IP '%s' as an IP address", ips)
	}

	ip4 := ip.To4()
	if ip4 == nil {
		logger.GetLogger().Fatalf("Failed to parse NODE_IP '%s' as 4 byte format", ips)
	}
	// Assign the parsed IP to tcpip.MyIP
	MyIP = [4]byte(ip4)

	AddWhiteListIPs(MyIP)
	AddWhiteListIPs([4]byte{0, 0, 0, 0})
	// Rest of your application logic here...
	logger.GetLogger().Printf("Successfully set NODE_IP to %d.%d.%d.%d", int(MyIP[0]), int(MyIP[1]), int(MyIP[2]), int(MyIP[3]))
	validPeersConnected[MyIP] = 100

	// Get WHITELIST_IP environment variable
	ips = os.Getenv("WHITELIST_IP")
	if ips == "" {
		logger.GetLogger().Println("Warning: WHITELIST_IP environment variable is not set")
		return
	}

	// Parse the IP address
	ip = net.ParseIP(ips)
	if ip == nil {
		logger.GetLogger().Println("Warning: Failed to parse WHITELIST_IP '%s' as an IP address", ips)
		return
	}

	ip4 = ip.To4()
	if ip4 == nil {
		logger.GetLogger().Println("Warning: failed to parse WHITELIST_IP '%s' as 4 byte format", ips)
		return
	}
	AddWhiteListIPs([4]byte(ip4))
}

func GetIp() [4]byte {
	ifaces, err := net.Interfaces()
	if err != nil {
		logger.GetLogger().Println("Can not obtain net interface")
		return [4]byte{}
	}
	ipInternal := [4]byte{}
	zeros := [4]byte{}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			logger.GetLogger().Println("Can not get net addresses")
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
		logger.GetLogger().Println("Wrong Address", err)
		return nil, err
	}
	conn, err := net.ListenTCP(protocol, addr)
	if err != nil {
		logger.GetLogger().Printf("Some error %v\n", err)
		return nil, err
	}
	return conn, nil
}

func Accept(topic [2]byte, conn *net.TCPListener) (*net.TCPConn, error) {
	tcpConn, err := conn.AcceptTCP()
	if err != nil {
		return nil, fmt.Errorf("error accepting connection: %w", err)
	}

	if !RegisterPeer(topic, tcpConn) {
		tcpConn.Close()
		return nil, fmt.Errorf("error with registration of connection: %w", err)
	}
	tcpConn.SetKeepAlive(true)
	return tcpConn, nil
}

func Send(conn *net.TCPConn, message []byte) error {

	message = append(common.MessageInitialization[:], message...)
	message = append(message, []byte("<-END->")...)

	// Set write deadline to 2 seconds
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))

	_, err := conn.Write(message)
	if err != nil {
		logger.GetLogger().Printf("Can't send response: %v", err)
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
		if err == io.EOF {
			return []byte("<-CLS->")
		}
		logger.GetLogger().Println("n=", n, "err", err.Error())
		//handleConnectionError(err, topic, conn)
		return []byte("<-ERR->")
	}

	return buf[:n]
}

// ValidRegisterPeer Confirm that ip is valid node
func ValidRegisterPeer(ip [4]byte) {
	PeersMutex.Lock()
	defer PeersMutex.Unlock()
	if n, ok := validPeersConnected[ip]; ok {
		if n < 3 {
			validPeersConnected[ip]++
		}
		return
	}
	validPeersConnected[ip] = common.ConnectionMaxTries

}

// NodeRegisterPeer Confirm that ip is valid node IP
func NodeRegisterPeer(ip [4]byte) {
	PeersMutex.Lock()
	defer PeersMutex.Unlock()
	if _, ok := nodePeersConnected[ip]; ok {
		validPeersConnected[ip] = common.ConnectionMaxTries
		return
	}
	nodePeersConnected[ip] = common.ConnectionMaxTries
}

// ReduceTrustRegisterPeer limit connections attempts needs to be peer lock
func ReduceTrustRegisterPeer(ip [4]byte) {
	// || bytes.Equal(ip[:2], InternalIP[:2])
	if bytes.Equal(ip[:], MyIP[:]) || bytes.Equal(ip[:], []byte{0, 0, 0, 0}) {
		return
	}
	if _, ok := validPeersConnected[ip]; !ok {
		return
	}

	validPeersConnected[ip]--
	if validPeersConnected[ip] <= 0 {
		delete(validPeersConnected, ip)
	}
}

// RegisterPeer registers a new peer connection
func RegisterPeer(topic [2]byte, tcpConn *net.TCPConn) bool {

	raddr := tcpConn.RemoteAddr().String()
	ra := strings.Split(raddr, ":")
	ips := strings.Split(ra[0], ".")
	var ip [4]byte
	for i := 0; i < 4; i++ {
		num, err := strconv.Atoi(ips[i])
		if err != nil {
			fmt.Println("Invalid IP address segment:", ips[i])
			return false
		}
		ip[i] = byte(num)
	}
	if IsIPBanned(ip) {
		logger.GetLogger().Println("IP is BANNED", ip)
		return false
	}
	var topicipBytes [6]byte
	copy(topicipBytes[:], append(topic[:], ip[:]...))

	PeersMutex.Lock()
	defer PeersMutex.Unlock()

	// Check if we already have a connection for this peer
	if existingConn, ok := tcpConnections[topic][ip]; ok {
		logger.GetLogger().Println("connection just exists")
		//return false
		// Try to close the existing connection if it's still open
		if existingConn != nil {
			err := existingConn.SetKeepAlivePeriod(1 * time.Second)
			if err != nil {
				logger.GetLogger().Printf("Error setting keep-alive period. Closing for peer %v on topic %v", ip, topic)
				existingConn.Close()
			} else {
				logger.GetLogger().Printf("active existing connection for peer %v on topic %v", ip, topic)
				return false
			}
		}
		// Remove the old connection from our maps
		//delete(tcpConnections[topic], ip)
		//delete(peersConnected, topicipBytes)
	} else {
		validPeersConnected[ip] = common.ConnectionMaxTries
	}

	logger.GetLogger().Printf("Registering new connection from address %s on topic %v", ra[0], topic)

	// Initialize the map for the topic if it doesn't exist
	if _, ok := tcpConnections[topic]; !ok {
		tcpConnections[topic] = make(map[[4]byte]*net.TCPConn)
	}

	// Register the new connection
	tcpConnections[topic][ip] = tcpConn
	peersConnected[topicipBytes] = topic
	return true

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
	if PeersMutex.TryLock() {
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
	return [][]byte{}
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
				logger.GetLogger().Println("Found new peer with ip", topicip)
				oldPeers[topicip] = topic
				chanPeer <- topicip[:]
			}
		}
		for topicip := range oldPeers {
			_, ok := peersConnected[topicip]
			if ok == false {
				logger.GetLogger().Println("New peer is deleted with ip", topicip)
				delete(oldPeers, topicip)
			}
		}
		PeersMutex.Unlock()

		time.Sleep(time.Second * 10)
	}
}
