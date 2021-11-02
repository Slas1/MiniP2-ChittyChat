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
	"sync"

	"github.com/thecodeteam/goodbye"
	"google.golang.org/grpc"
)

var channelName = flag.String("channel", "default", "Channel name")
var senderName = flag.String("sender", "default", "Sender name")
var tcpServer = flag.String("server", ":8080", "TCP server")
var LamportTime lamportTime

type lamportTime struct {
	time int
	*sync.Mutex
}

func (lt *lamportTime) max(otherValue int) int {
	if lt.time > otherValue {
		return lt.time
	}
	return otherValue
}

func (lt *lamportTime) update(otherValue int) {
	lt.Lock()

	lt.time = lt.max(otherValue) + 1

	lt.Unlock()
}

func (lt *lamportTime) incrementWithOne() {
	lt.Lock()

	lt.time++

	lt.Unlock()
}

func joinChannel(ctx context.Context, client chittyChatpb.ChittyChatClient) {

	channel := chittyChatpb.Channel{Name: *channelName, SendersName: *senderName}
	stream, err := client.JoinChannel(ctx, &channel)
	if err != nil {
		log.Fatalf("client could not join channel: %v", err)
	}

	fmt.Printf("Joined channel: %v \n", *channelName)
	sendMessage(ctx, client, "Participant "+*senderName+" joined Chitty-Chat")

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
			LamportTime.update(int(in.Time))

			fmt.Printf("(%v): %v - Lamport time: "+strconv.Itoa(LamportTime.time)+"\n", in.Sender, in.Message)
		}

	}()

	<-waitc
}

func sendMessage(ctx context.Context, client chittyChatpb.ChittyChatClient, message string) {
	stream, err := client.SendMessage(ctx)
	if err != nil {
		log.Printf("Cant send message: %v", err)
	}
	LamportTime.incrementWithOne()

	msg := chittyChatpb.Message{
		Channel: &chittyChatpb.Channel{
			Name:        *channelName,
			SendersName: *senderName,
		},
		Message: message,
		Time:    int32(LamportTime.time),
		Sender:  *senderName,
	}
	stream.Send(&msg)

	stream.CloseAndRecv()
}

func clearCurrentLine() {
	fmt.Print("\n\033[1A\033[K")
}

func main() {
	LamportTime = lamportTime{0, new(sync.Mutex)}

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
	goodbye.RegisterWithPriority(func(ctx context.Context, sig os.Signal) {
		sendMessage(ctx, client, "Participant "+*senderName+" left Chitty-Chat")
	}, 1)
	goodbye.RegisterWithPriority(func(ctx context.Context, sig os.Signal) { conn.Close() }, 5)

	go joinChannel(ctx, client)

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		if len(scanner.Text()) > 1 && len(scanner.Text()) < 128 {
			go sendMessage(ctx, client, scanner.Text())
		} else {
			fmt.Println("Message has to be between 1 and 128 chars")
			continue
		}
		clearCurrentLine()

	}
}
