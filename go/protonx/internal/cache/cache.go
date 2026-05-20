package cache

import (
	"sync"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

type CacheEntry struct {
	Link *proton.Link
	KR   *crypto.KeyRing
}

type Cache struct {
	data     map[string]*CacheEntry
	children map[string]map[string]struct{}
	mutex    sync.RWMutex
}

func New() *Cache {
	return &Cache{
		data:     make(map[string]*CacheEntry),
		children: make(map[string]map[string]struct{}),
	}
}

func (cache *Cache) Get(linkID string) *CacheEntry {
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()

	data, ok := cache.data[linkID]
	if !ok {
		return nil
	}
	return &CacheEntry{
		Link: data.Link,
		KR:   data.KR,
	}
}

func (cache *Cache) Insert(linkID string, link *proton.Link, kr *crypto.KeyRing) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	cache.data[linkID] = &CacheEntry{
		Link: link,
		KR:   kr,
	}

	if link == nil {
		return
	}

	data, ok := cache.children[link.ParentLinkID]
	if ok {
		data[link.LinkID] = struct{}{}
		cache.children[link.ParentLinkID] = data
	} else {
		cache.children[link.ParentLinkID] = map[string]struct{}{
			link.LinkID: struct{}{},
		}
	}
}

func (cache *Cache) removeNoLock(linkID string, includingChildren bool) {
	data, ok := cache.data[linkID]
	if !ok {
		return
	}
	link := data.Link
	delete(cache.data, linkID)

	dataParent, ok := cache.children[link.ParentLinkID]
	if ok {
		delete(dataParent, link.LinkID)
		cache.children[link.ParentLinkID] = dataParent
	}

	if !includingChildren {
		return
	}

	dataChildren, ok := cache.children[link.LinkID]
	if !ok {
		return
	}

	for k := range dataChildren {
		cache.removeNoLock(k, true)
	}
	delete(cache.children, link.LinkID)
}

func (cache *Cache) Remove(linkID string, includingChildren bool) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	cache.removeNoLock(linkID, includingChildren)
}
