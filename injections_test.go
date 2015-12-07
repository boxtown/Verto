package verto

import (
	"sync"
	"testing"
)

func TestIContainerGet(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed get."

	i := NewContainer()
	i.data["a"] = &injectionDef{obj: "b"}

	// Test basic get
	data := i.Get("a")
	if v, ok := data.(string); !ok {
		t.Errorf(err)
	} else if v != "b" {
		t.Errorf(err)
	}

	// Test thread safety
	for j := 0; j < 10; j++ {
		go func() {
			data := i.Get("a")
			if v, ok := data.(string); !ok {
				t.Errorf(err)
			} else if v != "b" {
				t.Errorf(err)
			}
		}()
	}
}

func TestIContainerTryGet(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed try get."

	i := NewContainer()
	i.data["a"] = &injectionDef{obj: "b"}

	// Test basic try get
	data, ok := i.TryGet("a")
	if !ok {
		t.Errorf(err)
	}
	if v, ok := data.(string); !ok {
		t.Errorf(err)
	} else if v != "b" {
		t.Errorf(err)
	}

	// Test bad try get
	_, ok = i.TryGet("b")
	if ok {
		t.Errorf(err)
	}

	// Test thread safety
	var wg sync.WaitGroup
	for j := 0; j < 10; j++ {
		wg.Add(1)
		go func() {
			data, ok := i.TryGet("a")
			if !ok {
				t.Errorf(err)
			}
			if v, ok := data.(string); !ok {
				t.Errorf(err)
			} else if v != "b" {
				t.Errorf(err)
			}
			wg.Done()
		}()
	}
	wg.Wait()

	// Test lazy init
	i.data["a"] = &injectionDef{fn: func(r ReadOnlyInjections) interface{} { return "b" }, lifetime: SINGLETON}
	wg = sync.WaitGroup{}
	for j := 0; j < 10; j++ {
		wg.Add(1)
		go func() {
			data, _ := i.TryGet("a")
			if v, ok := data.(string); !ok {
				t.Errorf(err)
			} else if v != "b" {
				t.Errorf(err)
			}
			wg.Done()
		}()
	}
	wg.Wait()

	i.data["a"] = &injectionDef{fn: func(r ReadOnlyInjections) interface{} { return "b" }, lifetime: REQUEST}
	data, _ = i.TryGet("a")
	if data != nil {
		t.Errorf(err)
	}
}

func TestIContainerSet(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed set."

	i := NewContainer()

	// Test basic set
	i.Set("a", "b")
	data := i.Get("a")
	if v, ok := data.(string); !ok {
		t.Errorf(err)
	} else if v != "b" {
		t.Errorf(err)
	}

	// Test thread safety
	for j := 0; j < 10; j++ {
		go func(val int) {
			i.Set("a", val)
		}(j)
	}
}

func TestIContainerLazy(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed lazy."

	i := NewContainer()
	i.Set("key", "val")

	// Test basic lazy
	i.Lazy("a", func(r ReadOnlyInjections) interface{} { return "b" }, SINGLETON)
	data := i.Get("a")
	if v, ok := data.(string); !ok {
		t.Errorf(err)
	} else if v != "b" {
		t.Errorf(err)
	}

	// Test thread safety
	i.Lazy("b",
		func(r ReadOnlyInjections) interface{} {
			v := r.Get("b")
			if v != nil {
				// this shouldn't occur because
				// of the scoping
				return "c"
			}
			return "b"
		}, REQUEST)

	var wg sync.WaitGroup
	for j := 0; j < 10000; j++ {
		wg.Add(1)
		clone := i.Clone()
		go func(clone *IClone, val int) {
			v := clone.Get("b")
			if v != "b" {
				t.Errorf(err)
			}
			wg.Done()
		}(clone, j)
	}
	wg.Wait()

	data = i.Get("b")
	if data != nil {
		t.Errorf(err)
	}
}

func TestIContainerDelete(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed delete."

	i := NewContainer()
	i.Set("a", "b")
	i.Delete("a")
	_, ok := i.TryGet("a")
	if ok {
		t.Errorf(err)
	}
}

func TestIContainerClear(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed clear injections."

	i := NewContainer()

	i.Set("a", "b")
	i.Set("c", "d")
	i.Set("e", "f")

	i.Clear()
	_, ok := i.TryGet("a")
	if ok {
		t.Errorf(err)
	}
	_, ok = i.TryGet("c")
	if ok {
		t.Errorf(err)
	}
	_, ok = i.TryGet("e")
	if ok {
		t.Errorf(err)
	}
}
