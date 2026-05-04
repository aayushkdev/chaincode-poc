package shimadapter

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hyperledger/fabric-chaincode-go/v2/shim"
	"github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type FabricXStateClient interface {
	GetState(namespace, key string) ([]byte, error)
	GetMultipleStates(namespace string, keys ...string) ([][]byte, error)
	PutState(namespace, key string, value []byte) error
	DelState(namespace, key string) error
	GetStateByRange(namespace, startKey, endKey string) ([]StateResult, error)
}

type StateResult struct {
	Key   string
	Value []byte
}

type FabricXStub struct {
	txID      string
	txTime    time.Time
	args      [][]byte
	namespace string
	client    FabricXStateClient

	writeSet map[string]writeEntry
	readSet  map[string]struct{}
	event    *peer.ChaincodeEvent
}

type writeEntry struct {
	value   []byte
	deleted bool
}

func NewFabricXStub(namespace string, args [][]byte, client FabricXStateClient) *FabricXStub {
	ts := time.Now().UTC()
	h := sha256.New()
	for _, arg := range args {
		_, _ = h.Write(arg)
	}
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, uint64(ts.UnixNano()))
	_, _ = h.Write(buf)

	return &FabricXStub{
		txID:      hex.EncodeToString(h.Sum(nil))[:32],
		txTime:    ts,
		args:      cloneArgs(args),
		namespace: namespace,
		client:    client,
		writeSet:  make(map[string]writeEntry),
		readSet:   make(map[string]struct{}),
	}
}

func (s *FabricXStub) GetArgs() [][]byte {
	return cloneArgs(s.args)
}

func (s *FabricXStub) GetStringArgs() []string {
	out := make([]string, len(s.args))
	for i, arg := range s.args {
		out[i] = string(arg)
	}
	return out
}

func (s *FabricXStub) GetFunctionAndParameters() (string, []string) {
	if len(s.args) == 0 {
		return "", nil
	}

	function := string(s.args[0])
	params := make([]string, 0, len(s.args)-1)
	for _, arg := range s.args[1:] {
		params = append(params, string(arg))
	}
	return function, params
}

func (s *FabricXStub) GetArgsSlice() ([]byte, error) {
	var out []byte
	for _, arg := range s.args {
		out = append(out, arg...)
	}
	return out, nil
}

func (s *FabricXStub) GetTxID() string {
	return s.txID
}

func (s *FabricXStub) GetChannelID() string {
	return s.namespace
}

func (s *FabricXStub) InvokeChaincode(_ string, _ [][]byte, _ string) *peer.Response {
	return &peer.Response{Status: 500, Message: "cross-chaincode invoke: not supported in Fabric-X PoC"}
}

func (s *FabricXStub) GetState(key string) ([]byte, error) {
	if entry, ok := s.writeSet[key]; ok {
		if entry.deleted {
			return nil, nil
		}
		return cloneBytes(entry.value), nil
	}

	s.readSet[key] = struct{}{}
	return s.client.GetState(s.namespace, key)
}

func (s *FabricXStub) GetMultipleStates(keys ...string) ([][]byte, error) {
	if len(keys) == 0 {
		return nil, nil
	}

	values := make([][]byte, len(keys))
	var pending []string
	pendingIndex := make(map[string][]int)

	for i, key := range keys {
		if entry, ok := s.writeSet[key]; ok {
			if !entry.deleted {
				values[i] = cloneBytes(entry.value)
			}
			continue
		}
		s.readSet[key] = struct{}{}
		pending = append(pending, key)
		pendingIndex[key] = append(pendingIndex[key], i)
	}

	if len(pending) == 0 {
		return values, nil
	}

	persisted, err := s.client.GetMultipleStates(s.namespace, pending...)
	if err != nil {
		return nil, err
	}
	for i, value := range persisted {
		for _, index := range pendingIndex[pending[i]] {
			values[index] = cloneBytes(value)
		}
	}
	return values, nil
}

func (s *FabricXStub) PutState(key string, value []byte) error {
	if err := validateSimpleKey(key); err != nil {
		return err
	}
	s.writeSet[key] = writeEntry{value: cloneBytes(value)}
	return nil
}

func (s *FabricXStub) DelState(key string) error {
	if err := validateSimpleKey(key); err != nil {
		return err
	}
	s.writeSet[key] = writeEntry{deleted: true}
	return nil
}

func (s *FabricXStub) SetStateValidationParameter(_ string, _ []byte) error {
	return nil
}

func (s *FabricXStub) GetStateValidationParameter(_ string) ([]byte, error) {
	return nil, nil
}

func (s *FabricXStub) GetStateByRange(startKey, endKey string) (shim.StateQueryIteratorInterface, error) {
	results, err := s.client.GetStateByRange(s.namespace, startKey, endKey)
	if err != nil {
		return nil, err
	}
	return newResultsIterator(mergeWithWriteSet(results, s.writeSet, startKey, endKey)), nil
}

func (s *FabricXStub) GetStateByRangeWithPagination(startKey, endKey string, pageSize int32, bookmark string) (shim.StateQueryIteratorInterface, *peer.QueryResponseMetadata, error) {
	iter, err := s.GetStateByRange(startKey, endKey)
	if err != nil {
		return nil, nil, err
	}
	return iter, &peer.QueryResponseMetadata{
		FetchedRecordsCount: pageSize,
		Bookmark:            bookmark,
	}, nil
}

func (s *FabricXStub) GetStateByPartialCompositeKey(objectType string, keys []string) (shim.StateQueryIteratorInterface, error) {
	prefix, err := s.CreateCompositeKey(objectType, keys)
	if err != nil {
		return nil, err
	}
	return s.GetStateByRange(prefix, prefix+string(maxUnicodeRune))
}

func (s *FabricXStub) GetStateByPartialCompositeKeyWithPagination(objectType string, keys []string, pageSize int32, bookmark string) (shim.StateQueryIteratorInterface, *peer.QueryResponseMetadata, error) {
	iter, err := s.GetStateByPartialCompositeKey(objectType, keys)
	if err != nil {
		return nil, nil, err
	}
	return iter, &peer.QueryResponseMetadata{
		FetchedRecordsCount: pageSize,
		Bookmark:            bookmark,
	}, nil
}

func (s *FabricXStub) GetAllStatesCompositeKeyWithPagination(pageSize int32, bookmark string) (shim.StateQueryIteratorInterface, *peer.QueryResponseMetadata, error) {
	iter, err := s.GetStateByRange(compositeKeyNamespace, compositeKeyNamespace+string(maxUnicodeRune))
	if err != nil {
		return nil, nil, err
	}
	return iter, &peer.QueryResponseMetadata{
		FetchedRecordsCount: pageSize,
		Bookmark:            bookmark,
	}, nil
}

func (s *FabricXStub) CreateCompositeKey(objectType string, attributes []string) (string, error) {
	if err := validateCompositeSegment(objectType); err != nil {
		return "", fmt.Errorf("objectType: %w", err)
	}

	var b strings.Builder
	b.WriteString(compositeKeyNamespace)
	b.WriteString(objectType)
	b.WriteString(compositeKeyNamespace)
	for _, attr := range attributes {
		if err := validateCompositeSegment(attr); err != nil {
			return "", fmt.Errorf("attribute %q: %w", attr, err)
		}
		b.WriteString(attr)
		b.WriteString(compositeKeyNamespace)
	}
	return b.String(), nil
}

func (s *FabricXStub) SplitCompositeKey(compositeKey string) (string, []string, error) {
	if !strings.HasPrefix(compositeKey, compositeKeyNamespace) {
		return "", nil, fmt.Errorf("not a composite key")
	}
	parts := strings.Split(compositeKey[1:], compositeKeyNamespace)
	if len(parts) < 2 {
		return "", nil, fmt.Errorf("malformed composite key")
	}
	return parts[0], parts[1 : len(parts)-1], nil
}

func (s *FabricXStub) GetQueryResult(_ string) (shim.StateQueryIteratorInterface, error) {
	return nil, fmt.Errorf("rich queries: not supported in Fabric-X PoC")
}

func (s *FabricXStub) GetQueryResultWithPagination(_ string, _ int32, _ string) (shim.StateQueryIteratorInterface, *peer.QueryResponseMetadata, error) {
	return nil, nil, fmt.Errorf("rich queries: not supported in Fabric-X PoC")
}

func (s *FabricXStub) GetHistoryForKey(_ string) (shim.HistoryQueryIteratorInterface, error) {
	return nil, fmt.Errorf("history queries: not supported in Fabric-X PoC")
}

func (s *FabricXStub) GetPrivateData(_, _ string) ([]byte, error) {
	return nil, fmt.Errorf("private data: not supported in Fabric-X PoC")
}

func (s *FabricXStub) GetMultiplePrivateData(_ string, _ ...string) ([][]byte, error) {
	return nil, fmt.Errorf("private data: not supported in Fabric-X PoC")
}

func (s *FabricXStub) GetPrivateDataHash(_, _ string) ([]byte, error) {
	return nil, fmt.Errorf("private data: not supported in Fabric-X PoC")
}

func (s *FabricXStub) PutPrivateData(_ string, _ string, _ []byte) error {
	return fmt.Errorf("private data: not supported in Fabric-X PoC")
}

func (s *FabricXStub) DelPrivateData(_, _ string) error {
	return fmt.Errorf("private data: not supported in Fabric-X PoC")
}

func (s *FabricXStub) PurgePrivateData(_, _ string) error {
	return fmt.Errorf("private data: not supported in Fabric-X PoC")
}

func (s *FabricXStub) SetPrivateDataValidationParameter(_, _ string, _ []byte) error {
	return fmt.Errorf("private data: not supported in Fabric-X PoC")
}

func (s *FabricXStub) GetPrivateDataValidationParameter(_, _ string) ([]byte, error) {
	return nil, fmt.Errorf("private data: not supported in Fabric-X PoC")
}

func (s *FabricXStub) GetPrivateDataByRange(_, _, _ string) (shim.StateQueryIteratorInterface, error) {
	return nil, fmt.Errorf("private data: not supported in Fabric-X PoC")
}

func (s *FabricXStub) GetPrivateDataByPartialCompositeKey(_, _ string, _ []string) (shim.StateQueryIteratorInterface, error) {
	return nil, fmt.Errorf("private data: not supported in Fabric-X PoC")
}

func (s *FabricXStub) GetPrivateDataQueryResult(_, _ string) (shim.StateQueryIteratorInterface, error) {
	return nil, fmt.Errorf("private data: not supported in Fabric-X PoC")
}

func (s *FabricXStub) GetCreator() ([]byte, error) {
	return nil, nil
}

func (s *FabricXStub) GetTransient() (map[string][]byte, error) {
	return nil, nil
}

func (s *FabricXStub) GetBinding() ([]byte, error) {
	return nil, nil
}

func (s *FabricXStub) GetDecorations() map[string][]byte {
	return nil
}

func (s *FabricXStub) GetSignedProposal() (*peer.SignedProposal, error) {
	return nil, nil
}

func (s *FabricXStub) GetTxTimestamp() (*timestamppb.Timestamp, error) {
	return timestamppb.New(s.txTime), nil
}

func (s *FabricXStub) SetEvent(name string, payload []byte) error {
	s.event = &peer.ChaincodeEvent{
		TxId:        s.txID,
		ChaincodeId: s.namespace,
		EventName:   name,
		Payload:     cloneBytes(payload),
	}
	return nil
}

func (s *FabricXStub) StartWriteBatch() {}

func (s *FabricXStub) FinishWriteBatch() error { return nil }

func (s *FabricXStub) CommitWriteSet() error {
	for key, entry := range s.writeSet {
		var err error
		if entry.deleted {
			err = s.client.DelState(s.namespace, key)
		} else {
			err = s.client.PutState(s.namespace, key, entry.value)
		}
		if err != nil {
			return fmt.Errorf("commit write set for key %q: %w", key, err)
		}
	}
	return nil
}

func (s *FabricXStub) Event() *peer.ChaincodeEvent {
	if s.event == nil {
		return nil
	}
	return cloneEvent(s.event)
}

func (s *FabricXStub) MarshalReadSet() ([]byte, error) {
	keys := make([]string, 0, len(s.readSet))
	for key := range s.readSet {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return json.Marshal(keys)
}

func mergeWithWriteSet(persisted []StateResult, writeSet map[string]writeEntry, startKey, endKey string) []StateResult {
	merged := make(map[string][]byte, len(persisted))
	for _, result := range persisted {
		merged[result.Key] = cloneBytes(result.Value)
	}
	for key, entry := range writeSet {
		if !inRange(key, startKey, endKey) {
			continue
		}
		if entry.deleted {
			delete(merged, key)
			continue
		}
		merged[key] = cloneBytes(entry.value)
	}

	out := make([]StateResult, 0, len(merged))
	for key, value := range merged {
		out = append(out, StateResult{Key: key, Value: value})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

const (
	compositeKeyNamespace = "\x00"
	maxUnicodeRune        = '\U0010FFFF'
)

func validateSimpleKey(key string) error {
	if key == "" {
		return fmt.Errorf("key must not be empty")
	}
	if strings.HasPrefix(key, compositeKeyNamespace) {
		return fmt.Errorf("key must not start with %q", compositeKeyNamespace)
	}
	return nil
}

func validateCompositeSegment(value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("must be valid UTF-8")
	}
	if strings.ContainsRune(value, rune(0)) {
		return fmt.Errorf("must not contain U+0000")
	}
	if strings.ContainsRune(value, maxUnicodeRune) {
		return fmt.Errorf("must not contain U+10FFFF")
	}
	return nil
}

func inRange(key, startKey, endKey string) bool {
	return (startKey == "" || key >= startKey) && (endKey == "" || key < endKey)
}

func cloneArgs(args [][]byte) [][]byte {
	out := make([][]byte, len(args))
	for i, arg := range args {
		out[i] = cloneBytes(arg)
	}
	return out
}

func cloneEvent(event *peer.ChaincodeEvent) *peer.ChaincodeEvent {
	if event == nil {
		return nil
	}
	return &peer.ChaincodeEvent{
		ChaincodeId: event.ChaincodeId,
		TxId:        event.TxId,
		EventName:   event.EventName,
		Payload:     cloneBytes(event.Payload),
	}
}
