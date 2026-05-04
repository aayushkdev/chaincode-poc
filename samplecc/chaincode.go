package samplecc

import (
	"github.com/hyperledger/fabric-chaincode-go/v2/shim"
	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

func NewChaincode() (shim.Chaincode, error) {
	return contractapi.NewChaincode(&SmartContract{})
}
