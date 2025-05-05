package tcpip

import (
	"bytes"
	"context"
	"github.com/okuralabs/okura-node/common"
	"log"
	"sync"
	"time"
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
	// internal IP should not be banned || bytes.Equal(ip[:2], InternalIP[:2])
	if bytes.Equal(ip[:], MyIP[:]) || bytes.Equal(ip[:], []byte{0, 0, 0, 0}) {
		return
	}
	bannedIPMutex.Lock()
	log.Println("BANNING ", ip)
	bannedIP[ip] = common.GetCurrentTimeStampInSecond() + common.BannedTimeSeconds
	bannedIPMutex.Unlock()
	if PeersMutex.TryLock() {
		defer PeersMutex.Unlock()
		if _, ok := validPeersConnected[ip]; ok {
			delete(validPeersConnected, ip)
		}
		if _, ok := nodePeersConnected[ip]; ok {
			delete(nodePeersConnected, ip)
		}
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
}

func ReduceAndCheckIfBanIP(ip [4]byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	PeersMutex.Lock()
	defer PeersMutex.Unlock()
	select {
	case <-ctx.Done():
		// Handle timeout
		log.Println("ReduceAndCheckIfBanIP: timeout in sending")

	default:
		if _, ok := validPeersConnected[ip]; ok {
			ReduceTrustRegisterPeer(ip)
		}
		if _, ok := validPeersConnected[ip]; !ok {
			log.Println("not trusted ip", ip)
			BanIP(ip)
		}
	}
}
