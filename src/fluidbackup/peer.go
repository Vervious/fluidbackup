package fluidbackup

import "sync"
import "fmt"
import "io/ioutil"
import "time"
import "math/rand"

const (
	STATUS_ONLINE = 0
	STATUS_OFFLINE = 1
)

type PeerId struct {
	Address string
	Port int
}

type Peer struct {
	mu sync.Mutex
	protocol *Protocol
	id PeerId
	status int
	localBytes int // how many bytes we've agreed to store for this peer
	remoteBytes int // how many bytes peer is willing to store for us

	// cached values
	localUsedBytes int
	remoteUsedBytes int
}

func MakePeer(id PeerId, protocol *Protocol) *Peer {
	this := new(Peer)
	this.protocol = protocol
	this.id = id
	this.status = STATUS_ONLINE

	go func() {
		this.update()

		if Debug {
			time.Sleep(time.Duration(rand.Intn(3000)) * time.Millisecond + 3 * time.Second)
		} else {
			time.Sleep(time.Duration(rand.Intn(60000)) * time.Millisecond + 30 * time.Second)
		}
	}()

	return this
}

func (this *Peer) proposeAgreement(localBytes int, remoteBytes int) bool {
	if this.protocol.proposeAgreement(this.id, localBytes, remoteBytes) {
		this.eventAgreement(localBytes, remoteBytes)
		return true
	}

	return false
}

func (this *Peer) eventAgreement(localBytes int, remoteBytes int) {
	this.mu.Lock()
	this.localBytes += localBytes
	this.remoteBytes += remoteBytes
	this.mu.Unlock()
}

func (this *Peer) storeShard(shard *BlockShard) bool {
	return this.protocol.storeShard(this.id, int64(shard.Id), shard.Contents)
}

/*
 * Attempts to reserve a number of bytes for storage on this peer.
 * Returns true if the bytes have been reserved for use by caller, or false if reservation failed.
 */
func (this *Peer) reserveBytes(bytes int) bool {
	this.mu.Lock()
	defer this.mu.Unlock()

	if this.remoteBytes - this.remoteUsedBytes >= bytes {
		this.remoteUsedBytes += bytes
		return true
	} else {
		return false
	}
}

func (this *Peer) getShardPath(label int64) string {
	return fmt.Sprintf("store/%s:%d_%d.obj", this.id.Address, this.id.Port, label)
}

func (this *Peer) eventStoreShard(label int64, bytes []byte) bool {
	this.mu.Lock()
	defer this.mu.Unlock()

	// confirm the peer still has space on our storage to reserve
	if this.remoteBytes < this.remoteUsedBytes + len(bytes) {
		return false
	}

	// okay, store it in the file and update cached usage
	err := ioutil.WriteFile(this.getShardPath(label), bytes, 0644)

	if err != nil {
		Log.Warn.Printf("Failed to write peer shard (%s:%d #%d): %s\n", this.id.Address, this.id.Port, label, err.Error())
		return false
	}

	this.remoteUsedBytes += len(bytes)
	return true
}

func (this *Peer) isOnline() bool {
	this.mu.Lock()
	defer this.mu.Unlock()
	return this.status == STATUS_ONLINE
}

func (this *Peer) update() {
	online := this.protocol.ping(this.id)

	this.mu.Lock()
	if online && this.status == STATUS_OFFLINE {
		Log.Info.Printf("Peer %s:%d came online", this.id.Address, this.id.Port)
		this.status = STATUS_ONLINE
	} else if !online && this.status == STATUS_ONLINE {
		Log.Info.Printf("Peer %s:%d went offline", this.id.Address, this.id.Port)
		this.status = STATUS_OFFLINE
	}
	this.mu.Unlock()
}
