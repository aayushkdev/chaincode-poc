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

## Project layout

- `samplecc/`
  The sample asset-transfer contract and the constructor that exposes it as `shim.Chaincode`.
- `runner/`
  The direct invocation path for `Init` and `Invoke`.
- `shimadapter/`
  The shim-compatible stub and the in-memory backend.
- `cmd/poc/`
  A runnable demo that executes a short asset-transfer flow and prints the resulting ledger state.
- `poc_test.go`
  Tests covering the adapter path with create, read, update, delete, transfer, and range-query behavior.

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

## How the demo works

The demo program in `cmd/poc` does the following:

1. creates an in-memory backend
2. constructs the Fabric sample chaincode
3. runs `InitLedger`
4. creates `asset7`
5. transfers `asset2` to `Priya`
6. queries `GetAllAssets`
7. prints the final ledger snapshot as JSON

The important part is that the contract is still using normal Fabric APIs while the backend is provided by the PoC adapter.



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


## Next step

The next useful step is replacing `shimadapter.InMemoryStateClient` with a real service-backed implementation while leaving the Fabric sample contract unchanged.
