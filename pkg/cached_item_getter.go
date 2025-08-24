package pkg

import "context"

type CacheMonitor struct {
	NumMisses int
	NumHits   int
	MaxSize   int
}

func (cm *CacheMonitor) UpdateMaxSize(size int) {
	if size > cm.MaxSize {
		cm.MaxSize = size
	}
}

type CachedItemGetter struct {
	Getter  ItemGetter
	Monitor CacheMonitor
	cache   map[string][]byte
}

func (c *CachedItemGetter) Clear(resource string) {
	delete(c.cache, resource)
}

func (c *CachedItemGetter) Item(ctx context.Context, path string) ([]byte, error) {
	data, ok := c.cache[path]
	if ok {
		c.Monitor.NumHits += 1
		return data, nil
	}
	c.Monitor.NumMisses += 1
	data, err := c.Getter.Item(ctx, path)
	if err != nil {
		return data, err
	}
	c.cache[path] = data
	c.Monitor.UpdateMaxSize(len(c.cache))
	return data, err
}

func NewCachedItemGetter(getter ItemGetter) *CachedItemGetter {
	return &CachedItemGetter{
		Getter: getter,
		cache:  make(map[string][]byte),
	}
}
