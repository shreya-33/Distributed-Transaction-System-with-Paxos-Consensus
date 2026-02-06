package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	pb "rpc/proto"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Transaction struct {
	FromServer string
	ToServer   string
	Amount     int64
}

type Set struct {
	Transactions []Transaction
	LiveServers  []string
}

func getUniqueTransactionID() string {
	return uuid.New().String() // Returns a unique UUID string
}

func readCSV(filename string) ([]Set, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("could not open file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("could not read csv: %v", err)
	}
	var sets []Set
	var currentSet *Set

	// Skip the header line
	if len(records) > 0 {
		records = records[1:] // Remove the first record (header)
	}

	for _, record := range records {
		if len(record) == 0 {
			continue // Skip empty lines
		}

		// Check if the first column has a set number (indicating a new set)
		if record[0] != "" {
			// If we encounter a new set and currentSet is not nil, append it to sets
			if currentSet != nil {
				sets = append(sets, *currentSet)
			}

			// Create a new Set
			currentSet = &Set{
				LiveServers: parseLiveServers(record[2]), // Parse live servers from the third column
			}
		}

		// Parse the transaction if the second column has a transaction
		if len(record) > 1 && record[1] != "" {
			transactionStr := strings.Trim(record[1], "\"")
			parts := strings.Split(strings.Trim(transactionStr, "()"), ",")
			if len(parts) == 3 {
				amount, err := strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
				if err == nil { // Check if parsing was successful
					currentSet.Transactions = append(currentSet.Transactions, Transaction{
						FromServer: strings.TrimSpace(parts[0]),
						ToServer:   strings.TrimSpace(parts[1]),
						Amount:     amount,
					})
				}
			}
		}
	}

	// Append the last set if exists
	if currentSet != nil {
		sets = append(sets, *currentSet)
	}

	return sets, nil
}

func parseLiveServers(liveServersStr string) []string {
	// Remove brackets and split the string into a slice
	liveServersStr = strings.Trim(liveServersStr, "[]")
	return strings.Split(liveServersStr, ",")
}

var mu sync.Mutex

// , wg *sync.WaitGroup
func initiateTransaction(tx Transaction, wg *sync.WaitGroup) {
	defer wg.Done()
	mu.Lock()
	defer mu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Recovered in initiateTransaction: %v\n", r)
		}
	}()
	address := getServerAddress(tx.FromServer)
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Did not connect to server %s: %v", tx.FromServer, err)
	}
	defer conn.Close()

	// Create the gRPC client
	client := pb.NewPaxosServiceClient(conn)

	if err != nil {
		return
	}
	// Prepare the transaction request
	req := &pb.TransactionRequest{
		TransactionId:  getUniqueTransactionID(),
		SenderServer:   tx.FromServer,
		ReceiverServer: tx.ToServer,
		Amount:         tx.Amount, // Define a helper to convert string to float
	}

	// Make the PerformTransaction gRPC call
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := client.PerformTransaction(ctx, req)
	if err != nil {
		log.Fatalf("Could not perform transaction: %v", err)
	}

	// Handle the response
	if res.Success {
		// fmt.Printf("Transaction from %s to %s of amount %d was successful\n", tx.FromServer, tx.ToServer, tx.Amount)
		// fmt.Printf("%s \n", res.Message)
	} else {
		fmt.Printf("Transaction from %s to %s failed: %s\n", tx.FromServer, tx.ToServer, res.Message)
	}
}

func printBalance(server string) int64 {
	address := getServerAddress(server)
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Did not connect to server %s: %v", server, err)
	}
	defer conn.Close()

	// Create the gRPC client
	client := pb.NewPaxosServiceClient(conn)

	// Make the PerformTransaction gRPC call
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	res, err := client.GetBalance(ctx, &emptypb.Empty{})
	if err != nil {
		log.Fatalf("Could not perform transaction: %v", err)
	}
	return res.Balance
}

func getServerAddress(server string) string {
	switch server {
	case "S1":
		return "localhost:5001"
	case "S2":
		return "localhost:5002"
	case "S3":
		return "localhost:5003"
	case "S4":
		return "localhost:5004"
	case "S5":
		return "localhost:5005"
	default:
		return ""
	}
}

// func printAllBalances() {
// 	fmt.Printf("Bal S1:  %d\t", printBalance("S1"))
// 	fmt.Printf("Bal S2:  %d\t", printBalance("S2"))
// 	fmt.Printf("Bal S3:  %d\t", printBalance("S3"))
// 	fmt.Printf("Bal S4:  %d\t", printBalance("S4"))
// 	fmt.Printf("Bal S5:  %d\n", printBalance("S5"))

// }

func updateServerState(server string, state bool) {
	//fmt.Printf("Updating server %s to %t function started \n", server, state)
	address := getServerAddress(server)
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Did not connect to server %s: %v", server, err)
	}
	defer conn.Close()

	// Create the gRPC client
	client := pb.NewPaxosServiceClient(conn)

	// Make the PerformTransaction gRPC call
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	stateUpdateRequest := &pb.StateUpdateRequest{
		State: state,
	}
	client.UpdateServerState(ctx, stateUpdateRequest)
	//fmt.Printf("State update for server %s to %t  \n", server, state)

}

func main() {
	// Load transactions and servers from CSV file
	filename := "transactions.csv" // CSV file with transaction sets
	sets, err := readCSV(filename)
	if err != nil {
		log.Fatalf("Error reading CSV file: %v", err)
	}

	serverSet := []string{"S1", "S2", "S3", "S4", "S5"}

	// Keep track of the current set being processed
	currentSetIndex := 0
	totalSets := len(sets)

	// Loop until all sets have been processed
	for {
		set := sets[currentSetIndex] // Get the current set

		// Infinite loop to keep asking for user input for the current set
		// Display menu options
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("\nSelect an action:")
		fmt.Println("1 - Begin Transaction set")
		fmt.Println("2 - PrintBalance")
		fmt.Println("3 - PrintLog")
		fmt.Println("4 - PrintDB")
		fmt.Println("5 - Performance")
		fmt.Println("6 - Print Client Balance")

		// Read user input
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading input:", err)
			continue
		}

		// Clean up the input
		input = strings.TrimSpace(input)

		// Convert input to integer
		option, err := strconv.Atoi(input)
		if err != nil || (option < 1 || option > 6) {
			fmt.Println("Select option between 1 and 6")
			continue
		}

		switch option {
		case 1:
			// Begin transaction set
			if currentSetIndex > totalSets {
				fmt.Printf("Transaction sets completed, please select other options")
			} else {
				fmt.Printf("\n-- Processing Set %d with live servers: %v --\n", currentSetIndex+1, set.LiveServers)
				var wg sync.WaitGroup
				currentSetIndex++

				for _, server := range serverSet {
					isAlive := false
					for _, liveServer := range set.LiveServers {
						// Trim any potential spaces and compare
						if strings.TrimSpace(server) == strings.TrimSpace(liveServer) {
							isAlive = true
							break
						}
					}
					updateServerState(server, isAlive)
				}

				// Process each transaction in the current set
				for _, tx := range set.Transactions {
					wg.Add(1)
					go initiateTransaction(tx, &wg)
				}

				// Wait for all goroutines to finish
				wg.Wait()
			}

		case 2:
			// Show balances
			fmt.Print("Enter server name: ")
			serveradd, _ := reader.ReadString('\n')
			serveradd = strings.TrimSpace(serveradd)
			address := getServerAddress(serveradd)
			conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("Did not connect to server %s: %v", serveradd, err)
			}
			defer conn.Close()

			// Create the gRPC client
			client := pb.NewPaxosServiceClient(conn)

			// Make the PerformTransaction gRPC call
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			res, err := client.GetBalance(ctx, &emptypb.Empty{})
			if err != nil {
				log.Fatalf("Did not connect to server %s: %v", serveradd, err)
			}
			fmt.Printf("Printing Balance of %s Server = %d \n", serveradd, res.Balance)
			fmt.Printf("\n")
			fmt.Printf("\n")

		case 3:
			fmt.Print("Enter server name: ")
			serveradd, _ := reader.ReadString('\n')
			serveradd = strings.TrimSpace(serveradd)
			address := getServerAddress(serveradd)
			conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("Did not connect to server %s: %v", serveradd, err)
			}
			defer conn.Close()

			// Create the gRPC client
			client := pb.NewPaxosServiceClient(conn)

			// Make the PerformTransaction gRPC call
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			stateUpdateRequest := &pb.GetLocalLogsRequest{
				Server: serveradd,
			}
			res, err := client.GetLocalLogs(ctx, stateUpdateRequest)
			if err != nil {
				log.Fatalf("Did not connect to server %s: %v", serveradd, err)
			}
			if len(res.LocalLogs) > 0 {
				fmt.Printf("Printing local logs of %s Server.... \n", serveradd)
				for _, trns := range res.LocalLogs {
					fmt.Printf("%s ---> %s : %d \n", trns.SenderServer, trns.ReceiverServer, trns.Amount)
				}
			} else {
				fmt.Printf("No local logs found for %s Server \n", serveradd)
			}
			fmt.Printf("\n")
			fmt.Printf("\n")
			fmt.Printf("\n")

		case 4:
			fmt.Print("Enter server name: ")
			serveradd, _ := reader.ReadString('\n')
			serveradd = strings.TrimSpace(serveradd)
			address := getServerAddress(serveradd)
			conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("Did not connect to server %s: %v", serveradd, err)
			}
			defer conn.Close()

			// Create the gRPC client
			client := pb.NewPaxosServiceClient(conn)

			// Make the PerformTransaction gRPC call
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			MBRequest := &pb.GetMainBlocksRequest{
				Server: serveradd,
			}
			res, err := client.GetMainBlocks(ctx, MBRequest)
			if err != nil {
				log.Fatalf("Did not connect to server %s: %v", serveradd, err)
			}
			if len(res.MainBlockData) > 0 {
				fmt.Printf("Printing Main Block of %s Server.... \n", serveradd)
				for _, block := range res.MainBlockData {
					fmt.Printf("Print Block %d of %d \n", block.BallotNumber, len(res.MainBlockData))
					for _, trns := range block.CombinedTransactions {
						fmt.Printf("%s ---> %s : %d \n", trns.SenderServer, trns.ReceiverServer, trns.Amount)
					}
					fmt.Printf("\n")
				}
			} else {
				fmt.Printf("No main block found \n")
			}
			fmt.Printf("\n")
			fmt.Printf("\n")

		case 5:
			fmt.Print("Enter server name: ")
			serveradd, _ := reader.ReadString('\n')
			serveradd = strings.TrimSpace(serveradd)
			address := getServerAddress(serveradd)
			conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("Did not connect to server %s: %v", serveradd, err)
			}
			defer conn.Close()

			// Create the gRPC client
			client := pb.NewPaxosServiceClient(conn)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			res, err := client.GetPerformance(ctx, &emptypb.Empty{})
			if err != nil {
				log.Fatalf("Did not connect to server %s: %v", serveradd, err)
			}
			fmt.Printf("Printing Performance Details of %s Server.... \n", serveradd)
			fmt.Printf("IncomingRPCCount: %d \n", res.IncomingRPCCount)
			fmt.Printf("OutgoingRPCCount: %d \n", res.OutgoingRPCCount)
			fmt.Printf("NumberofCommitedTransactions: %d \n", res.NumberofCommitedTransactions)
			fmt.Printf("\n")
			fmt.Printf("\n")

		case 6:
			fmt.Print("Enter server name: ")
			serveradd, _ := reader.ReadString('\n')
			serveradd = strings.TrimSpace(serveradd)

			fmt.Print("Enter client name: ")
			clientadd, _ := reader.ReadString('\n')
			clientadd = strings.TrimSpace(clientadd)
			address := getServerAddress(serveradd)
			conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("Did not connect to server %s: %v", serveradd, err)
			}
			defer conn.Close()

			// Create the gRPC client
			client := pb.NewPaxosServiceClient(conn)

			// Make the PerformTransaction gRPC call
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			getClientBalanceRequest := &pb.GetClientBalFromServerReq{
				Server: serveradd,
				Client: clientadd,
			}
			res, err := client.GetClientBalanceFromServer(ctx, getClientBalanceRequest)
			if err != nil {
				log.Fatalf("Did not connect to server %s: %v", serveradd, err)
			}
			fmt.Printf("Server %s Balance in Server %s  =  %d \n", clientadd, serveradd, res.Balance)

		}

		// if currentSetIndex >= totalSets {
		// 	break
		// }
	}

	fmt.Println("Exiting. All sets have been processed.")
}
