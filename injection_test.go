package verto

import (
	"testing"
)

func TestInjectionsGet(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed get."

	i := NewInjections()
	i.data["a"] = "b"

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

func TestInjectionsTryGet(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed try get."

	i := NewInjections()
	i.data["a"] = "b"

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
	for j := 0; j < 10; j++ {
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
		}()
	}
}

func TestInjectionsSet(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed set."

	i := NewInjections()

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

func TestInjectionsDelete(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed delete."

	i := NewInjections()
	i.Set("a", "b")
	i.Delete("a")
	_, ok := i.TryGet("a")
	if ok {
		t.Errorf(err)
	}
}

func TestInjectionsClear(t *testing.T) {
	defer func() {
		err := recover()
		if err != nil {
			t.Errorf(err.(error).Error())
		}
	}()

	err := "Failed clear injections."

	i := NewInjections()

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
