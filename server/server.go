package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"chittyChatpb/chittyChatpb"

	"google.golang.org/grpc"
)

type chittyChatServer struct {
	chittyChatpb.UnimplementedChittyChatServer
	channel map[string][]chan *chittyChatpb.Message
}

func (s *chittyChatServer) JoinChannel(ch *chittyChatpb.Channel, msgStream chittyChatpb.ChittyChat_JoinChannelServer) error {

	msgChannel := make(chan *chittyChatpb.Message)
	s.channel[ch.Name] = append(s.channel[ch.Name], msgChannel)

	for {
		select {
		case <-msgStream.Context().Done():
			return nil
		case msg := <-msgChannel:
			fmt.Printf("GO ROUTINE (got message): %v \n", msg)
			msgStream.Send(msg)
		}
	}
}

func (s *chittyChatServer) SendMessage(msgStream chittyChatpb.ChittyChat_SendMessageServer) error {
	msg, err := msgStream.Recv()

	if err == io.EOF {
		return nil
	}

	if err != nil {
		return err
	}

	ack := chittyChatpb.MessageAck{Status: "SENT"}
	msgStream.SendAndClose(&ack)

	go func() {
		streams := s.channel[msg.Channel.Name]
		for _, msgChan := range streams {
			msgChan <- msg
		}
	}()

	return nil
}

func newServer() *chittyChatServer {
	s := &chittyChatServer{
		channel: make(map[string][]chan *chittyChatpb.Message),
	}
	return s
}

func main() {
	lis, err := net.Listen("tcp", "localhost:8080")

	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := newServer()
	go func() {
		for {
			fmt.Printf("Connected clients: %v \n", len(s.channel["default"]))
			time.Sleep(5 * time.Second)
		}
	}()

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	chittyChatpb.RegisterChittyChatServer(grpcServer, s)
	grpcServer.Serve(lis)
}
