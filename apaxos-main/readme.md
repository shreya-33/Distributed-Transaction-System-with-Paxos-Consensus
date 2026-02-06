# Distributed Transaction System with Paxos Consensus

## Overview
This project implements a **distributed transaction system** using the **Paxos consensus algorithm** to ensure fault tolerance and consistency across multiple nodes. The system coordinates transactions among distributed participants, guaranteeing agreement even in the presence of node failures or network delays.

The implementation is written in **Go** and uses **gRPC** for inter-node communication, simulating a real-world distributed systems environment.

---

## Architecture
The system follows a **leader-based Paxos architecture** consisting of:

- **Clients**: Initiate transaction requests.
- **Paxos Nodes (Acceptors/Proposers)**: Participate in consensus to agree on transaction ordering and state.
- **gRPC Services**: Enable reliable communication between nodes.
- **Constants & Config Modules**: Centralized configuration for consensus parameters.

Each transaction is proposed, agreed upon using Paxos rounds, and then committed consistently across nodes.

---

## How Paxos Is Used
Paxos is used to achieve **consensus on distributed transactions** by:

1. Proposing a transaction value.
2. Running prepare and accept phases across nodes.
3. Ensuring a majority agreement before committing.
4. Handling failures without compromising consistency.

This guarantees:
- **Safety**: No two nodes commit conflicting transactions.
- **Fault tolerance**: The system continues operating despite node failures.
- **Consistency**: All nodes agree on the same transaction order.

---

## How to Run

### Prerequisites
- Go (1.20+ recommended)
- Protocol Buffers
- gRPC

### Steps
```bash
# Clone the repository
git clone https://github.com/shreya-33/Distributed-Transaction-System-with-Paxos-Consensus.git
cd Distributed-Transaction-System-with-Paxos-Consensus

# Download dependencies
go mod tidy

# Run Paxos server
go run paxos.go

# Run client
go run client/client.go
