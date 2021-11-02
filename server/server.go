package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"chittyChatpb/chittyChatpb"

	"google.golang.org/grpc"
)

type chittyChatServer struct {
	chittyChatpb.UnimplementedChittyChatServer
	channel     map[string][]chan *chittyChatpb.Message
	LamportTime lamportTime
}

type lamportTime struct {
	time int
	*sync.Mutex
}

func (s *lamportTime) max(lt int, otherValue int) int {
	if lt > otherValue {
		return lt
	}
	return otherValue
}

func (lt *lamportTime) update(otherValue int) {
	lt.Lock()

	lt.time = lt.max(lt.time, otherValue) + 1

	lt.Unlock()
}

func (lt *lamportTime) incrementWithOne() {
	lt.Lock()

	lt.time++

	lt.Unlock()
}

func (s *chittyChatServer) JoinChannel(ch *chittyChatpb.Channel, msgStream chittyChatpb.ChittyChat_JoinChannelServer) error {

	msgChannel := make(chan *chittyChatpb.Message)
	s.channel[ch.Name] = append(s.channel[ch.Name], msgChannel)

	for {
		select {
		case <-msgStream.Context().Done():
			newChannels := make([]chan *chittyChatpb.Message, 0, 20)
			for _, c := range s.channel[ch.Name] {
				if c != msgChannel {
					newChannels = append(newChannels, c)
				}
			}
			s.channel[ch.Name] = newChannels

			return nil
		case msg := <-msgChannel:
			msgStream.Send(msg)
		}
	}
}

func (s *chittyChatServer) SendMessage(msgStream chittyChatpb.ChittyChat_SendMessageServer) error {
	msg, err := msgStream.Recv()
	log.Printf("Server got message: %v \n", msg)
	s.LamportTime.update(int(msg.Time))

	if err == io.EOF {
		return nil
	}

	if err != nil {
		return err
	}

	ack := chittyChatpb.MessageAck{Status: "SENT"}
	msgStream.SendAndClose(&ack)
	s.LamportTime.incrementWithOne()
	msg.Time = int32(s.LamportTime.time)

	go func() {
		streams := s.channel[msg.Channel.Name]
		for _, msgChan := range streams {
			msgChan <- msg
		}
	}()

	log.Printf("Server broadcasted a message: %v \n", msg)

	return nil
}

func newServer() *chittyChatServer {
	s := &chittyChatServer{
		channel:     make(map[string][]chan *chittyChatpb.Message),
		LamportTime: lamportTime{0, new(sync.Mutex)},
	}
	return s
}

func main() {
	log.SetFlags(log.Lmicroseconds)
	lis, err := net.Listen("tcp", "localhost:8080")

	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := newServer()
	go func() {
		for {
			fmt.Printf("Connected clients: %v LamportTime: %v \n", len(s.channel["default"]), s.LamportTime.time)
			time.Sleep(30 * time.Second)
		}
	}()

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	chittyChatpb.RegisterChittyChatServer(grpcServer, s)
	grpcServer.Serve(lis)
}
