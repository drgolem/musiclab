package ringbuffer

import (
	"errors"
	"fmt"
)

type RingBuffer struct {
	buf      []byte
	capacity int
	readPos  int
	writePos int
	size     int
}

func NewRingBuffer(capacity int) RingBuffer {
	rb := RingBuffer{
		capacity: capacity,
		buf:      make([]byte, capacity),
	}

	return rb
}

func (rb *RingBuffer) Capacity() int {
	return rb.capacity
}

func (rb *RingBuffer) Size() int {
	return rb.size
}

func (rb *RingBuffer) AvailableWriteSize() int {
	return rb.capacity - rb.size
}

func (rb *RingBuffer) IsFull() bool {
	return (rb.capacity - rb.size) == 0
}

func (rb *RingBuffer) Reset() {
	rb.size = 0
	rb.readPos = 0
	rb.writePos = 0
}

func (rb *RingBuffer) Read(n int, dst []byte) (int, error) {
	if rb.size < n {
		return 0, errors.New(fmt.Sprintf("invalid n. sz: %d, n: %d", rb.size, n))
	}
	if rb.readPos+n <= rb.capacity {
		copy(dst, rb.buf[rb.readPos:rb.readPos+n])
		rb.readPos += n
		rb.size -= n
		return n, nil
	}

	copy(dst, rb.buf[rb.readPos:rb.capacity])
	copy(dst[rb.capacity-rb.readPos:], rb.buf[0:n-(rb.capacity-rb.readPos)])

	rb.readPos = n - (rb.capacity - rb.readPos)
	rb.size -= n

	return n, nil
}

func (rb *RingBuffer) Write(data []byte) (int, error) {
	szData := len(data)
	if szData > rb.Capacity()-rb.Size() {
		errMsg := fmt.Sprintf("data len exceed capacity. %d > %d", szData, rb.Capacity()-rb.Size())
		return 0, errors.New(errMsg)
	}

	szToEnd := rb.writePos + szData
	if szToEnd <= rb.Capacity() {
		copy(rb.buf[rb.writePos:], data)
		rb.writePos += len(data)
	} else {
		sz1 := rb.Capacity() - rb.writePos
		copy(rb.buf[rb.writePos:], data[:sz1])
		copy(rb.buf[0:], data[sz1:])
		rb.writePos = szData - sz1
	}
	rb.size += szData

	return szData, nil
}
