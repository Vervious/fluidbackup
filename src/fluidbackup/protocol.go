package fluidbackup

import "net"
import "net/rpc"
import "fmt"

type Protocol struct {
	peerList *PeerList
	rpc *rpc.Server
	port int
}

func MakeProtocol(port int) *Protocol {
	this := new(Protocol)
	this.port = port

	this.rpc = rpc.NewServer()
	this.rpc.Register(this)
	l, e := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if e != nil {
		Log.Error.Printf("Error while initializing RPC handler: %s", e.Error())
		return nil
	}
	go func() {
		for {
			conn, _ := l.Accept()
			this.rpc.ServeConn(conn)
		}
	}()

	return this
}

type PingArgs struct {
	Me PeerId
}

type PingReply struct {

}

type ProposeAgreementArgs struct {
	Me PeerId
	MyBytes int
	YourBytes int
}

type ProposeAgreementReply struct {
	Accept bool
}

type StoreShardArgs struct {
	Me PeerId
	Label int64
	Bytes []byte
}

type StoreShardReply struct {
	Confirm bool
}

func (this *Protocol) setPeerList(peerList *PeerList) {
	this.peerList = peerList
}

func (this *Protocol) getMe() PeerId {
	ifaces, err := net.Interfaces()

	if err != nil {
		Log.Warn.Println(err)
	}

	for _, i := range ifaces {
		addrs, err := i.Addrs()

		if err == nil {
			for _, addr := range addrs {
				switch addr.(type) {
				case *net.IPAddr:
					return PeerId{Address: addr.String(), Port: this.port}
				}
			}
		}
	}

	return PeerId{Address: "127.0.0.1", Port: this.port}
}

func (this *Protocol) call(peerId PeerId, fn string, args interface{}, reply interface{}) bool {
	c, errx := rpc.Dial("tcp", peerId.String())
	if errx != nil {
		return false
	}
	defer c.Close()

	err := c.Call("Protocol.Handle" + fn, args, reply)
	if err == nil {
		return true
	}

	Log.Warn.Println(err)
	return false
}

func (this *Protocol) ping(peerId PeerId) bool {
	args := &PingArgs{Me: this.getMe()}
	var reply PingReply
	success := this.call(peerId, "Ping", args, &reply)
	Log.Debug.Printf("Ping %s: %t", peerId.String(), success)
	return success
}

func (this *Protocol) proposeAgreement(peerId PeerId, localBytes int, remoteBytes int) bool {
	args := &ProposeAgreementArgs{
		Me: this.getMe(),
		MyBytes: localBytes,
		YourBytes: remoteBytes,
	}
	var reply ProposeAgreementReply
	success := this.call(peerId, "ProposeAgreement", args, &reply)
	Log.Debug.Printf("ProposeAgreement %s (local: %d; remote: %d): %t", peerId.String(), localBytes, remoteBytes, success && reply.Accept)
	return success && reply.Accept
}

func (this *Protocol) storeShard(peerId PeerId, label int64, bytes []byte) bool {
	args := &StoreShardArgs{
		Me: this.getMe(),
		Label: label,
		Bytes: bytes,
	}
	var reply StoreShardReply
	success := this.call(peerId, "StoreShard", args, &reply)
	return success && reply.Confirm
}

func (this *Protocol) HandlePing(args *PingArgs, reply *PingReply) error {
	return nil
}

func (this *Protocol) HandleProposeAgreement(args *ProposeAgreementArgs, reply *ProposeAgreementReply) error {
	if this.peerList == nil {
		reply.Accept = false
	} else {
		reply.Accept = this.peerList.HandleProposeAgreement(args.Me, args.YourBytes, args.MyBytes)
	}

	return nil
}

func (this *Protocol) HandleStoreShard(args *StoreShardArgs, reply *StoreShardReply) error {
	if this.peerList == nil {
		reply.Confirm = false
	} else {
		reply.Confirm = this.peerList.HandleStoreShard(args.Me, args.Label, args.Bytes)
	}

	return nil
}
