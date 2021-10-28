package main

import (
	"bufio"
	"chittyChatpb/chittyChatpb"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"google.golang.org/grpc"
)

var channelName = flag.String("channel", "default", "Channel name")
var senderName = flag.String("sender", "default", "Sender name")
var tcpServer = flag.String("server", ":8080", "TCP server")

func joinChannel(ctx context.Context, client chittyChatpb.ChittyChatClient) {

	channel := chittyChatpb.Channel{Name: *channelName, SendersName: *senderName}
	stream, err := client.JoinChannel(ctx, &channel)
	if err != nil {
		log.Fatalf("client could not join channel: %v", err)
	}

	fmt.Printf("Joined channel: %v \n", *channelName)

	waitc := make(chan struct{})

	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				close(waitc)
				return
			}

			if err != nil {
				log.Fatalf("Failed to recieve message: %v \n", err)
			}

			if *senderName != in.Sender {
				fmt.Printf("MESSAGE: (%v) -> %v \n", in.Sender, in.Message)
			}
		}
	}()

	<-waitc
}

func sendMessage(ctx context.Context, client chittyChatpb.ChittyChatClient, message string) {
	stream, err := client.SendMessage(ctx)
	if err != nil {
		log.Printf("Cant send message: %v", err)
	}

	msg := chittyChatpb.Message{
		Channel: &chittyChatpb.Channel{
			Name:        *channelName,
			SendersName: *senderName,
		},
		Message: message,
		Sender:  *senderName,
	}
	stream.Send(&msg)

	ack, err := stream.CloseAndRecv()
	fmt.Printf("Message sent: %v \n", ack)
}

func main() {

	flag.Parse()

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithBlock(), grpc.WithInsecure())

	conn, err := grpc.Dial(*tcpServer, opts...)
	if err != nil {
		log.Fatalf("Fail to dial(connect): %v", err)
	}

	defer conn.Close()

	ctx := context.Background()
	client := chittyChatpb.NewChittyChatClient(conn)

	go joinChannel(ctx, client)

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		go sendMessage(ctx, client, scanner.Text())
	}
}
