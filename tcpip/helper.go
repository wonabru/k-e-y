package tcpip

import (
	"github.com/okuralabs/okura-node/common"
	"log"
	"sync"
)

var bannedIP map[[4]byte]int64
var bannedIPMutex sync.RWMutex

func init() {
	bannedIP = map[[4]byte]int64{}
}
func IsIPBanned(ip [4]byte) bool {
	bannedIPMutex.RLock()
	defer bannedIPMutex.RUnlock()
	if hbanned, ok := bannedIP[ip]; ok {
		if common.GetCurrentTimeStampInSecond() < hbanned {
			return true
		}
	}
	return false
}

func BanIP(ip [4]byte) {
	bannedIPMutex.Lock()
	defer bannedIPMutex.Unlock()
	log.Println("banning ", ip)
	bannedIP[ip] = common.GetCurrentTimeStampInSecond() + common.BannedTimeSeconds
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
