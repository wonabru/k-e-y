package tcpip

import (
	"github.com/okuralabs/okura-node/common"
	"log"
	"sync"
)

const bannedTime int64 = 5 //1440 * 7 * 6 // 7 days
var bannedIP map[[4]byte]int64
var bannedIPMutex sync.RWMutex

func init() {
	bannedIP = map[[4]byte]int64{}
}
func IsIPBanned(ip [4]byte, h int64) bool {
	bannedIPMutex.RLock()
	defer bannedIPMutex.RUnlock()
	if hbanned, ok := bannedIP[ip]; ok {
		if h < hbanned {
			return true
		}
	}
	return false
}

func BanIP(ip [4]byte) {
	bannedIPMutex.Lock()
	defer bannedIPMutex.Unlock()
	log.Println("banning ", ip)
	bannedIP[ip] = common.GetHeight() + bannedTime
	ReduceTrustRegisterPeer(ip)
	//PeersMutex.RLock()
	//tcpConns := tcpConnections[topic]
	//tcpConn, ok := tcpConns[ip]
	//PeersMutex.RUnlock()
	//if ok {
	//	PeersMutex.Lock()
	//	defer PeersMutex.Unlock()
	//	CloseAndRemoveConnection(tcpConn)
	//}

}
