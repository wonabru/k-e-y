package tcpip

import (
	"bytes"
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
		if hbanned > common.GetCurrentTimeStampInSecond() {
			return true
		}
	}
	return false
}

func BanIP(ip [4]byte) {
	if bytes.Equal(ip[:], MyIP[:]) || bytes.Equal(ip[:3], InternalIP[:3]) {
		return
	}
	bannedIPMutex.Lock()
	log.Println("BANNING ", ip)
	bannedIP[ip] = common.GetCurrentTimeStampInSecond() + common.BannedTimeSeconds
	bannedIPMutex.Unlock()
	PeersMutex.Lock()
	defer PeersMutex.Unlock()
	ReduceTrustRegisterPeer(ip)

	tcpConns := tcpConnections[NonceTopic]
	tcpConn, ok := tcpConns[ip]
	if ok {
		CloseAndRemoveConnection(tcpConn)
		return
	}
	tcpConns = tcpConnections[TransactionTopic]
	tcpConn, ok = tcpConns[ip]
	if ok {
		CloseAndRemoveConnection(tcpConn)
		return
	}
	tcpConns = tcpConnections[SyncTopic]
	tcpConn, ok = tcpConns[ip]
	if ok {
		CloseAndRemoveConnection(tcpConn)
		return
	}
}
