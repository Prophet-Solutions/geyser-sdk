package geyser_grpc

import (
	"encoding/binary"
	"fmt"
	"strconv"

	geyser_pb "github.com/Prophet-Solutions/geyser-sdk/pb"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// ConvertTransaction converts a Geyser parsed transaction into an rpc.GetTransactionResult format.
func ConvertTransaction(geyserTx *geyser_pb.SubscribeUpdateTransaction) (*rpc.GetTransactionResult, error) {
	var tx rpc.GetTransactionResult

	meta := geyserTx.Transaction.Meta
	transaction := geyserTx.Transaction.Transaction

	tx.Meta.PreBalances = meta.PreBalances
	tx.Meta.PostBalances = meta.PostBalances
	tx.Meta.Err = meta.Err
	tx.Meta.Fee = meta.Fee
	tx.Meta.ComputeUnitsConsumed = meta.ComputeUnitsConsumed
	tx.Meta.LogMessages = meta.LogMessages

	for _, preTokenBalance := range meta.PreTokenBalances {
		owner := solana.MustPublicKeyFromBase58(preTokenBalance.Owner)
		tx.Meta.PreTokenBalances = append(tx.Meta.PreTokenBalances, rpc.TokenBalance{
			AccountIndex: uint16(preTokenBalance.AccountIndex),
			Owner:        &owner,
			Mint:         solana.MustPublicKeyFromBase58(preTokenBalance.Mint),
			UiTokenAmount: &rpc.UiTokenAmount{
				Amount:         preTokenBalance.UiTokenAmount.Amount,
				Decimals:       uint8(preTokenBalance.UiTokenAmount.Decimals),
				UiAmount:       &preTokenBalance.UiTokenAmount.UiAmount,
				UiAmountString: preTokenBalance.UiTokenAmount.UiAmountString,
			},
		})
	}

	for _, postTokenBalance := range meta.PostTokenBalances {
		owner := solana.MustPublicKeyFromBase58(postTokenBalance.Owner)
		tx.Meta.PostTokenBalances = append(tx.Meta.PostTokenBalances, rpc.TokenBalance{
			AccountIndex: uint16(postTokenBalance.AccountIndex),
			Owner:        &owner,
			Mint:         solana.MustPublicKeyFromBase58(postTokenBalance.Mint),
			UiTokenAmount: &rpc.UiTokenAmount{
				Amount:         postTokenBalance.UiTokenAmount.Amount,
				Decimals:       uint8(postTokenBalance.UiTokenAmount.Decimals),
				UiAmount:       &postTokenBalance.UiTokenAmount.UiAmount,
				UiAmountString: postTokenBalance.UiTokenAmount.UiAmountString,
			},
		})
	}

	for i, innerInst := range meta.InnerInstructions {
		tx.Meta.InnerInstructions[i].Index = uint16(innerInst.Index)
		for x, inst := range innerInst.Instructions {
			accounts, err := bytesToUint16Slice(inst.Accounts)
			if err != nil {
				return nil, err
			}

			tx.Meta.InnerInstructions[i].Instructions[x].Accounts = accounts
			tx.Meta.InnerInstructions[i].Instructions[x].ProgramIDIndex = uint16(inst.ProgramIdIndex)
			if err = tx.Meta.InnerInstructions[i].Instructions[x].Data.UnmarshalJSON(inst.Data); err != nil {
				return nil, err
			}
		}
	}

	for _, reward := range meta.Rewards {
		comm, _ := strconv.ParseUint(reward.Commission, 10, 64)
		commission := uint8(comm)
		tx.Meta.Rewards = append(tx.Meta.Rewards, rpc.BlockReward{
			Pubkey:      solana.MustPublicKeyFromBase58(reward.Pubkey),
			Lamports:    reward.Lamports,
			PostBalance: reward.PostBalance,
			RewardType:  rpc.RewardType(reward.RewardType.String()),
			Commission:  &commission,
		})
	}

	for _, readOnlyAddress := range meta.LoadedReadonlyAddresses {
		tx.Meta.LoadedAddresses.ReadOnly = append(tx.Meta.LoadedAddresses.ReadOnly, solana.PublicKeyFromBytes(readOnlyAddress))
	}

	for _, writableAddress := range meta.LoadedWritableAddresses {
		tx.Meta.LoadedAddresses.ReadOnly = append(tx.Meta.LoadedAddresses.ReadOnly, solana.PublicKeyFromBytes(writableAddress))
	}

	solTx, err := tx.Transaction.GetTransaction()
	if err != nil {
		return nil, err
	}

	if transaction.Message.Versioned {
		solTx.Message.SetVersion(1)
	}

	solTx.Message.RecentBlockhash = solana.HashFromBytes(transaction.Message.RecentBlockhash)
	solTx.Message.Header = solana.MessageHeader{
		NumRequiredSignatures:       uint8(transaction.Message.Header.NumRequiredSignatures),
		NumReadonlySignedAccounts:   uint8(transaction.Message.Header.NumReadonlySignedAccounts),
		NumReadonlyUnsignedAccounts: uint8(transaction.Message.Header.NumReadonlyUnsignedAccounts),
	}

	for _, sig := range transaction.Signatures {
		solTx.Signatures = append(solTx.Signatures, solana.SignatureFromBytes(sig))
	}

	for _, table := range transaction.Message.AddressTableLookups {
		solTx.Message.AddressTableLookups = append(solTx.Message.AddressTableLookups, solana.MessageAddressTableLookup{
			AccountKey:      solana.PublicKeyFromBytes(table.AccountKey),
			WritableIndexes: table.WritableIndexes,
			ReadonlyIndexes: table.ReadonlyIndexes,
		})
	}

	for _, inst := range transaction.Message.Instructions {
		accounts, err := bytesToUint16Slice(inst.Accounts)
		if err != nil {
			return nil, err
		}

		solTx.Message.Instructions = append(solTx.Message.Instructions, solana.CompiledInstruction{
			ProgramIDIndex: uint16(inst.ProgramIdIndex),
			Accounts:       accounts,
			Data:           inst.Data,
		})
	}

	return &tx, nil
}

func BatchConvertTransaction(geyserTxns ...*geyser_pb.SubscribeUpdateTransaction) []*rpc.GetTransactionResult {
	txns := make([]*rpc.GetTransactionResult, len(geyserTxns), 0)
	for _, tx := range geyserTxns {
		txn, err := ConvertTransaction(tx)
		if err != nil {
			continue
		}
		txns = append(txns, txn)
	}
	return txns
}

// ConvertBlockHash converts a Geyser type block to a github.com/gagliardetto/solana-go Solana block.
func ConvertBlockHash(geyserBlock *geyser_pb.SubscribeUpdateBlock) *rpc.GetBlockResult {
	block := new(rpc.GetBlockResult)

	blockTime := solana.UnixTimeSeconds(geyserBlock.BlockTime.Timestamp)
	block.BlockTime = &blockTime
	block.BlockHeight = &geyserBlock.BlockHeight.BlockHeight
	block.Blockhash = solana.Hash{[]byte(geyserBlock.Blockhash)[32]}
	block.ParentSlot = geyserBlock.ParentSlot

	for _, reward := range geyserBlock.Rewards.Rewards {
		commission, err := strconv.ParseUint(reward.Commission, 10, 8)
		if err != nil {
			return nil
		}

		var rewardType rpc.RewardType
		switch reward.RewardType {
		case 1:
			rewardType = rpc.RewardTypeFee
		case 2:
			rewardType = rpc.RewardTypeRent
		case 3:
			rewardType = rpc.RewardTypeStaking
		case 4:
			rewardType = rpc.RewardTypeVoting
		}

		comm := uint8(commission)
		block.Rewards = append(block.Rewards, rpc.BlockReward{
			Pubkey:      solana.MustPublicKeyFromBase58(reward.Pubkey),
			Lamports:    reward.Lamports,
			PostBalance: reward.PostBalance,
			Commission:  &comm,
			RewardType:  rewardType,
		})
	}

	return block
}

func BatchConvertBlockHash(geyserBlocks ...*geyser_pb.SubscribeUpdateBlock) []*rpc.GetBlockResult {
	blocks := make([]*rpc.GetBlockResult, len(geyserBlocks), 0)
	for _, block := range geyserBlocks {
		blocks = append(blocks, ConvertBlockHash(block))
	}
	return blocks
}

func bytesToUint16Slice(data []byte) ([]uint16, error) {
	if len(data)%2 != 0 {
		return nil, fmt.Errorf("length of byte slice must be even to convert to uint16 slice")
	}

	uint16s := make([]uint16, len(data)/2)

	for i := 0; i < len(data); i += 2 {
		uint16s[i/2] = binary.LittleEndian.Uint16(data[i : i+2])
	}

	return uint16s, nil
}
