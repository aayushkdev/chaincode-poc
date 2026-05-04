package shimadapter

import (
	"fmt"
	"sort"
	"sync"
)

// InMemoryStateClient is a stand-in for Fabric-X query and commit services.
type InMemoryStateClient struct {
	mu    sync.RWMutex
	state map[string]map[string][]byte
}

func NewInMemoryStateClient() *InMemoryStateClient {
	return &InMemoryStateClient{
		state: make(map[string]map[string][]byte),
	}
}

func (c *InMemoryStateClient) namespaceState(namespace string) map[string][]byte {
	ns, ok := c.state[namespace]
	if !ok {
		ns = make(map[string][]byte)
		c.state[namespace] = ns
	}
	return ns
}

func (c *InMemoryStateClient) GetState(namespace, key string) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	value, ok := c.namespaceState(namespace)[key]
	if !ok {
		return nil, nil
	}
	return cloneBytes(value), nil
}

func (c *InMemoryStateClient) GetMultipleStates(namespace string, keys ...string) ([][]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(keys) == 0 {
		return nil, nil
	}

	values := make([][]byte, len(keys))
	ns := c.namespaceState(namespace)
	for i, key := range keys {
		if value, ok := ns[key]; ok {
			values[i] = cloneBytes(value)
		}
	}
	return values, nil
}

func (c *InMemoryStateClient) PutState(namespace, key string, value []byte) error {
	if key == "" {
		return fmt.Errorf("key must not be empty")
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.namespaceState(namespace)[key] = cloneBytes(value)
	return nil
}

func (c *InMemoryStateClient) DelState(namespace, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.namespaceState(namespace), key)
	return nil
}

func (c *InMemoryStateClient) GetStateByRange(namespace, startKey, endKey string) ([]StateResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var results []StateResult
	for key, value := range c.namespaceState(namespace) {
		if inRange(key, startKey, endKey) {
			results = append(results, StateResult{
				Key:   key,
				Value: cloneBytes(value),
			})
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Key < results[j].Key })
	return results, nil
}

func (c *InMemoryStateClient) Snapshot(namespace string) map[string][]byte {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make(map[string][]byte)
	for key, value := range c.namespaceState(namespace) {
		out[key] = cloneBytes(value)
	}
	return out
}

func cloneBytes(in []byte) []byte {
	if in == nil {
		return nil
	}
	out := make([]byte, len(in))
	copy(out, in)
	return out
}
