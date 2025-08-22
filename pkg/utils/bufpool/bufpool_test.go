package bufpool

import (
	"testing"
)

func TestBufferPoolGetPut(t *testing.T) {
	// Get a buffer from the pool
	buf1 := Get()

	// Verify the buffer is the correct size
	if len(*buf1) != BufferSize {
		t.Errorf("Expected buffer size of %d, got %d", BufferSize, len(*buf1))
	}

	// Write some data to the buffer
	(*buf1)[0] = 42

	// Return the buffer to the pool
	Put(buf1)

	// Get another buffer, which should be the same one we just returned
	buf2 := Get()

	// Buffer may or may not be cleared, but we should be able to use it
	// Let's check if we have the expected buffer size
	if len(*buf2) != BufferSize {
		t.Errorf("Expected buffer size of %d, got %d", BufferSize, len(*buf2))
	}
}

func TestMultipleBuffers(t *testing.T) {
	// Get multiple buffers at once to ensure the pool can handle it
	const numBuffers = 10
	buffers := make([]B, numBuffers)

	// Get buffers from the pool
	for i := 0; i < numBuffers; i++ {
		buffers[i] = Get()
		// Verify each buffer is the correct size
		if len(*buffers[i]) != BufferSize {
			t.Errorf(
				"Buffer %d: Expected size of %d, got %d", i, BufferSize,
				len(*buffers[i]),
			)
		}
	}

	// Return all buffers to the pool
	for i := 0; i < numBuffers; i++ {
		Put(buffers[i])
	}
}

func BenchmarkGetPut(b *testing.B) {
	for i := 0; i < b.N; i++ {
		buf := Get()
		Put(buf)
	}
}

func BenchmarkGetPutParallel(b *testing.B) {
	b.RunParallel(
		func(pb *testing.PB) {
			for pb.Next() {
				buf := Get()
				Put(buf)
			}
		},
	)
}
