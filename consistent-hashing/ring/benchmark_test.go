package ring

import (
	"fmt"
	"testing"
)

func BenchmarkGetShard(b *testing.B) {
	configs := []struct {
		shards int
		vnodes int
	}{
		{3, 100},
		{10, 100},
		{100, 100},
	}

	for _, cfg := range configs {
		b.Run(fmt.Sprintf("%d_shards_%d_vnodes", cfg.shards, cfg.vnodes), func(b *testing.B) {
			r := New(cfg.vnodes)
			for i := 0; i < cfg.shards; i++ {
				r.AddShard(fmt.Sprintf("shard-%d", i))
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				r.GetShard("user:12345")
			}
		})
	}
}

func BenchmarkAddShard(b *testing.B) {
	r := New(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.AddShard(fmt.Sprintf("shard-%d", i))
	}
}
