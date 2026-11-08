package ringbuffer

// v4: batching on top of v3, publishes n items with a single atomic store

// writes as many items as fit (blocks until at least one slot is free), returns count written
func (q *SPSCCached[T]) PushBatch(vs []T) int {
	if len(vs) == 0 {
		return 0
	}
	t := q.tail.Load()
	size := uint64(len(q.buf))
	free := size - (t - q.headCache)
	if free == 0 {
		var b backoff
		for {
			q.headCache = q.head.Load()
			if free = size - (t - q.headCache); free != 0 {
				break
			}
			b.wait()
		}
	}
	n := min(uint64(len(vs)), free)
	for i := uint64(0); i < n; i++ {
		q.buf[(t+i)&q.mask] = vs[i]
	}
	q.tail.Store(t + n)
	return int(n)
}

// reads up to len(out) items (blocks until at least one is available), returns count read
func (q *SPSCCached[T]) PopBatch(out []T) int {
	if len(out) == 0 {
		return 0
	}
	h := q.head.Load()
	avail := q.tailCache - h
	if avail == 0 {
		var b backoff
		for {
			q.tailCache = q.tail.Load()
			if avail = q.tailCache - h; avail != 0 {
				break
			}
			b.wait()
		}
	}
	n := min(uint64(len(out)), avail)
	for i := uint64(0); i < n; i++ {
		out[i] = q.buf[(h+i)&q.mask]
	}
	q.head.Store(h + n)
	return int(n)
}
