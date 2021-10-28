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
	"strconv"

	"github.com/thecodeteam/goodbye"
	"google.golang.org/grpc"
)

var channelName = flag.String("channel", "default", "Channel name")
var senderName = flag.String("sender", "default", "Sender name")
var tcpServer = flag.String("server", ":8080", "TCP server")
var LampartTime int32

func joinChannel(ctx context.Context, client chittyChatpb.ChittyChatClient) {

	channel := chittyChatpb.Channel{Name: *channelName, SendersName: *senderName}
	stream, err := client.JoinChannel(ctx, &channel)
	if err != nil {
		log.Fatalf("client could not join channel: %v", err)
	}

	fmt.Printf("Joined channel: %v \n", *channelName)
	sendMessage(ctx, client, "Participant"+*senderName+" joined Chitty-Chat at Lamport time "+strconv.Itoa(int(LampartTime)))

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
			if in.Time >= LampartTime {
				LampartTime = in.Time
			}
			LampartTime++

			fmt.Printf("(%v): %v \n", in.Sender, in.Message)
		}

	}()

	<-waitc
}

func sendMessage(ctx context.Context, client chittyChatpb.ChittyChatClient, message string) {
	stream, err := client.SendMessage(ctx)
	if err != nil {
		log.Printf("Cant send message: %v", err)
	}
	LampartTime++
	msg := chittyChatpb.Message{
		Channel: &chittyChatpb.Channel{
			Name:        *channelName,
			SendersName: *senderName,
		},
		Message: message,
		Time:    LampartTime,
		Sender:  *senderName,
	}
	stream.Send(&msg)

	ack, err := stream.CloseAndRecv()
	ack.GetStatus()
	//fmt.Printf("Message sent: %v \n", ack)
}

func clearCurrentLine() {
	fmt.Print("\n\033[1A\033[K")
}

func main() {

	flag.Parse()

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithBlock(), grpc.WithInsecure())

	conn, err := grpc.Dial(*tcpServer, opts...)
	if err != nil {
		log.Fatalf("Fail to dial(connect): %v", err)
	}

	ctx := context.Background()
	client := chittyChatpb.NewChittyChatClient(conn)

	defer goodbye.Exit(ctx, -1)
	goodbye.Notify(ctx)
	//goodbye.RegisterWithPriority(func(ctx context.Context, sig os.Signal) { fmt.Printf("1: Test1 %[1]s\n", sig) }, 0)
	goodbye.RegisterWithPriority(func(ctx context.Context, sig os.Signal) {
		sendMessage(ctx, client, "Participant"+*senderName+" left Chitty-Chat at Lamport time "+strconv.Itoa(int(LampartTime+1)))
	}, 1)
	//goodbye.RegisterWithPriority(func(ctx context.Context, sig os.Signal) { fmt.Printf("2: Test2 %[1]s\n", sig) }, 2)
	goodbye.RegisterWithPriority(func(ctx context.Context, sig os.Signal) { conn.Close() }, 5)

	go joinChannel(ctx, client)

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		go sendMessage(ctx, client, scanner.Text())
		clearCurrentLine()

	}
}
