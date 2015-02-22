package utils

import (
	"strings"
)

type CiMap struct {
	sensitive   map[string]interface{}
	insensitive map[string]interface{}
}

func NewMap() *CiMap {
	return &CiMap{
		make(map[string]interface{}),
		make(map[string]interface{}),
	}
}

func (cm *CiMap) Get(key string, matchCase bool) (interface{}, bool) {
	if matchCase {
		v, ok := cm.sensitive[key]
		return v, ok
	}

	key = strings.ToLower(key)
	v, ok := cm.insensitive[key]
	return v, ok
}

func (cm *CiMap) Set(key string, value interface{}) {
	iKey := strings.ToLower(key)
	cm.insensitive[iKey] = value
	cm.sensitive[key] = value
}
