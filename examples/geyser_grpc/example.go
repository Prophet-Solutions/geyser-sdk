package main

import (
	"context"
	"log"

	geyser_grpc "github.com/Prophet-Solutions/geyser-sdk/grpc"
	proto "github.com/Prophet-Solutions/geyser-sdk/pb"
	"github.com/gagliardetto/solana-go"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := geyser_grpc.New(ctx, "http://1.1.8.1:6969/", true, nil)
	if err != nil {
		log.Fatal(err)
	}

	streamCtx, cancelStream := context.WithCancel(context.Background())
	defer cancelStream()

	if err := client.AddStreamClient(streamCtx, "main", proto.CommitmentLevel_FINALIZED.Enum()); err != nil {
		log.Fatal(err)
	}

	streamClient := client.GetStreamClient("main")
	if streamClient == nil {
		log.Fatal("client does not have a stream named main")
	}

	if err := streamClient.SubscribeTransaction("pump.fun", &proto.SubscribeRequestFilterTransactions{
		AccountRequired: []string{"6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P"},
	}); err != nil {
		log.Fatal(err)
	}

	go func() {
		for err := range streamClient.ErrCh {
			log.Println(err.Error())
		}
	}()

	for {
		select {
		case out, ok := <-streamClient.UpdateCh:
			if !ok {
				// UpdateCh is closed, exit the loop
				log.Fatal("update channel is closed")
			}

			go func() {
				filters := out.GetFilters()
				for _, filter := range filters {
					switch filter {
					case "pump.fun":
						signature := solana.SignatureFromBytes(out.GetTransaction().GetTransaction().GetSignature())
						//tx, err := geyser_grpc.ConvertTransaction(out.GetTransaction())
						log.Println(signature.String())
					default:
						log.Printf("unknown filter: %s", filter)
					}
				}
			}()
		}
	}
}
