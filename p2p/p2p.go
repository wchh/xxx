package p2p

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"time"

	ccrypto "xxx/crypto"
	"xxx/log"
	"xxx/types"

	"github.com/libp2p/go-libp2p"
	autonat "github.com/libp2p/go-libp2p-autonat"
	circuit "github.com/libp2p/go-libp2p-circuit"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	protocol "github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p-core/routing"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	// disc "github.com/libp2p/go-libp2p/p2p/discovery"

	"github.com/multiformats/go-multiaddr"
)

var plog = log.New("p2p")

type Node struct {
	*Conf
	host.Host
	tmap map[string]*pubsub.Topic
	C    chan *Msg
}

// const defaultMaxSize = 1024 * 1024

type Msg struct {
	Topic string
	Data  []byte
	PID   string
}

type Conf struct {
	Priv         ccrypto.PrivateKey
	Port         int
	NameService  string
	ForwardPeers bool
	Topics       []string
	BootPeers    []string
}

func NewNode(conf *Conf) (*Node, error) {
	ctx := context.Background()
	pr, err := crypto.UnmarshalEd25519PrivateKey(conf.Priv[:])
	if err != nil {
		return nil, err
	}
	h := newHost(ctx, pr, conf.Port, conf.NameService)
	ps, err := pubsub.NewGossipSub(
		ctx,
		h,
		pubsub.WithPeerOutboundQueueSize(128),
		pubsub.WithMaxMessageSize(pubsub.DefaultMaxMessageSize*10),
		pubsub.WithMessageSigning(false),
		pubsub.WithStrictSignatureVerification(false),
	)
	if err != nil {
		return nil, err
	}

	g := &Node{
		Host: h,
		Conf: conf,
		tmap: make(map[string]*pubsub.Topic),
	}
	g.setHandler()
	topics := conf.Topics
	topics = append(topics, remoteAddrTopic)

	for _, t := range topics {
		t := t
		tp, err := ps.Join(t)
		if err != nil {
			return nil, err
		}
		g.tmap[t] = tp
	}

	go g.run(ps, conf.ForwardPeers)
	return g, nil
}

func (g *Node) bootstrap(addrs ...string) error {
	for _, addr := range addrs {
		targetAddr, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			plog.Error("bootstrap error", "err", err)
			return err
		}

		targetInfo, err := peer.AddrInfoFromP2pAddr(targetAddr)
		if err != nil {
			plog.Error("bootstrap error", "err", err)
			return err
		}

		g.Peerstore().AddAddrs(targetInfo.ID, targetInfo.Addrs, peerstore.AddressTTL)
		err = g.Connect(context.Background(), *targetInfo)
		if err != nil {
			plog.Error("bootstrap error", "err", err)
			continue
		}
		plog.Info("connect boot peer", "bootpeer", targetAddr.String())
		s, err := g.NewStream(context.Background(), targetInfo.ID, protocol.ID(remoteAddrTopic))
		if err != nil {
			plog.Error("bootstrap error", "err", err)
			continue
		}
		s.Write([]byte(g.ID()))
		s.Close()
	}
	return nil
}

func (g *Node) setHandler() {
	g.SetStreamHandler(protocol.ID(remoteAddrTopic), func(s network.Stream) {
		maddr := s.Conn().RemoteMultiaddr()
		pid := s.Conn().RemotePeer()
		plog.Info("remote peer", "peer", pid, "addr", maddr)
		g.Peerstore().AddAddrs(pid, []multiaddr.Multiaddr{maddr}, peerstore.AddressTTL)
	})
}

func (g *Node) handlePeers(data []byte) {
	var ais []peer.AddrInfo
	err := json.Unmarshal(data, &ais)
	if err != nil {
		plog.Error("pid unmarshal error", "err", err)
		return
	}
	for _, ai := range ais {
		if ai.ID != g.ID() {
			plog.Info("add remote peer", "addr", ai.String())
			g.Peerstore().AddAddrs(ai.ID, ai.Addrs, peerstore.AddressTTL)
			err = g.Connect(context.Background(), ai)
			if err != nil {
				plog.Error("connect error", "err", err)
			}
		}
	}
}

func (g *Node) run(ps *pubsub.PubSub, forwardPeers bool) {
	go g.runBootstrap(ps)
	go printPeerstore(g)
	if forwardPeers {
		go g.sendPeersAddr()
	}

	read := func(s *pubsub.Subscription) {
		for {
			m, err := s.Next(context.Background())
			if err != nil {
				panic(err)
			}
			if g.ID() == m.ReceivedFrom {
				continue
			}
			if s.Topic() == remoteAddrTopic {
				go g.handlePeers(m.Data)
			} else {
				g.C <- &Msg{Data: m.Data, Topic: s.Topic()}
			}
		}
	}

	for _, tp := range g.tmap {
		sb, err := tp.Subscribe()
		if err != nil {
			panic(err)
		}
		go read(sb)
	}
}

func (g *Node) runBootstrap(ps *pubsub.PubSub) {
	for range time.NewTicker(time.Second * 60).C {
		np := ps.ListPeers(PeerInfoTopic)
		plog.Info("pos33 peers ", "len", len(np), "peers", np)
		if len(np) < 3 && len(np) < len(g.BootPeers) {
			g.bootstrap(g.BootPeers...)
		}
	}
}

func (g *Node) publish(topic string, data []byte) error {
	t, ok := g.tmap[topic]
	if !ok {
		return errors.New("not support topic")
	}
	return t.Publish(context.Background(), data)
}

func (g *Node) Publish(topic string, msg types.Message) error {
	data, err := types.Marshal(msg)
	if err != nil {
		return err
	}
	return g.publish(topic, data)
}

func Pub2pid(pub []byte) (peer.ID, error) {
	p, err := crypto.UnmarshalEd25519PublicKey(pub)
	if err != nil {
		plog.Error("pub2pid error", "err", err)
		return "", err
	}

	pid, err := peer.IDFromPublicKey(p)
	if err != nil {
		plog.Error("pub2pid2 error", "err", err)
		return "", err
	}
	return pid, nil
}

func newHost(ctx context.Context, priv crypto.PrivKey, port int, ns string) host.Host {
	var idht *dht.IpfsDHT
	h, err := libp2p.New(ctx,
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port), // regular tcp connections
		),
		// libp2p.EnableNATService(),
		// libp2p.DefaultTransports,
		// libp2p.Transport(libp2pquic.NewTransport),
		libp2p.NATPortMap(),
		libp2p.Routing(func(h host.Host) (routing.PeerRouting, error) {
			dht, err := dht.New(ctx, h)
			idht = dht
			return idht, err
		}),
		libp2p.EnableRelay(circuit.OptHop),
		libp2p.EnableRelay(),
	)

	if err != nil {
		panic(err)
	}

	addrInfo := peer.AddrInfo{
		ID:    h.ID(),
		Addrs: h.Addrs(),
	}
	paddr, err := peer.AddrInfoToP2pAddrs(&addrInfo)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile("yccpeeraddr.txt", []byte(paddr[0].String()+"\n"), 0644)
	if err != nil {
		panic(err)
	}
	plog.Info("host inited", "host", paddr)

	discover(ctx, h, idht, ns)

	return h
}

func (g *Node) sendPeersAddr() {
	for range time.NewTicker(time.Second * 60).C {
		peers := g.Peerstore().PeersWithAddrs()
		var ais []*peer.AddrInfo
		for _, id := range peers {
			maddr := g.Peerstore().Addrs(id)
			ai := &peer.AddrInfo{Addrs: maddr, ID: id}
			ais = append(ais, ai)
			plog.Info("peer:", "pid", id.String()[:16], "addr", maddr)
		}
		data, err := json.Marshal(ais)
		if err != nil {
			plog.Error("pid marshal error", "err", err)
			return
		}
		g.publish(remoteAddrTopic, data)
	}
}

func printPeerstore(h host.Host) {
	for range time.NewTicker(time.Second * 60).C {
		peers := h.Peerstore().PeersWithAddrs()
		plog.Info("peersstore len", "len", peers.Len(), "pids", peers)
		for _, id := range peers {
			plog.Info("peer:", "pid", id.String()[:16], "addr", h.Peerstore().Addrs(id))
		}
	}
}

// type mdnsNotifee struct {
// 	h   host.Host
// 	ctx context.Context
// }

// func (m *mdnsNotifee) HandlePeerFound(pi peer.AddrInfo) {
// 	if m.h.Network().Connectedness(pi.ID) != network.Connected {
// 		plog.Info("mdns peer found", "pid", pi.ID.String())
// 		m.h.Connect(m.ctx, pi)
// 	}
// }

func discover(ctx context.Context, h host.Host, idht *dht.IpfsDHT, ns string) {
	_, err := autonat.New(ctx, h)
	if err != nil {
		panic(err)
	}
	// mdns, err := discovery.NewMdnsService(ctx, h, time.Second*10, ns)
	// if err != nil {
	// 	panic(err)
	// }

	// mn := &mdnsNotifee{h: h, ctx: ctx}
	// mdns.RegisterNotifee(mn)

	err = idht.Bootstrap(ctx)
	if err != nil {
		panic(err)
	}
	routingDiscovery := discovery.NewRoutingDiscovery(idht)
	discovery.Advertise(ctx, routingDiscovery, ns)

	peerChan, err := routingDiscovery.FindPeers(ctx, ns)
	if err != nil {
		panic(err)
	}

	go func() {
		host := h
		for peer := range peerChan {
			if peer.ID == host.ID() {
				continue
			}

			stream, err := host.NewStream(ctx, peer.ID, protocol.ID(remoteAddrTopic))
			if err != nil {
				plog.Error("NewStream error:", "err", err)
				return
			}

			time.AfterFunc(time.Second*3, func() { stream.Close() })
			plog.Info("Connected to:", "peer", peer)
		}
	}()
}

// func peerAddr(h host.Host) multiaddr.Multiaddr {
// 	peerInfo := &{
// 		ID:    h.ID(),
// 		Addrs: h.Addrs(),
// 	}
// 	addrs, err := peerstore.InfoToP2pAddrs(peerInfo)
// 	if err != nil {
// 		panic(err)
// 	}
// 	return addrs[0]
// }

//////////////////////////////////

const (
	PreBlockTopic = "preblock"
	NewBlockTopic = "newblock"

	MakerSortTopic      = "makersort"
	CommitteeSortTopic  = "committeesort"
	ConsensusBlockTopic = "consensusblock"

	MakerVoteTopic     = "makervote"
	CommitteeVoteTopic = "committeevote"
	BlockVoteTopic     = "blockvote"

	BlocksTopic    = "blocksreply"
	GetBlocksTopic = "getblocks"

	PeerInfoTopic = "peerinfo"

	remoteAddrTopic = "remoteaddress"
)
