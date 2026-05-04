package runner

import (
	"fmt"
	"log"

	"github.com/hyperledger/fabric-chaincode-go/v2/shim"
	"github.com/hyperledger/fabric-protos-go-apiv2/peer"

	"github.com/aayush/chaincode-poc/shimadapter"
)

type InvocationResult struct {
	Response *peer.Response
	TxID     string
	Event    *peer.ChaincodeEvent
}

type Runner struct {
	chaincode shim.Chaincode
	client    shimadapter.FabricXStateClient
	namespace string
}

func New(cc shim.Chaincode, client shimadapter.FabricXStateClient, namespace string) *Runner {
	return &Runner{
		chaincode: cc,
		client:    client,
		namespace: namespace,
	}
}

func (r *Runner) Invoke(args ...string) (InvocationResult, error) {
	return r.invoke(true, args...)
}

func (r *Runner) Query(args ...string) (InvocationResult, error) {
	return r.invoke(false, args...)
}

func (r *Runner) invoke(commit bool, args ...string) (InvocationResult, error) {
	if len(args) == 0 {
		return InvocationResult{}, fmt.Errorf("at least one chaincode argument is required")
	}

	byteArgs := make([][]byte, len(args))
	for i, arg := range args {
		byteArgs[i] = []byte(arg)
	}

	stub := shimadapter.NewFabricXStub(r.namespace, byteArgs, r.client)
	log.Printf("[runner] tx=%s fn=%s commit=%t", stub.GetTxID(), args[0], commit)

	var response *peer.Response
	if args[0] == "init" || args[0] == "Init" {
		response = r.chaincode.Init(stub)
	} else {
		response = r.chaincode.Invoke(stub)
	}
	if response == nil {
		return InvocationResult{}, fmt.Errorf("chaincode returned nil response")
	}

	result := InvocationResult{
		Response: response,
		TxID:     stub.GetTxID(),
		Event:    stub.Event(),
	}
	if response.Status != int32(shim.OK) {
		return result, fmt.Errorf("chaincode returned error (status %d): %s", response.Status, response.Message)
	}
	if !commit {
		return result, nil
	}
	if err := stub.CommitWriteSet(); err != nil {
		return result, fmt.Errorf("commit write set: %w", err)
	}
	return result, nil
}
