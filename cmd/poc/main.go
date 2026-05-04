package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aayush/chaincode-poc/runner"
	"github.com/aayush/chaincode-poc/samplecc"
	"github.com/aayush/chaincode-poc/shimadapter"
)

func main() {
	log.SetFlags(0)

	client := shimadapter.NewInMemoryStateClient()
	cc, err := samplecc.NewChaincode()
	if err != nil {
		log.Fatalf("create chaincode: %v", err)
	}

	r := runner.New(cc, client, "asset-transfer")

	mustInvoke(r, "InitLedger")
	mustInvoke(r, "CreateAsset", "asset7", "yellow", "15", "Aayush", "880")
	mustInvoke(r, "TransferAsset", "asset2", "Priya")

	fmt.Println("Current ledger snapshot via chaincode query:")
	resp := mustQuery(r, "GetAllAssets")

	var assets []samplecc.Asset
	if err := json.Unmarshal(resp.Response.Payload, &assets); err != nil {
		log.Fatalf("decode query result: %v", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(assets); err != nil {
		log.Fatalf("print assets: %v", err)
	}
}

func mustInvoke(r *runner.Runner, args ...string) runner.InvocationResult {
	result, err := r.Invoke(args...)
	if err != nil {
		log.Fatalf("invoke %v: %v", args, err)
	}
	return result
}

func mustQuery(r *runner.Runner, args ...string) runner.InvocationResult {
	result, err := r.Query(args...)
	if err != nil {
		log.Fatalf("query %v: %v", args, err)
	}
	return result
}
