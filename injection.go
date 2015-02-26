package verto

import (
	"sync"
)

// Injections is a thread-safe map of keys to data objects.
// Injections is used by Verto to allow outside dependencies to
// be injected by the user into request handlers and plugins.
type Injections struct {
	mutex *sync.RWMutex
	data  map[string]interface{}
}

// NewInjections returns a pointer to a newly initiated Injections object.
func NewInjections() *Injections {
	return &Injections{
		mutex: &sync.RWMutex{},
		data:  make(map[string]interface{}),
	}
}

// Get returns the value associated with key in Injections
// or nil if the key does not exist.
func (i *Injections) Get(key string) interface{} {
	i.mutex.RLock()
	defer i.mutex.RUnlock()

	v, ok := i.data[key]
	if !ok {
		return nil
	}
	return v
}

// TryGet returns the value associated with key in Injections
// and a boolean indicating if the retrieval was successful or not.
func (i *Injections) TryGet(key string) (interface{}, bool) {
	i.mutex.RLock()
	defer i.mutex.RUnlock()

	v, ok := i.data[key]
	return v, ok
}

// Set associates a key with a value in Injections.
func (i *Injections) Set(key string, value interface{}) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.data[key] = value
}

// Delete deletes a key-value association in Injections.
func (i *Injections) Delete(key string) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	delete(i.data, key)
}

// Clear deletes all key-value associations in Injections.
func (i *Injections) Clear() {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.data = make(map[string]interface{})
}
