# Fabric Chaincode Compatibility PoC

This repository is a small compatibility experiment around Hyperledger Fabric chaincode.

The goal is to show that an existing Fabric Go contract can keep its business logic unchanged even if the execution boundary underneath it changes. Instead of running inside the usual peer-managed chaincode process, the contract is invoked directly through a local runner and a shim-compatible adapter.

The contract used here is the real `asset-transfer-basic` sample from `fabric-samples`.

## What was done in this PoC

This PoC keeps the Fabric sample contract logic and changes the runtime around it.

Specifically:

- the sample contract from `fabric-samples/asset-transfer-basic` was copied into `samplecc/`
- the contract is wrapped as `shim.Chaincode` so it can still be driven through Fabric's chaincode interfaces
- a custom runner in `runner/` replaces the usual peer-to-chaincode execution loop
- a compatibility stub in `shimadapter/` implements the Fabric shim methods the contract expects
- an in-memory backend is used as the state store so the demo can run locally without any Fabric network

In other words, the contract code stays Fabric-style, and the adaptation work happens in the execution and state-access layers.

## What this proves

- Fabric contract logic can remain unchanged.
- The classic peer/shim loop is not the only way to execute the contract.
- Ledger access can be isolated behind a backend client abstraction.

## What this does not prove

- native Fabric-X support for classic chaincode
- endorsement, identity, or signature handling
- private data, history queries, rich queries, or cross-chaincode calls
- production readiness

This is a migration PoC, not a complete runtime.

## How the PoC works, step by step

This PoC is meant to answer one narrow question: can existing Go chaincode keep using the familiar Fabric programming model even if the runtime underneath it is replaced?

In this repository, the answer is demonstrated by keeping the contract logic from `asset-transfer-basic` intact and swapping out the execution environment around it.

### 1. The original Fabric contract is kept as-is

The code in `samplecc/` is the Fabric sample asset-transfer contract. Its business logic still calls normal Fabric APIs such as:

- `GetState`
- `PutState`
- `DelState`
- `GetStateByRange`

Nothing in the contract logic knows about the PoC runtime. From the contract's point of view, it is still talking to a normal Fabric stub.

### 2. The contract is exposed through the standard chaincode interface

`samplecc.NewChaincode()` uses Fabric's Go contract API to build a `shim.Chaincode`.

That matters because the PoC is not calling asset functions directly as plain Go methods. It is still driving the contract through the same `Init`/`Invoke` boundary that Fabric chaincode normally exposes. This keeps the compatibility story realistic: the adapter targets the chaincode interface, not a rewritten contract.

### 3. The runner replaces the peer-managed execution loop

In a normal Fabric network, a client proposal reaches a peer, the peer starts or connects to chaincode, sends the invocation over the shim protocol, and later applies the result to the ledger.

In this PoC, `runner.Runner` stands in for that orchestration layer.

For each invocation, the runner:

1. receives a function name and arguments such as `CreateAsset asset7 yellow 15 Aayush 880`
2. converts them into the byte-argument format expected by Fabric chaincode
3. creates a fresh compatibility stub for that transaction
4. calls `Init` or `Invoke` on the chaincode
5. checks the chaincode response status
6. commits the collected writes if the call is treated as a state-changing operation

So the peer-to-chaincode loop is replaced, but the chaincode entrypoint is still the same.

### 4. The compatibility stub emulates the Fabric shim

`shimadapter.FabricXStub` is the core adapter in this PoC.

Its job is to satisfy the methods the contract expects from `shim.ChaincodeStubInterface`. It provides:

- argument handling such as `GetFunctionAndParameters`
- transaction metadata such as `GetTxID`
- state APIs such as `GetState`, `PutState`, `DelState`, and `GetStateByRange`
- a small in-memory transaction context using read and write sets

This is the key idea behind the migration experiment: existing chaincode does not need to know how state is implemented underneath, as long as the stub methods it calls continue to behave the way it expects.

### 5. Reads and writes are buffered before commit

The stub does not immediately write to the backend when chaincode calls `PutState` or `DelState`.

Instead, it stages those changes in an in-memory write set for the current transaction. During execution:

- `GetState` first checks whether the key was already updated in the current transaction
- if not, it reads from the backend client
- `PutState` records a pending value
- `DelState` records a pending delete
- range queries merge committed backend data with staged in-transaction updates

This makes the flow closer to a real ledger execution model, where contract logic runs against a transaction-scoped view and changes are only applied after successful completion.

### 6. The backend is abstracted behind a client interface

The stub does not talk directly to a Fabric peer or ledger. It talks to a `FabricXStateClient` interface instead.

That interface currently supports a minimal state surface:

- `GetState`
- `GetMultipleStates`
- `PutState`
- `DelState`
- `GetStateByRange`

The concrete implementation in this PoC is `InMemoryStateClient`, which stores state in a Go map grouped by namespace.

This is an important design point for the proposal: once the chaincode-facing side is expressed as a shim-compatible stub and the state-facing side is expressed as a backend client, the two concerns become separable. The in-memory store is only a stand-in for a future service-backed implementation.

### 7. The demo drives a realistic asset flow end to end

The program in `cmd/poc` wires everything together and runs a short scenario:

1. create an in-memory backend
2. construct the unmodified Fabric sample chaincode
3. create a runner around that chaincode and backend
4. invoke `InitLedger` to seed the six standard sample assets
5. invoke `CreateAsset` to add `asset7`
6. invoke `TransferAsset` to change the owner of `asset2` to `Priya`
7. query `GetAllAssets`
8. print the returned assets as formatted JSON

The important point is not the asset business flow itself. The important point is that every one of those operations is executed through normal Fabric-style contract code while the execution loop and state layer are entirely provided by the PoC.

### 8. What this demonstrates for Fabric-X

This repository does not claim that classic Fabric chaincode runs on Fabric-X today. There is no direct Fabric-X service integration here yet.

What it does demonstrate is the architectural claim behind the proposal:

- chaincode logic can stay unchanged
- the classic peer-managed execution loop can be replaced
- shim behavior can be recreated by an adapter
- ledger/state access can be hidden behind a backend interface

That is the starting point for a real Fabric-X migration path. The next step would be to replace the in-memory backend with calls into actual Fabric-X services while preserving the same contract and the same shim-facing adapter.


## Prerequisites

- Go `1.26.x`
- network access the first time Go modules are downloaded

If the default Go proxy is unreliable, use `GOPROXY=direct`.

## How to run

Fetch dependencies:

```bash
go mod tidy
```

Run the tests:

```bash
go test ./...
```

Run the demo:

```bash
go run ./cmd/poc
```

## Expected demo behavior

The demo should print a JSON array containing:

- the six standard assets from the Fabric sample
- `asset7`
- `asset2` with owner `Priya`

You will also see runner logs for:

- `InitLedger`
- `CreateAsset`
- `TransferAsset`
- `GetAllAssets`

<img width="842" height="844" alt="image" src="https://github.com/user-attachments/assets/e82ea580-b26f-491f-92df-4efd3718e3be" />


## Next step

The next useful step is replacing `shimadapter.InMemoryStateClient` with a real service-backed implementation while leaving the Fabric sample contract unchanged.
