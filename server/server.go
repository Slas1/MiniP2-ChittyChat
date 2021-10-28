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
	channel     map[string][]chan *chittyChatpb.Message
	LampartTime int
}

func (s *chittyChatServer) JoinChannel(ch *chittyChatpb.Channel, msgStream chittyChatpb.ChittyChat_JoinChannelServer) error {

	msgChannel := make(chan *chittyChatpb.Message)
	s.channel[ch.Name] = append(s.channel[ch.Name], msgChannel)

	for {
		select {
		case <-msgStream.Context().Done():
			fmt.Println("client left")

			newChannels := make([]chan *chittyChatpb.Message, 0, 20)
			for _, c := range s.channel[ch.Name] {
				if c != msgChannel {
					newChannels = append(newChannels, c)
				}
			}
			s.channel[ch.Name] = newChannels

			return nil
		case msg := <-msgChannel:
			fmt.Printf("GO ROUTINE (got message): %v \n", msg)
			if int(msg.Time) >= s.LampartTime {
				s.LampartTime = int(msg.Time) + 1
			} else {
				s.LampartTime++
			}
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
	s.LampartTime++
	msg.Time = int32(s.LampartTime)

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
		channel:     make(map[string][]chan *chittyChatpb.Message),
		LampartTime: 0,
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
