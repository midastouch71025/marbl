package producer

import "sync"

// InFlight tracks task IDs currently being delivered to avoid duplicate in-process sends.
type InFlight struct {
	mu sync.Mutex
	m  map[int64]struct{}
}

func (i *InFlight) TryAdd(id int64) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.m == nil {
		i.m = make(map[int64]struct{})
	}
	if _, ok := i.m[id]; ok {
		return false
	}
	i.m[id] = struct{}{}
	return true
}

func (i *InFlight) Remove(id int64) {
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.m, id)
}

func (i *InFlight) Len() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	return len(i.m)
}
