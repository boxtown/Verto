package verto

import (
	"sync"
)

type LifeTime int64

const (
	SINGLETON LifeTime = iota
	REQUEST
)

// FactoryFn represents a factory function for lazy initialization
// of injectable objects. FactoryFn takes in a ReadOnlyInjections interface
// to allow ReadOnly access to the outer Injections container
type FactoryFn func(r ReadOnlyInjections) interface{}

// Injections is a thread-safe map of keys to data objects.
// Injections is used by Verto to allow outside dependencies to
// be injected by the user into request handlers and plugins.
type Injections interface {
	// Get returns the value associated with key in Injections
	// or nil if the key does not exist. Get will evaluate any
	// factory functions associated with key if they have not
	// already been evaluated
	Get(key string) interface{}

	// TryGet returns the value associated with key in Injections
	// and a boolean indicating if the retrieval was successful or not.
	// If TryGet is successful and the key is associated with a un-evaluated
	// factory function, the factory function will be evaluated
	TryGet(key string) (interface{}, bool)

	// Set associates a key with a value in Injections.
	Set(key string, value interface{})

	// Lazy associates a factory function with a key that will lazily initialize
	// an object using the factory function when the key is retrieved.
	Lazy(key string, fn FactoryFn, lifetime LifeTime)

	// Delete deletes a key-value association in Injections.
	Delete(key string)

	// Clear deletes all key-value associations in Injections.
	Clear()
}

// ReadOnlyInjections is provides a read-only interface
// for Injections. It is up to the implementor to ensure
// the implementation is read-only.
type ReadOnlyInjections interface {
	Get(key string) interface{}
	TryGet(key string) (interface{}, bool)
}

// IContainer is a master container for Injections and should only
// be instantiated once with NewContainer. The container should be
// cloned for each request and the request should use the clone instead
// of the master container. IContainer implements the Injections interface
// but, generally speaking, the Get and TryGet functions should only be called
// from requests on cloned containers.
type IContainer struct {
	mutex *sync.RWMutex
	data  map[string]*injectionDef
}

// NewContainer returns a pointer to a newly initiated Injections Container.
func NewContainer() *IContainer {
	return &IContainer{
		mutex: &sync.RWMutex{},
		data:  make(map[string]*injectionDef),
	}
}

// Clone returns a thread-specific clone of the IContainer.
func (i *IContainer) Clone() *IClone {
	return &IClone{
		IContainer: i,
		mutex:      &sync.RWMutex{},
		threadData: make(map[string]interface{}),
	}
}

// Get calls TryGet and disposes of the returned bool.
// Thus it is possible to get nil values for a key if the
// key does not exist or is a lazy function with a per-request
// LifeTime
func (i *IContainer) Get(key string) interface{} {
	v, _ := i.TryGet(key)
	return v
}

// TryGet attempts to retrieve the value associated with the
// passed in key and returns the value and a success boolean.
// If the key does not exist or is associated with a per-request
// LifeTime lazy function, a nil interface and false will be returned.
// Otherwise, the associated value and true is returned. This
// function will evaluate lazy functions with a singleton LifeTime
func (i *IContainer) TryGet(key string) (interface{}, bool) {
	i.mutex.RLock()

	v, ok := i.data[key]
	if !ok {
		// If no association exists, release the lock
		// and return negative
		i.mutex.RUnlock()
		return nil, false
	}

	var val interface{}
	if v.obj == nil && v.fn != nil {
		// if the definition needs to be lazily evaluated,
		// we have to release the read lock and re-lock
		// with the write lock

		i.mutex.RUnlock()
		i.mutex.Lock()

		// double check condition after acquiring write lock
		if v.obj == nil && v.fn != nil {
			// condition still holds, proceed to evaluation logic
			if v.lifetime == SINGLETON {
				// If the lifetime is singleton, then we evaluate
				// the factory function, release the write-lock and return
				// the evaluated value
				val = v.fn(readOnlyInjections{&IClone{IContainer: i}})
				v.obj = val
				i.mutex.Unlock()
				return val, true
			} else {
				// Since this is the master container, it doesn't make
				// sense to evaluate per-request lifetime functions.
				// Release the write-lock and return negative
				i.mutex.Unlock()
				return nil, false
			}
		} else if v.obj != nil {
			// if object has been evaluated since we released the read-lock
			// and acquired the write-lock, release write-lock and return value
			val = v.obj
			i.mutex.Unlock()
			return val, true
		}
	}

	// Value exists, release the read-lock and return the value
	val = v.obj
	i.mutex.RUnlock()
	return val, true
}

// Set associates a value with a key for this container and all its
// clones. Values always have a singleton LifeTime.
func (i *IContainer) Set(key string, value interface{}) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.data[key] = &injectionDef{obj: value, lifetime: SINGLETON}
}

// Lazy associates a factory function with the passed in LifeTime with the
// passed in key for this container and all its clones. The factory function
// will be evaluated upon retrieval through Get or TryGet.
// Per-request LifeTime functions can only be evaluated by clones.
func (i *IContainer) Lazy(key string, fn FactoryFn, lifetime LifeTime) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.data[key] = &injectionDef{fn: fn, lifetime: lifetime}
}

// Delete deletes the value or factory function associated
// with the key for this container. This function will not
// delete per-request evaluated values for existing clones.
func (i *IContainer) Delete(key string) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	delete(i.data, key)
}

// Clear clears out all key-value (or factory) associations
// for this container. This function will not delete per-request
// evaluated values for existing clones.
func (i *IContainer) Clear() {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.data = make(map[string]*injectionDef)
}

// IClone is a cloned version of the IContainer
// and should have a 1-1 relation with an http.Request.
// IClone maintains a request-specific map for evaluating
// per-request factory functions
type IClone struct {
	*IContainer

	mutex      *sync.RWMutex
	threadData map[string]interface{}
}

// Get calls TryGet on the IClone and disregards the
// boolean value indicating success
func (i *IClone) Get(key string) interface{} {
	v, _ := i.TryGet(key)
	return v
}

// TryGet attempts to retrieve the desired value first
// from the global injection store and then from the thread-specific
// map. If need be, lazy factory functions are evaluated first.
// IClone's TryGet will, unlike the TryGet for the IContainer, evaluate
// per-request factory functions. Each IClone will execute the per-request
// function only once in its lifetime. The per-request scoping comes from
// the IContainer spawning an IClone per incoming http.Request
func (i *IClone) TryGet(key string) (interface{}, bool) {
	i.IContainer.mutex.RLock()

	v, ok := i.IContainer.data[key]
	if !ok {
		// If no key-value association exists, release the read-lock
		// and return negative
		i.IContainer.mutex.RUnlock()
		return nil, false
	}

	var val interface{}
	if v.obj == nil && v.fn != nil {
		// If the definition needs to be lazily evaluated,
		// then we must release the read-lock and proceed with more
		// specific locking
		i.IContainer.mutex.RUnlock()

		// First check for value in threadData
		i.mutex.RLock()
		if check, ok := i.threadData[key]; ok {
			i.mutex.RUnlock()
			return check, true
		}
		i.mutex.RUnlock()

		// Value not in thread data, try to evaluate fn
		// double-check condition first
		i.IContainer.mutex.Lock()
		if v.obj == nil && v.fn != nil {
			// Condition still holds after checking thread data
			// and acquiring write-lock, proceed to evaluation logic
			if v.lifetime == SINGLETON {
				// Lifetime is singleton. Evaluate function, set value,
				// release the write-lock and return the value
				val = v.fn(readOnlyInjections{i})
				v.obj = val
				i.IContainer.mutex.Unlock()
				return val, true
			} else {
				// Life time is per-request. Release the unnecessary
				// master container write-lock and acquire the thread
				// specific write-lock
				i.IContainer.mutex.Unlock()
				i.mutex.Lock()

				if check, ok := i.threadData[key]; ok {
					// If the function has been evaluated since we last checked
					// and acquired the thread specific write-lock, then just release
					// the write-lock and return the value
					i.mutex.Unlock()
					return check, true
				} else {
					// Condition still holds after acquiring thread specific write-lock,
					// evaluate the function, set the value in thread specific data,
					// release the thread specific write-lock and return the value
					val = v.fn(readOnlyInjections{i})
					i.threadData[key] = val
					i.mutex.Unlock()
					return val, true
				}
			}
		} else if v.obj != nil {
			// Object has been evaluated since we released the read-lock and
			// acquired the write-lock. Release the write-lock and return the
			// evaluated value
			val = v.obj
			i.IContainer.mutex.Unlock()
			return val, true
		}
	}

	// Value exists, release read-lock and return value
	val = v.obj
	i.IContainer.mutex.RUnlock()
	return val, true
}

// Delete will delete the key value association in the global
// injections container as well as in the thread-specific map.
// Delete does not affect any other IClone instances
func (i *IClone) Delete(key string) {
	i.IContainer.Delete(key)
	i.mutex.Lock()
	defer i.mutex.Unlock()

	delete(i.threadData, key)
}

// Clear will clear all key value associations in the global
// injections container as well as in the thread-specific map.
// Clear does not affect any other IClone instances.
func (i *IClone) Clear() {
	i.IContainer.Clear()
	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.threadData = make(map[string]interface{})
}

// readOnlyInjections is an implementation of the ReadOnlyInjections
// interface in order to provide factory functions with read access
// to the outer container.
type readOnlyInjections struct {
	*IClone
}

// Get calls TryGet on the readOnlyInjections instance
// and disregards the success bool
func (r readOnlyInjections) Get(key string) interface{} {
	v, _ := r.TryGet(key)
	return v
}

// TryGet attempts to retrieve the desired value first from the global injection
// map, and then from the thread-specific map. Lazy functions are NOT evaluated.
func (r readOnlyInjections) TryGet(key string) (interface{}, bool) {
	v, ok := r.IContainer.data[key]
	if !ok {
		return nil, false
	}
	if v.obj != nil {
		return v.obj, true
	}
	if r.threadData != nil {
		if v, ok := r.threadData[key]; ok {
			return v, true
		}
	}
	return nil, false
}

// struct containing injection definition
// information
type injectionDef struct {
	obj      interface{}
	fn       FactoryFn
	lifetime LifeTime
}
