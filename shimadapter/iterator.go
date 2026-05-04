package shimadapter

import "github.com/hyperledger/fabric-protos-go-apiv2/ledger/queryresult"

type resultsIterator struct {
	results []StateResult
	pos     int
	closed  bool
}

func newResultsIterator(results []StateResult) *resultsIterator {
	return &resultsIterator{results: results}
}

func (it *resultsIterator) HasNext() bool {
	return !it.closed && it.pos < len(it.results)
}

func (it *resultsIterator) Next() (*queryresult.KV, error) {
	if !it.HasNext() {
		return nil, nil
	}

	result := it.results[it.pos]
	it.pos++
	return &queryresult.KV{
		Key:   result.Key,
		Value: cloneBytes(result.Value),
	}, nil
}

func (it *resultsIterator) Close() error {
	it.closed = true
	return nil
}
