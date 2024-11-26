package main

import (
	"fmt"
	"log"

	geyser_enhanced_ws "github.com/Prophet-Solutions/geyser-sdk/enhanced_ws"
)

func main() {
	client := geyser_enhanced_ws.NewClient("wss://atlas-mainnet.helius-rpc.com?api-key=<api-key>")

	err := client.Connect()
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	// Either monitor transaction or account

	// Transaction Subscribe
	filter := geyser_enhanced_ws.TransactionSubscribeFilter{
		Vote:            nil,
		Failed:          nil,
		Signature:       nil,
		AccountInclude:  []string{"6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P"}, // up to 50k for Helius
		AccountExclude:  nil,
		AccountRequired: nil,
	}

	showReward := true
	maxSupportedTransactionVersion := uint8(1)
	options := &geyser_enhanced_ws.TransactionSubscribeOptions{
		Commitment:                     "processed",
		Encoding:                       "base64",
		TransactionDetails:             "full",
		ShowRewards:                    &showReward,
		MaxSupportedTransactionVersion: &maxSupportedTransactionVersion,
	}

	txCh, err := client.TransactionSubscribe("tx-sub-1", filter, options)
	if err != nil {
		log.Fatalf("Failed to subscribe to transactions: %v", err)
	}

	// Account Subscribe
	//accountOptions := &geyser_enhanced_ws.AccountSubscribeOptions{
	//	Encoding:   "jsonParsed",
	//	Commitment: "finalized",
	//}

	//acctCh, err := client.AccountSubscribe("acct-sub-1", "6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P", accountOptions)
	//if err != nil {
	//	log.Fatalf("Failed to subscribe to account: %v", err)
	//}

	// Handle errors
	go func() {
		for err := range client.ErrCh {
			log.Printf("Error: %v", err)
		}
	}()

	// Read messages
	go func() {
		for msg := range txCh {
			decodedMsg := msg.(geyser_enhanced_ws.TransactionNotification)
			fmt.Printf("Received transaction message: %v\n", decodedMsg)
		}
	}()

	//go func() {
	//	for msg := range acctCh {
	//		decodedMsg := msg.(geyser_enhanced_ws.AccountNotification)
	//		fmt.Printf("Received account message: %v\n", decodedMsg.Params.Result.Value.Data)
	//	}
	//}()

	// Keep the main function running
	select {}
}
