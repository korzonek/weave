package gossip

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/weaveworks/mesh"

	"github.com/weaveworks/weave/common"
)

// Router to convey gossip from one gossiper to another, for testing
type unicastMessage struct {
	sender mesh.PeerName
	buf    []byte
}
type broadcastMessage struct {
	sender mesh.PeerName
	data   mesh.GossipData
}
type gossipMessage struct {
	sender mesh.PeerName
	data   mesh.GossipData
}
type exitMessage struct {
	exitChan chan struct{}
}
type flushMessage struct {
	flushChan chan struct{}
}

type TestRouter struct {
	gossipChans map[mesh.PeerName]chan interface{}
	loss        float32 // 0.0 means no loss
}

func NewTestRouter(loss float32) *TestRouter {
	return &TestRouter{make(map[mesh.PeerName]chan interface{}, 100), loss}
}

func (grouter *TestRouter) Stop() {
	for peer := range grouter.gossipChans {
		grouter.RemovePeer(peer)
	}
}

func (grouter *TestRouter) gossipBroadcast(sender mesh.PeerName, update mesh.GossipData) {
	for _, gossipChan := range grouter.gossipChans {
		select {
		case gossipChan <- broadcastMessage{sender: sender, data: update}:
		default: // drop the message if we cannot send it
			common.Log.Errorf("Dropping message")
		}
	}
}

func (grouter *TestRouter) gossip(sender mesh.PeerName, update mesh.GossipData) error {
	count := int(math.Log2(float64(len(grouter.gossipChans))))
	for dest, gossipChan := range grouter.gossipChans {
		if dest == sender {
			continue
		}
		select {
		case gossipChan <- gossipMessage{sender: sender, data: update}:
		default: // drop the message if we cannot send it
			common.Log.Errorf("Dropping message")
		}
		count--
		if count <= 0 {
			break
		}
	}
	return nil
}

func (grouter *TestRouter) Flush() {
	for _, gossipChan := range grouter.gossipChans {
		flushChan := make(chan struct{})
		gossipChan <- flushMessage{flushChan: flushChan}
		<-flushChan
	}
}

func (grouter *TestRouter) RemovePeer(peer mesh.PeerName) {
	gossipChan := grouter.gossipChans[peer]
	resultChan := make(chan struct{})
	gossipChan <- exitMessage{exitChan: resultChan}
	<-resultChan
	delete(grouter.gossipChans, peer)
}

type TestRouterClient struct {
	router *TestRouter
	sender mesh.PeerName
}

func (grouter *TestRouter) run(sender mesh.PeerName, gossiper mesh.Gossiper, gossipChan chan interface{}) {
	gossipTimer := time.Tick(2 * time.Second)
	for {
		select {
		case gossip := <-gossipChan:
			switch message := gossip.(type) {
			case exitMessage:
				close(message.exitChan)
				return

			case flushMessage:
				close(message.flushChan)

			case unicastMessage:
				if rand.Float32() > (1.0 - grouter.loss) {
					continue
				}
				if err := gossiper.OnGossipUnicast(message.sender, message.buf); err != nil {
					panic(fmt.Sprintf("Error doing gossip unicast to %s: %s", message.sender, err))
				}

			case broadcastMessage:
				if rand.Float32() > (1.0 - grouter.loss) {
					continue
				}
				for _, msg := range message.data.Encode() {
					if _, err := gossiper.OnGossipBroadcast(message.sender, msg); err != nil {
						panic(fmt.Sprintf("Error doing gossip broadcast: %s", err))
					}
				}
			case gossipMessage:
				if rand.Float32() > (1.0 - grouter.loss) {
					continue
				}
				for _, msg := range message.data.Encode() {
					diff, err := gossiper.OnGossip(msg)
					if err != nil {
						panic(fmt.Sprintf("Error doing gossip: %s", err))
					}
					if diff == nil {
						continue
					}
					// Sanity check - reconsuming the diff should yield nil
					for _, diffMsg := range diff.Encode() {
						if nextDiff, err := gossiper.OnGossip(diffMsg); err != nil {
							panic(fmt.Sprintf("Error doing gossip: %s", err))
						} else if nextDiff != nil {
							panic(fmt.Sprintf("Breach of gossip interface: %v != nil", nextDiff))
						}
					}
					grouter.gossip(message.sender, diff)
				}
			}
		case <-gossipTimer:
			grouter.gossip(sender, gossiper.Gossip())
		}
	}
}

func (grouter *TestRouter) Connect(sender mesh.PeerName, gossiper mesh.Gossiper) mesh.Gossip {
	gossipChan := make(chan interface{}, 100)

	go grouter.run(sender, gossiper, gossipChan)

	grouter.gossipChans[sender] = gossipChan
	return TestRouterClient{grouter, sender}
}

func (client TestRouterClient) GossipUnicast(dstPeerName mesh.PeerName, buf []byte) error {
	select {
	case client.router.gossipChans[dstPeerName] <- unicastMessage{sender: client.sender, buf: buf}:
	default: // drop the message if we cannot send it
		common.Log.Errorf("Dropping message")
	}
	return nil
}

func (client TestRouterClient) GossipBroadcast(update mesh.GossipData) {
	client.router.gossipBroadcast(client.sender, update)
}
