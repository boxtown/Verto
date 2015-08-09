package mux

import (
	"testing"
)

func BenchmarkLPM(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	m := &DefaultMatcher{}
	m.Add("/user/{name}", nil)
	for i := 0; i < b.N; i++ {
		m.LongestPrefixMatch("/user/gordon")
	}
}

func BenchmarkMatch(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	m := &DefaultMatcher{}
	m.Add("/user/{name}", nil)
	for i := 0; i < b.N; i++ {
		m.Match("/user/gordon")
	}
}
