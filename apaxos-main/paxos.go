package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"rpc/constants"
	pb "rpc/proto"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type PaxosState int

type Transaction struct {
	From   string
	To     string
	Amount int
}

type PreparePhaseResponse struct {
	Success              bool
	CombinedTransactions []*pb.TransactionRequest
	ActivateCatchUp      bool
}

// type MainBlock struct {
// 	BallotNumber         int32
// 	CombinedTransactions []*pb.TransactionRequest
// }

type Paxos struct {
	mu                           sync.Mutex
	peers                        []string                 // List of peer server addresses
	me                           string                   // Unique identifier for this server
	currentBalance               int64                    // Balance of this server
	executedLogs                 []*pb.TransactionRequest // Array for executed transactions
	unexecutedQueue              []*pb.TransactionRequest // Queue for unexecuted transactions
	mainBlockQueue               []*pb.MainBlock          // Queue for consensus blocks
	acceptValue                  []*pb.TransactionRequest
	acceptNumber                 int32
	currentBallot                int32 // Current ballot number
	lastCommitedBallot           int32
	acceptedBallot               int  // Accepted ballot number
	state                        bool // treu if server is on state or flase
	incomingRPCCount             int64
	outgoingRPCCount             int64
	numberofCommitedTransactions int64
}

// PrepareArgs defines the RPC arguments for the Prepare phase
type PrepareArgs struct {
	ProposalBallot int // Proposal number
}

// PrepareReply defines the RPC reply for the Prepare phase
type PrepareReply struct {
	AcceptedBallot    int           // The highest ballot number this acceptor has accepted
	AcceptedValue     interface{}   // The value associated with the accepted ballot
	ACK               bool          // Whether the prepare was accepted
	LocalTransactions []Transaction // Return current balance
}

// AcceptArgs defines the RPC arguments for the Accept phase
type AcceptArgs struct {
	ProposalBallot int         // Proposal number
	Value          interface{} // Value being proposed
}

// AcceptReply defines the RPC reply for the Accept phase
type AcceptReply struct {
	AcceptOK       bool // Whether the accept was successful
	CurrentBalance int  // Return current balance
}

// CommitArgs defines the RPC arguments for the Commit phase
type CommitArgs struct {
	ProposalBallot int
	Value          interface{}
}

// CommitReply defines the RPC reply for the Commit phase
type CommitReply struct {
	Committed bool
	Balance   int
}

type PaxosServer struct {
	pb.UnimplementedPaxosServiceServer
	px *Paxos
	// Add other fields as necessary
}

func (px *Paxos) activeCatchUp() {
	fmt.Printf("Server enter to catch up function.... \n")
	latestBallet := px.lastCommitedBallot
	catchupBlocks := []*pb.MainBlock{}
	for _, peer := range px.peers {
		if peer == px.me {
			continue
		}
		conn, err := grpc.NewClient(constants.ServerMap[peer], grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			fmt.Errorf("failed to dial peer %s: %v", peer, err)
		}
		defer conn.Close()

		// Create a new Paxos client
		client := pb.NewPaxosServiceClient(conn)

		// Create the PaxosRequest for the Prepare phase
		prepareRequest := &pb.CatchUpRequest{
			LastCommitedBallot: px.lastCommitedBallot,
		}

		// Send the Prepare gRPC request
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		prepareReply, err := client.SendCatchUpData(ctx, prepareRequest)

		if err != nil {
			fmt.Errorf("Prepare request to peer %s failed: %v", peer, err)
			continue
		}

		if prepareReply.CatchUpBallot > latestBallet {
			latestBallet = prepareReply.CatchUpBallot
			catchupBlocks = prepareReply.CatchUpBlocks
		}
	}
	fmt.Printf("Received all commited ballots from one servertill BN: %d as my BN is %d \n", latestBallet, px.lastCommitedBallot)
	px.currentBallot = latestBallet
	px.lastCommitedBallot = latestBallet
	px.mainBlockQueue = append(px.mainBlockQueue, catchupBlocks...)
}

func (px *Paxos) initiatePaxosConsensus(transaction *pb.TransactionRequest) bool {
	// Increment the ballot number
	fmt.Printf("call happened %s\n", px.me)
	fmt.Printf("Server %s initiating paxos prepare phase with ballot number %d \n", px.me, px.lastCommitedBallot+1)
	// Phase 1: Prepare phase
	px.mu.Lock()
	px.currentBallot = px.lastCommitedBallot + 1
	px.mu.Unlock()
	prepareSuccess := px.sendPreparePhase(px.currentBallot)
	if !prepareSuccess.Success {
		log.Printf("Prepare phase of server %s failed\n", px.me)
		if prepareSuccess.ActivateCatchUp {
			px.activeCatchUp()
		}
		time.Sleep(10 * time.Second)
		return false
	}

	fmt.Printf("Server %s completed prepare phase below are the logs\n", px.me)
	fmt.Printf("Combined transactions from one server to  %s : \n", px.me)
	for idx, trans := range prepareSuccess.CombinedTransactions {
		fmt.Printf("After Prepare phase: Received CombTR from idx %d for server %s: Transaction-> Server %s -> Server %s -> Amount: %d \n", idx, px.me, trans.SenderServer, trans.ReceiverServer, trans.Amount)
	}

	// Phase 2: Accept phase
	fmt.Printf("Server %s initiating paxos accept phase \n", px.me)
	acceptSuccess := px.sendAcceptPhase(px.currentBallot, prepareSuccess.CombinedTransactions)
	if !acceptSuccess.ACK {
		log.Println("Accept phase failed of server %s failed \n", px.me)
		return false
	}
	fmt.Printf("Accept phase for server %s completed \n", px.me)

	// Phase 3: Commit phase
	fmt.Printf("Server %s initiating paxos commit phase \n", px.me)
	fmt.Printf("Local and Main Blocks of %s Server before commit phase: \n", px.me)
	fmt.Printf("Local of-> %s \n", px.me)
	for _, trans := range px.executedLogs {
		fmt.Printf("Commit_Ph_Bef of Server %s local: Tans: %s => %s: Amt: %d \n", px.me, trans.SenderServer, trans.ReceiverServer, trans.Amount)
	}
	fmt.Printf("MainBlock of-> %s \n", px.me)
	for _, trans := range px.mainBlockQueue {
		fmt.Printf("Ballot number-> %d \n", trans.BallotNumber)
		for _, mbq := range trans.CombinedTransactions {
			fmt.Printf("Commit_Ph_Bef of Server %s MB: Tans: %s => %s: Amt: %d \n", px.me, mbq.SenderServer, mbq.ReceiverServer, mbq.Amount)
		}

	}
	px.sendCommitPhase(px.currentBallot, prepareSuccess.CombinedTransactions)
	px.reinitiateTransactionAfterConsensus(transaction)

	fmt.Printf("Local and Main Blocks of %s Server after commit phase: \n", px.me)
	fmt.Printf("Local of-> %s \n", px.me)
	for _, trans := range px.executedLogs {
		fmt.Printf("Commit_Ph_Aft of Server %s local: Tans: %s => %s: Amt: %d \n", px.me, trans.SenderServer, trans.ReceiverServer, trans.Amount)
	}
	fmt.Printf("MainBlock of-> %s \n", px.me)
	for _, trans := range px.mainBlockQueue {
		fmt.Printf("Ballot number-> %d \n", trans.BallotNumber)
		for _, mbq := range trans.CombinedTransactions {
			fmt.Printf("Commit_Ph_Aft of Server %s MB: Tans: %s => %s: Amt: %d \n", px.me, mbq.SenderServer, mbq.ReceiverServer, mbq.Amount)
		}

	}
	return true // Transaction committed successfully
}

func (px *Paxos) reinitiateTransactionAfterConsensus(transaction *pb.TransactionRequest) {
	if px.currentBalance >= transaction.Amount {
		// Process the transaction
		px.mu.Lock()
		px.currentBalance -= transaction.Amount
		px.executedLogs = append(px.executedLogs, transaction) // Log the executed transaction
		fmt.Printf("Transaction at %s executed successfully. New balance: %d\n", px.me, px.currentBalance)
		px.mu.Unlock()
	} else {
		px.mu.Lock()
		px.unexecutedQueue = append(px.unexecutedQueue, transaction)
		px.mu.Unlock()
	}
}

// Phase 1: Prepare phase
func (px *Paxos) sendPreparePhase(ballot int32) PreparePhaseResponse {
	// fmt.Printf("Send prepare phase from %s \n", px.me)
	px.mu.Lock()
	px.outgoingRPCCount += 1
	quorum := (len(px.peers) / 2) + 1 // Majority quorum
	prepareSuccessCount := 1
	activateCatchUp := false
	combinedTransactions := []*pb.TransactionRequest{}
	currentUpdatedData := px.acceptedBallot
	px.mu.Unlock()
	for _, peer := range px.peers {
		if peer == px.me {
			px.mu.Lock()
			combinedTransactions = append(combinedTransactions, px.executedLogs...)
			px.mu.Unlock()
			continue
		}
		conn, err := grpc.NewClient(constants.ServerMap[peer], grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			fmt.Errorf("failed to dial peer %s: %v", peer, err)
		}
		defer conn.Close()

		// Create a new Paxos client
		client := pb.NewPaxosServiceClient(conn)

		// Create the PaxosRequest for the Prepare phase
		prepareRequest := &pb.PrepareRequest{
			BallotNumber: px.currentBallot,
		}

		// Send the Prepare gRPC request
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		prepareReply, err := client.Prepare(ctx, prepareRequest)

		if err != nil {
			fmt.Errorf("Prepare request to peer %s failed: %v", peer, err)
			continue
		}

		if prepareReply.ACK {
			fmt.Printf("got prepare reply response to %s with local tramsactions \n", px.me)
			prepareSuccessCount++
			combinedTransactions = append(combinedTransactions, prepareReply.LocalTransactions...)
		}
		if !prepareReply.ACK && prepareReply.BallotNumber >= int32(currentUpdatedData+1) {
			activateCatchUp = true
		}

	}

	// Return true if quorum is reached
	return PreparePhaseResponse{
		Success:              prepareSuccessCount >= quorum,
		CombinedTransactions: combinedTransactions,
		ActivateCatchUp:      activateCatchUp,
	}
}

// Phase 2: Accept phase
func (px *Paxos) sendAcceptPhase(ballot int32, combinedTransactions []*pb.TransactionRequest) pb.AcceptResponse {
	px.mu.Lock()
	px.outgoingRPCCount += 1
	quorum := (len(px.peers) / 2) + 1 // Majority quorum
	acceptSuccessCount := 1
	px.mu.Unlock()

	for _, peer := range px.peers {
		if peer == px.me {
			continue
		}

		conn, err := grpc.NewClient(constants.ServerMap[peer], grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			fmt.Errorf("failed to dial peer %s: %v", peer, err)
		}
		defer conn.Close()

		// Create a new Paxos client
		client := pb.NewPaxosServiceClient(conn)

		// Create the PaxosRequest for the Prepare phase
		acceptRequest := &pb.AcceptRequest{
			BallotNumber:         ballot,
			CombinedTransactions: combinedTransactions,
			SyncMainBlock:        px.mainBlockQueue,
		}
		// Send the Prepare gRPC request
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		acceptReply, err := client.Accept(ctx, acceptRequest)

		if err != nil {
			fmt.Errorf("Prepare request to peer %s failed: %v", peer, err)
			continue
		}
		if acceptReply.ACK {
			acceptSuccessCount++
		}
	}

	// Return true if quorum is reached
	return pb.AcceptResponse{
		ACK: acceptSuccessCount >= quorum,
	}
}

// Phase 3: Commit phase
func (px *Paxos) sendCommitPhase(ballot int32, combinedTransactions []*pb.TransactionRequest) {
	px.outgoingRPCCount += 1
	for _, peer := range px.peers {
		if peer == px.me {
			px.mainBlockQueue = append(px.mainBlockQueue, &pb.MainBlock{
				BallotNumber:         ballot,
				CombinedTransactions: combinedTransactions,
			})

			for _, transaction := range combinedTransactions {
				if transaction.ReceiverServer == px.me {
					px.currentBalance += transaction.Amount
				}
				if transaction.SenderServer == px.me {
					for i, trans := range px.executedLogs {
						if trans.TransactionId == transaction.TransactionId {
							px.executedLogs = append(px.executedLogs[:i], px.executedLogs[i+1:]...)
						}
					}
				}
			}
			px.acceptNumber = -1
			px.acceptValue = []*pb.TransactionRequest{}
			px.currentBallot = ballot
			px.lastCommitedBallot = ballot

		}

		conn, err := grpc.NewClient(constants.ServerMap[peer], grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			fmt.Errorf("failed to dial peer %s: %v", peer, err)
		}
		defer conn.Close()

		// Create a new Paxos client
		client := pb.NewPaxosServiceClient(conn)

		// Create the PaxosRequest for the Prepare phase
		commitRequest := &pb.CommitRequest{
			BallotNumber:         ballot,
			CombinedTransactions: combinedTransactions,
		}

		// Send the Prepare gRPC request
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		client.Commit(ctx, commitRequest)

	}

}

func (s *PaxosServer) PerformTransaction(ctx context.Context, req *pb.TransactionRequest) (*pb.TransactionResponse, error) {
	// s.px.mu.Lock()
	// defer s.px.mu.Unlock()
	fmt.Printf("Queuing transaction %s -> %s of Amount %d \n", req.SenderServer, req.ReceiverServer, req.Amount)
	// Create a new transaction from the request
	newTransaction := &pb.TransactionRequest{
		TransactionId:  req.TransactionId,
		SenderServer:   req.SenderServer,
		ReceiverServer: req.ReceiverServer,
		Amount:         int64(req.Amount),
	}

	// Add the transaction to the unexecuted queue
	s.px.unexecutedQueue = append(s.px.unexecutedQueue, newTransaction)
	fmt.Printf("Transaction from %s to %s for amount %d added to pending queue.\n", newTransaction.SenderServer, newTransaction.ReceiverServer, newTransaction.Amount)

	// For now, return a success response that the transaction is added to the pending queue
	return &pb.TransactionResponse{
		Success: true,
		Message: "Transaction added to pending queue",
	}, nil
}

func (s *PaxosServer) Prepare(ctx context.Context, req *pb.PrepareRequest) (*pb.PrepareResponse, error) {
	if !s.px.state {
		return nil, fmt.Errorf("server is not live")
	}
	s.px.incomingRPCCount += 1
	// fmt.Printf("Got prepare request from a server to serve %s with ballot no %d my ballot number is %d and CB is %d\n", s.px.me, req.BallotNumber, s.px.currentBallot, s.px.currentBalance)
	s.px.mu.Lock()
	defer s.px.mu.Unlock()

	prepareRequest := &pb.PrepareRequest{
		BallotNumber: req.BallotNumber,
	}
	prepareResponse := &pb.PrepareResponse{}
	if s.px.currentBallot < prepareRequest.BallotNumber {
		prepareResponse = &pb.PrepareResponse{
			ACK:               true,
			BallotNumber:      prepareRequest.BallotNumber,
			AcceptNumber:      int32(s.px.acceptedBallot),
			AcceptValue:       s.px.acceptValue,
			LocalTransactions: s.px.executedLogs,
		}
		s.px.currentBallot = prepareRequest.BallotNumber
		fmt.Printf("sending true for prepare phase request from server %s for ballot number request %d as my current ballot is %d \n", s.px.me, prepareRequest.BallotNumber, s.px.currentBallot)
	} else {
		prepareResponse = &pb.PrepareResponse{
			ACK:               false,
			BallotNumber:      s.px.currentBallot,
			AcceptNumber:      int32(s.px.acceptedBallot),
			AcceptValue:       s.px.acceptValue,
			LocalTransactions: s.px.executedLogs,
		}
		fmt.Printf("sending false for prepare phase request from server %s for ballot number request %d as my current ballot is %d \n", s.px.me, prepareRequest.BallotNumber, s.px.currentBallot)
	}

	return prepareResponse, nil
}

func (s *PaxosServer) Accept(ctx context.Context, req *pb.AcceptRequest) (*pb.AcceptResponse, error) {
	if !s.px.state {
		return nil, fmt.Errorf("server is not live")
	}
	s.px.incomingRPCCount += 1
	// fmt.Printf("Accept phase enter request ballnumber: %d, my ballot number %d", req.BallotNumber, s.px.currentBallot)
	s.px.mu.Lock()
	defer s.px.mu.Unlock()
	acceptRequest := &pb.AcceptRequest{
		BallotNumber:         req.BallotNumber,
		CombinedTransactions: req.CombinedTransactions,
		SyncMainBlock:        req.SyncMainBlock,
	}
	if s.px.currentBallot <= acceptRequest.BallotNumber {
		fmt.Printf("Current Bance : %d after accept phase\n", s.px.currentBalance)
		if s.px.acceptNumber != -1 {
			fmt.Printf("Server previous state is accepted but not commmited.... \n ")
			for _, mb := range acceptRequest.SyncMainBlock {
				if mb.BallotNumber >= s.px.acceptNumber {
					for _, trans := range mb.CombinedTransactions {
						if trans.ReceiverServer == s.px.me {
							s.px.mu.Lock()
							s.px.currentBalance += trans.Amount
							s.px.mu.Unlock()
						}
					}
					s.px.mainBlockQueue = append(s.px.mainBlockQueue, mb)
				}
			}
		} else if s.px.currentBallot < acceptRequest.BallotNumber-1 {
			fmt.Printf("Server is not having few blocks.... \n")
			for _, mb := range acceptRequest.SyncMainBlock {
				if mb.BallotNumber > s.px.currentBallot {
					for _, trans := range mb.CombinedTransactions {
						if trans.ReceiverServer == s.px.me {
							s.px.mu.Lock()
							s.px.currentBalance += trans.Amount
							s.px.mu.Unlock()
						}
					}
					s.px.mainBlockQueue = append(s.px.mainBlockQueue, mb)
				}
			}
		}
		s.px.acceptNumber = acceptRequest.BallotNumber
		s.px.acceptValue = acceptRequest.CombinedTransactions
		s.px.lastCommitedBallot = acceptRequest.BallotNumber
	} else {
		fmt.Printf("Current Bance : %d after accept phase\n", s.px.currentBalance)
		return &pb.AcceptResponse{
			ACK: false,
		}, nil
	}
	// if len(req.CombinedTransactions) == 0 {
	// 	s.px.acceptNumber = -1
	// 	s.px.acceptValue = []*pb.TransactionRequest{}
	// 	return &pb.AcceptResponse{
	// 		ACK: false,
	// 	}, nil
	// }
	s.px.acceptNumber = acceptRequest.BallotNumber
	s.px.acceptValue = acceptRequest.CombinedTransactions

	fmt.Printf("Current Bance : %d after accept phase\n", s.px.currentBalance)
	return &pb.AcceptResponse{
		ACK: true,
	}, nil

}

func (s *PaxosServer) Commit(ctx context.Context, req *pb.CommitRequest) (*emptypb.Empty, error) {
	if !s.px.state {
		return &emptypb.Empty{}, nil
	}
	s.px.incomingRPCCount += 1
	fmt.Printf("Got commit request from a server to serve %s with ballot no %d my ballot number is %d and CB is %d\n", s.px.me, req.BallotNumber, s.px.currentBallot, s.px.currentBalance)
	s.px.mu.Lock()
	defer s.px.mu.Unlock()
	commitRequest := &pb.CommitRequest{
		BallotNumber:         req.BallotNumber,
		CombinedTransactions: req.CombinedTransactions,
	}

	fmt.Printf("Current Main Block of the server in commit phase: \n")
	for _, block := range s.px.mainBlockQueue {
		fmt.Print("ballot Number: %d \n", block.BallotNumber)
		if block.BallotNumber == commitRequest.BallotNumber {
			fmt.Println("commitBlock of number: %d already in main block of server %s", block.BallotNumber, s.px.me)
			return &emptypb.Empty{}, nil
		}
	}
	fmt.Printf("Priniting current ballot data done \n")
	fmt.Printf("Combined transactions received from sender:  \n")
	for _, trns := range commitRequest.CombinedTransactions {
		fmt.Printf("Sneder %s -> %s Amt: %d \n", trns.SenderServer, trns.ReceiverServer, trns.Amount)
	}

	s.px.mainBlockQueue = append(s.px.mainBlockQueue, &pb.MainBlock{
		BallotNumber:         commitRequest.BallotNumber,
		CombinedTransactions: commitRequest.CombinedTransactions,
	})

	for _, transaction := range commitRequest.CombinedTransactions {
		fmt.Print("Entered this.....for TR %s and sender %s \n", transaction.ReceiverServer, s.px.me)
		if transaction.ReceiverServer == s.px.me {
			// s.px.mu.Lock()
			fmt.Print("Adding trns: %s -> %s of Amount %d to curBal before \n", transaction.SenderServer, transaction.ReceiverServer, transaction.Amount)
			s.px.currentBalance += transaction.Amount
			fmt.Print("Adding trns: %s -> %s of Amount %d to curBal after: \n", transaction.SenderServer, transaction.ReceiverServer, transaction.Amount)
			// s.px.mu.Unlock()
		}
		if transaction.SenderServer == s.px.me {
			fmt.Print("Removing logs from local....... \n")
			for i, trans := range s.px.executedLogs {
				if trans.TransactionId == transaction.TransactionId {
					// s.px.mu.Lock()
					fmt.Print("Removed log from local %s -> %s Amt: %d \n", transaction.SenderServer, transaction.ReceiverServer, transaction.Amount)
					s.px.executedLogs = append(s.px.executedLogs[:i], s.px.executedLogs[i+1:]...)
					// s.px.mu.Unlock()
				}
			}
		}
	}
	s.px.numberofCommitedTransactions += int64(len(commitRequest.CombinedTransactions))
	s.px.acceptNumber = -1
	s.px.acceptValue = []*pb.TransactionRequest{}
	s.px.currentBallot = req.BallotNumber
	s.px.lastCommitedBallot = req.BallotNumber
	// ***************  handle this for transactions which are added after the prepare phase initiated ***********

	return &emptypb.Empty{}, nil
}

func (s *PaxosServer) GetBalance(ctx context.Context, req *emptypb.Empty) (*pb.BalanceResponse, error) {
	fmt.Printf("Received Balance request, current Bal = %d \n", s.px.currentBalance)
	return &pb.BalanceResponse{
		Balance: s.px.currentBalance,
	}, nil
}

func (s *PaxosServer) UpdateServerState(ctx context.Context, req *pb.StateUpdateRequest) (*emptypb.Empty, error) {
	fmt.Printf("Updating server %s state to %t \n", s.px.me, req.State)
	s.px.state = req.State
	return &emptypb.Empty{}, nil
}

func (s *PaxosServer) SendCatchUpData(ctx context.Context, req *pb.CatchUpRequest) (*pb.CatchUpResponse, error) {
	fmt.Printf("Entered catched up to update mainlock of a server.... \n")
	catchupBlocks := []*pb.MainBlock{}
	if req.LastCommitedBallot < s.px.lastCommitedBallot {
		for _, block := range s.px.mainBlockQueue {
			if block.BallotNumber > req.LastCommitedBallot {
				catchupBlocks = append(catchupBlocks, block)
			}
		}
	}
	fmt.Printf("Sending catchup data with it latest ballot number: %d as the server requested as BN: %d \n", s.px.currentBallot, req.LastCommitedBallot)
	fmt.Printf("below are the block forwarded: \n")
	for _, block := range catchupBlocks {
		fmt.Printf("Block: %d \n", block.BallotNumber)
	}
	fmt.Printf("Blocks display done.... \n")
	return &pb.CatchUpResponse{
		CatchUpBallot: s.px.lastCommitedBallot,
		CatchUpBlocks: catchupBlocks,
	}, nil
}

func (s *PaxosServer) GetLocalLogs(ctx context.Context, req *pb.GetLocalLogsRequest) (*pb.GetLocalLogsResponse, error) {
	return &pb.GetLocalLogsResponse{
		LocalLogs: s.px.executedLogs,
	}, nil
}

func (s *PaxosServer) GetMainBlocks(ctx context.Context, req *pb.GetMainBlocksRequest) (*pb.GetMainBlocksResp, error) {
	return &pb.GetMainBlocksResp{
		MainBlockData: s.px.mainBlockQueue,
	}, nil
}

func (s *PaxosServer) GetClientBalanceFromServer(ctx context.Context, req *pb.GetClientBalFromServerReq) (*pb.GetClientBalFromServerRes, error) {
	s.px.mu.Lock()
	balance := int64(100)
	s.px.mu.Unlock()
	for _, block := range s.px.mainBlockQueue {
		for _, trns := range block.CombinedTransactions {
			if trns.SenderServer == req.Client {
				s.px.mu.Lock()
				balance -= trns.Amount
				s.px.mu.Unlock()
			} else if trns.ReceiverServer == req.Client {
				s.px.mu.Lock()
				balance += trns.Amount
				s.px.mu.Unlock()
			}
		}
	}
	for _, trns := range s.px.executedLogs {
		if trns.SenderServer == req.Client {
			s.px.mu.Lock()
			balance -= trns.Amount
			s.px.mu.Unlock()
		} else if trns.ReceiverServer == req.Client {
			s.px.mu.Lock()
			balance += trns.Amount
			s.px.mu.Unlock()
		}
	}
	return &pb.GetClientBalFromServerRes{
		Balance: balance,
	}, nil
}

func (s *PaxosServer) GetPerformance(ctx context.Context, req *emptypb.Empty) (*pb.PerformanceResponse, error) {
	return &pb.PerformanceResponse{
		IncomingRPCCount:             s.px.incomingRPCCount,
		OutgoingRPCCount:             s.px.outgoingRPCCount,
		NumberofCommitedTransactions: s.px.numberofCommitedTransactions,
	}, nil
}

func (px *Paxos) processPendingTransactions() {
	fmt.Printf("Processing Pending Transactions \n")
	// defer wg.Done()
	for {

		if len(px.unexecutedQueue) > 0 {
			px.mu.Lock()
			transaction := px.unexecutedQueue[0]
			px.unexecutedQueue = px.unexecutedQueue[1:]
			px.mu.Unlock()
			fmt.Printf("Processing transaction from %s to %s for amount %d\n", transaction.SenderServer, transaction.ReceiverServer, transaction.Amount)

			if px.currentBalance >= transaction.Amount {
				px.mu.Lock()
				px.currentBalance -= transaction.Amount
				px.executedLogs = append(px.executedLogs, transaction)
				fmt.Printf("Transaction at %s executed successfully. New balance: %d\n", px.me, px.currentBalance)
				px.mu.Unlock()
			} else if px.state {
				for {
					paxosState := px.initiatePaxosConsensus(transaction)
					if !paxosState {
						fmt.Printf("Consesus initalized again with less ballot number failure %s , Amount: %d \n", px.me, px.currentBallot)
						time.Sleep(200 * time.Millisecond)
					} else {
						break
					}
				}
			} else {
				px.mu.Lock()
				fmt.Println("Reinitiate transaction %s -> %s = Amt: %d to unexecuted block since server is dead ", transaction.SenderServer, transaction.ReceiverServer, transaction.Amount)
				px.reinitiateTransactionAfterConsensus(transaction)
				px.mu.Unlock()
			}
		}

		time.Sleep(5 * time.Second) // Wait for a second before checking the queue again
	}
}

func initializePaxosServer(port string, px *Paxos) bool {
	grpcServer := grpc.NewServer()
	pb.RegisterPaxosServiceServer(grpcServer, &PaxosServer{px: px})
	lis, err := net.Listen("tcp", port)

	if err != nil {
		log.Fatalf("failed to listen: %v", err)
		return false
	}

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()
	return true
}

func Make(peers []string, me string) *Paxos {
	px := &Paxos{
		peers:                        peers,
		me:                           me,
		currentBalance:               100,
		executedLogs:                 []*pb.TransactionRequest{},
		unexecutedQueue:              []*pb.TransactionRequest{},
		mainBlockQueue:               []*pb.MainBlock{},
		acceptValue:                  []*pb.TransactionRequest{},
		acceptNumber:                 -1,
		currentBallot:                0,
		lastCommitedBallot:           0,
		acceptedBallot:               -1,
		state:                        true,
		incomingRPCCount:             0,
		outgoingRPCCount:             0,
		numberofCommitedTransactions: 0,
	}
	// var wg sync.WaitGroup
	// wg.Add(1)
	// go px.processPendingTransactions(&wg)
	// wg.Wait()
	return px
}

func main() {

	if len(os.Args) < 2 {
		fmt.Println("Usage: go run paxos.go <server_name>")
		return
	}

	serverName := os.Args[1]
	address, exists := constants.ServerMap[serverName]

	if !exists {
		fmt.Printf("Server %s not found in the map\n", serverName)
		return
	}

	fmt.Println("Initializing Paxos state...")
	peers := []string{"S1", "S2", "S3", "S4", "S5"}
	px := Make(peers, serverName)
	fmt.Printf("Paxos instance initialized for server %s with bal %d\n", serverName, px.currentBalance)

	fmt.Printf("Starting server %s at %s\n", serverName, address)

	if initializePaxosServer(address, px) {
		px.processPendingTransactions()
	}
	fmt.Printf("server initialized %s\n", serverName)
}
