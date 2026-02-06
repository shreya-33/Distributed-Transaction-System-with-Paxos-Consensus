# Distributed Transaction System with Paxos Consensus

**Go · gRPC · Protocol Buffers · Distributed Systems**

A robust distributed transaction processing system that implements the **Paxos consensus algorithm** to achieve fault-tolerant agreement in a distributed environment. This project demonstrates real-world distributed systems concepts including consensus protocols, fault tolerance, and consistent state management.

---

## 🎯 Project Overview

This project implements a **distributed transaction system** where multiple servers must agree on transaction ordering and execution, even in the presence of **node failures or network partitions**.

The system simulates a banking-style transaction workflow and ensures **strong consistency** across all participating nodes using Paxos. It is designed to reflect how real distributed systems coordinate state safely under failure conditions.

Key distributed systems concepts demonstrated:

- Paxos consensus protocol
- Majority-based quorum decisions
- Fault tolerance and recovery
- Distributed state consistency
- Atomic transaction execution

---

## ⚙️ How Paxos Is Used

The system follows the classical Paxos approach to reach agreement on transactions:

- **Proposers** initiate transaction proposals
- **Acceptors** participate in voting rounds
- **Majority quorum** determines agreement
- Once consensus is reached, the transaction is committed across nodes

The implementation ensures:

- Safety under concurrent proposals
- No split-brain scenarios
- Correct ordering of transactions
- Recovery of failed or lagging nodes via state catch-up

---

## 🚀 Features

### Core Functionality
- Distributed consensus using Paxos
- Fault-tolerant transaction processing
- Atomic commit of transactions
- Consistent state across all nodes
- Automatic recovery and synchronization

### Additional Capabilities
- gRPC-based inter-node communication
- Protocol Buffers for efficient serialization
- Transaction logging and auditing
- Server liveness tracking
- Performance metrics (RPC counts, commits)

---

## 🛠️ Tech Stack

- **Language:** Go
- **Consensus Algorithm:** Paxos
- **Communication:** gRPC
- **Serialization:** Protocol Buffers
- **Concurrency:** Goroutines and mutexes
- **Networking:** TCP-based RPC communication

---

## 📋 Prerequisites

- Go (1.23+ recommended)
- Git
- Terminal / Command Prompt

---

## ▶️ How to Run
## 1. Clone the Repository

git clone https://github.com/shreya-33/Distributed-Transaction-System-with-Paxos-Consensus.git

cd apaxos-main

---

## 2. Install Dependencies

go mod tidy

---

## 3. Start Paxos Servers

Open multiple terminal windows and run:

go run paxos.go S1
go run paxos.go S2
go run paxos.go S3
go run paxos.go S4
go run paxos.go S5

---

## 4. Start the Client

cd client
go run client.go

---

## 📖 Usage

The client provides an interactive interface to:

Execute predefined transaction sets

View balances and logs

Inspect committed transaction history

Observe system behavior under partial failures

Track performance metrics

Failure Scenarios Simulated

All servers live

Partial server failure

Majority quorum only

Network partitions

---

## 🧪 Testing Scenarios

The system supports testing of:

Node crash failures

Network partitions

Concurrent transaction proposals

Recovery and catch-up of failed nodes

Majority quorum enforcement

These scenarios validate correctness, safety, and fault tolerance.

---

## 📂 Code Structure

apaxos-main/
├── paxos.go Core Paxos implementation
├── client/
│ ├── client.go Client application
│ └── transactions.csv Transaction scenarios
├── constants/
│ └── constants.go Server configuration
├── proto/
│ ├── service.proto gRPC service definitions
│ ├── service.pb.go Generated protobuf code
│ └── service_grpc.pb.go Generated gRPC bindings
├── go.mod
└── go.sum

---

## 🎓 Concepts Demonstrated

Distributed consensus (Paxos)

Fault-tolerant system design

Strong consistency guarantees

Quorum-based decision making

Distributed transaction ordering
