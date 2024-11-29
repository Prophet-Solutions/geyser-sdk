package geyser_grpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
	"time"

	geyser_pb "github.com/Prophet-Solutions/geyser-sdk/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
)

type Client struct {
	grpcConn *grpc.ClientConn
	Ctx      context.Context
	Geyser   geyser_pb.GeyserClient
	ErrCh    chan error
	s        *StreamManager
}

type StreamManager struct {
	clients map[string]*StreamClient
	mu      sync.RWMutex
}

type StreamClient struct {
	Ctx      context.Context
	geyser   geyser_pb.Geyser_SubscribeClient
	request  *geyser_pb.SubscribeRequest
	UpdateCh chan *geyser_pb.SubscribeUpdate
	ErrCh    chan error
	mu       sync.RWMutex
}

// New creates a new Client instance.
func New(ctx context.Context, grpcDialURL string, useGzipCompression bool, md metadata.MD) (*Client, error) {
	ch := make(chan error)
	conn, err := createAndObserveGRPCConn(ctx, ch, gRPCConnOptions{
		Target:             grpcDialURL,
		UseGzipCompression: useGzipCompression,
		ClientParameters: &keepalive.ClientParameters{
			Time:    time.Second * 10,
			Timeout: time.Second * 10,
		},
	})
	if err != nil {
		return nil, err
	}

	geyserClient := geyser_pb.NewGeyserClient(conn)

	if md != nil {
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	return &Client{
		grpcConn: conn,
		Ctx:      ctx,
		Geyser:   geyserClient,
		ErrCh:    ch,
		s: &StreamManager{
			clients: make(map[string]*StreamClient),
			mu:      sync.RWMutex{},
		},
	}, nil
}

// Close closes the client and all the streams.
func (c *Client) Close() error {
	for _, sc := range c.s.clients {
		sc.Stop()
	}
	return c.grpcConn.Close()
}

func (c *Client) Ping(count int32) (*geyser_pb.PongResponse, error) {
	return c.Geyser.Ping(c.Ctx, &geyser_pb.PingRequest{Count: count})
}

// AddStreamClient creates a new Geyser subscribe stream client. You can retrieve it with GetStreamClient.
func (c *Client) AddStreamClient(ctx context.Context, streamName string, commitmentLevel *geyser_pb.CommitmentLevel, opts ...grpc.CallOption) error {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()

	if _, exists := c.s.clients[streamName]; exists {
		return fmt.Errorf("client with name %s already exists", streamName)
	}

	stream, err := c.Geyser.Subscribe(ctx, opts...)
	if err != nil {
		return err
	}

	streamClient := &StreamClient{
		Ctx:    ctx,
		geyser: stream,
		request: &geyser_pb.SubscribeRequest{
			Accounts:           make(map[string]*geyser_pb.SubscribeRequestFilterAccounts),
			Slots:              make(map[string]*geyser_pb.SubscribeRequestFilterSlots),
			Transactions:       make(map[string]*geyser_pb.SubscribeRequestFilterTransactions),
			TransactionsStatus: make(map[string]*geyser_pb.SubscribeRequestFilterTransactions),
			Blocks:             make(map[string]*geyser_pb.SubscribeRequestFilterBlocks),
			BlocksMeta:         make(map[string]*geyser_pb.SubscribeRequestFilterBlocksMeta),
			Entry:              make(map[string]*geyser_pb.SubscribeRequestFilterEntry),
			AccountsDataSlice:  make([]*geyser_pb.SubscribeRequestAccountsDataSlice, 0),
			Commitment:         commitmentLevel,
		},
		UpdateCh: make(chan *geyser_pb.SubscribeUpdate),
		ErrCh:    make(chan error),
		mu:       sync.RWMutex{},
	}

	c.s.clients[streamName] = streamClient
	go streamClient.listen()

	return nil
}

func (s *StreamClient) Stop() {
	s.Ctx.Done()
	close(s.UpdateCh)
	close(s.ErrCh)
}

// GetStreamClient returns a StreamClient for the given streamName from the client's map.
func (c *Client) GetStreamClient(streamName string) *StreamClient {
	defer c.s.mu.RUnlock()
	c.s.mu.RLock()
	return c.s.clients[streamName]
}

// SetRequest sets a custom request to be used across all methods.
func (s *StreamClient) SetRequest(req *geyser_pb.SubscribeRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.request = req
}

// SetCommitmentLevel modifies the commitment level of the stream's request.
func (s *StreamClient) SetCommitmentLevel(commitmentLevel *geyser_pb.CommitmentLevel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.request.Commitment = commitmentLevel
}

// NewRequest creates a new empty *geyser_pb.SubscribeRequest.
func (s *StreamClient) NewRequest() *geyser_pb.SubscribeRequest {
	return &geyser_pb.SubscribeRequest{
		Accounts:           make(map[string]*geyser_pb.SubscribeRequestFilterAccounts),
		Slots:              make(map[string]*geyser_pb.SubscribeRequestFilterSlots),
		Transactions:       make(map[string]*geyser_pb.SubscribeRequestFilterTransactions),
		TransactionsStatus: make(map[string]*geyser_pb.SubscribeRequestFilterTransactions),
		Blocks:             make(map[string]*geyser_pb.SubscribeRequestFilterBlocks),
		BlocksMeta:         make(map[string]*geyser_pb.SubscribeRequestFilterBlocksMeta),
		Entry:              make(map[string]*geyser_pb.SubscribeRequestFilterEntry),
		AccountsDataSlice:  make([]*geyser_pb.SubscribeRequestAccountsDataSlice, 0),
	}
}

// SendCustomRequest sends a custom *geyser_pb.SubscribeRequest using the Geyser client.
func (s *StreamClient) SendCustomRequest(request *geyser_pb.SubscribeRequest) error {
	return s.geyser.Send(request)
}

func (s *StreamClient) sendRequest() error {
	return s.geyser.Send(s.request)
}

// SubscribeAccounts subscribes to account updates.
// Note: This will overwrite existing subscriptions for the given ID.
// To add new accounts without overwriting, use AppendAccounts.
func (s *StreamClient) SubscribeAccounts(filterName string, req *geyser_pb.SubscribeRequestFilterAccounts) error {
	s.mu.Lock()
	s.request.Accounts[filterName] = req
	s.mu.Unlock()
	return s.geyser.Send(s.request)
}

// GetAccounts returns all account addresses for the given filter name.
func (s *StreamClient) GetAccounts(filterName string) []string {
	defer s.mu.RUnlock()
	s.mu.RLock()
	return s.request.Accounts[filterName].Account
}

// AppendAccounts appends accounts to an existing subscription and sends the request.
func (s *StreamClient) AppendAccounts(filterName string, accounts ...string) error {
	s.request.Accounts[filterName].Account = append(s.request.Accounts[filterName].Account, accounts...)
	return s.geyser.Send(s.request)
}

// UnsubscribeAccounts unsubscribes specific accounts.
func (s *StreamClient) UnsubscribeAccounts(filterName string, accounts ...string) error {
	defer s.mu.Unlock()
	s.mu.Lock()
	if filter, exists := s.request.Accounts[filterName]; exists {
		filter.Account = slices.DeleteFunc(filter.Account, func(a string) bool {
			return slices.Contains(accounts, a)
		})
	}
	return s.sendRequest()
}

func (s *StreamClient) UnsubscribeAllAccounts(filterName string) error {
	delete(s.request.Accounts, filterName)
	return s.geyser.Send(s.request)
}

// SubscribeSlots subscribes to slot updates.
func (s *StreamClient) SubscribeSlots(filterName string, req *geyser_pb.SubscribeRequestFilterSlots) error {
	s.request.Slots[filterName] = req
	return s.geyser.Send(s.request)
}

// UnsubscribeSlots unsubscribes from slot updates.
func (s *StreamClient) UnsubscribeSlots(filterName string) error {
	delete(s.request.Slots, filterName)
	return s.geyser.Send(s.request)
}

// SubscribeTransaction subscribes to transaction updates.
func (s *StreamClient) SubscribeTransaction(filterName string, req *geyser_pb.SubscribeRequestFilterTransactions) error {
	s.request.Transactions[filterName] = req
	return s.geyser.Send(s.request)
}

// UnsubscribeTransaction unsubscribes from transaction updates.
func (s *StreamClient) UnsubscribeTransaction(filterName string) error {
	delete(s.request.Transactions, filterName)
	return s.geyser.Send(s.request)
}

// SubscribeTransactionStatus subscribes to transaction status updates.
func (s *StreamClient) SubscribeTransactionStatus(filterName string, req *geyser_pb.SubscribeRequestFilterTransactions) error {
	s.request.TransactionsStatus[filterName] = req
	return s.geyser.Send(s.request)
}

func (s *StreamClient) UnsubscribeTransactionStatus(filterName string) error {
	delete(s.request.TransactionsStatus, filterName)
	return s.geyser.Send(s.request)
}

// SubscribeBlocks subscribes to block updates.
func (s *StreamClient) SubscribeBlocks(filterName string, req *geyser_pb.SubscribeRequestFilterBlocks) error {
	s.request.Blocks[filterName] = req
	return s.geyser.Send(s.request)
}

func (s *StreamClient) UnsubscribeBlocks(filterName string) error {
	delete(s.request.Blocks, filterName)
	return s.geyser.Send(s.request)
}

// SubscribeBlocksMeta subscribes to block metadata updates.
func (s *StreamClient) SubscribeBlocksMeta(filterName string, req *geyser_pb.SubscribeRequestFilterBlocksMeta) error {
	s.request.BlocksMeta[filterName] = req
	return s.geyser.Send(s.request)
}

func (s *StreamClient) UnsubscribeBlocksMeta(filterName string) error {
	delete(s.request.BlocksMeta, filterName)
	return s.geyser.Send(s.request)
}

// SubscribeEntry subscribes to entry updates.
func (s *StreamClient) SubscribeEntry(filterName string, req *geyser_pb.SubscribeRequestFilterEntry) error {
	s.request.Entry[filterName] = req
	return s.geyser.Send(s.request)
}

func (s *StreamClient) UnsubscribeEntry(filterName string) error {
	delete(s.request.Entry, filterName)
	return s.geyser.Send(s.request)
}

// SubscribeAccountDataSlice subscribes to account data slice updates.
func (s *StreamClient) SubscribeAccountDataSlice(req []*geyser_pb.SubscribeRequestAccountsDataSlice) error {
	s.request.AccountsDataSlice = req
	return s.geyser.Send(s.request)
}

func (s *StreamClient) UnsubscribeAccountDataSlice() error {
	s.request.AccountsDataSlice = nil
	return s.geyser.Send(s.request)
}

// listen starts listening for responses and errors.
func (s *StreamClient) listen() {
	defer close(s.UpdateCh)
	defer close(s.ErrCh)

	for {
		select {
		case <-s.Ctx.Done():
			if err := s.Ctx.Err(); err != nil {
				s.ErrCh <- fmt.Errorf("stream context cancelled: %w", err)
			}
			return
		default:
			recv, err := s.geyser.Recv()
			if err != nil {
				if err == io.EOF {
					s.ErrCh <- errors.New("stream cancelled: EOF")
					return
				}
				select {
				case s.ErrCh <- fmt.Errorf("error receiving from stream: %w", err):
				case <-s.Ctx.Done():
					if err = s.Ctx.Err(); err != nil {
						s.ErrCh <- fmt.Errorf("stream context cancelled: %w", err)
					}
					return
				}
				return
			}
			select {
			case s.UpdateCh <- recv:
			case <-s.Ctx.Done():
				return
			}
		}
	}
}
