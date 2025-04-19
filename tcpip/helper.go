package tcpip

import (
	"github.com/okuralabs/okura-node/common"
	"log"
	"sync"
)

const bannedTime int64 = 1 //1440 * 7 * 6 // 7 days
var bannedIP map[[6]byte]int64
var bannedIPMutex sync.RWMutex

func init() {
	bannedIP = map[[6]byte]int64{}
}
func IsIPBanned(ip [4]byte, h int64, topic [2]byte) bool {
	bannedIPMutex.RLock()
	defer bannedIPMutex.RUnlock()
	topicip := [6]byte{}
	copy(topicip[:], append(topic[:], ip[:]...))
	if hbanned, ok := bannedIP[topicip]; ok {
		if h < hbanned {
			return true
		}
	}
	return false
}

func BanIP(ip [4]byte, topic [2]byte) {
	bannedIPMutex.Lock()
	defer bannedIPMutex.Unlock()
	log.Println("banning ", ip, " with topic ", topic[:])
	topicip := [6]byte{}
	copy(topicip[:], append(topic[:], ip[:]...))
	bannedIP[topicip] = common.GetHeight() + bannedTime

	PeersMutex.RLock()
	tcpConns := tcpConnections[topic]
	tcpConn, ok := tcpConns[ip]
	PeersMutex.RUnlock()
	if ok {
		CloseAndRemoveConnection(tcpConn)
	}

}
