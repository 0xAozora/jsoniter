package jsoniter

import (
	"io"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
)

func Test_writeByte_should_grow_buffer(t *testing.T) {
	should := require.New(t)
	stream := NewStream(ConfigDefault, nil, 1)
	stream.writeByte('1')
	should.Equal("1", string(stream.Buffer()))
	should.Equal(1, len(stream.buf))
	stream.writeByte('2')
	should.Equal("12", string(stream.Buffer()))
	should.Equal(2, len(stream.buf))
	stream.writeThreeBytes('3', '4', '5')
	should.Equal("12345", string(stream.Buffer()))
}

func Test_writeBytes_should_grow_buffer(t *testing.T) {
	should := require.New(t)
	stream := NewStream(ConfigDefault, nil, 1)
	stream.Write([]byte{'1', '2'})
	should.Equal("12", string(stream.Buffer()))
	should.Equal(2, len(stream.buf))
	stream.Write([]byte{'3', '4', '5', '6', '7'})
	should.Equal("1234567", string(stream.Buffer()))
	should.Equal(7, len(stream.buf))
}

func Test_writeIndention_should_grow_buffer(t *testing.T) {
	should := require.New(t)
	stream := NewStream(Config{IndentionStep: 2}.Froze(), nil, 1)
	stream.WriteVal([]int{1, 2, 3})
	should.Equal("[\n  1,\n  2,\n  3\n]", string(stream.Buffer()))
}

func Test_writeRaw_should_grow_buffer(t *testing.T) {
	should := require.New(t)
	stream := NewStream(ConfigDefault, nil, 1)
	stream.WriteRaw("123")
	should.Nil(stream.Error)
	should.Equal("123", string(stream.Buffer()))
}

func Test_writeString_should_grow_buffer(t *testing.T) {
	should := require.New(t)
	stream := NewStream(ConfigDefault, nil, 0)
	stream.WriteString("123")
	should.Nil(stream.Error)
	should.Equal(`"123"`, string(stream.Buffer()))
}

type NopWriter struct {
	bufferSize int
}

func (w *NopWriter) Write(p []byte) (n int, err error) {
	w.bufferSize = cap(p)
	return len(p), nil
}

func Test_flush_buffer_should_stop_grow_buffer(t *testing.T) {
	// Stream an array of a zillion zeros.
	writer := new(NopWriter)
	stream := NewStream(ConfigDefault, writer, 512)
	stream.WriteArrayStart()
	for i := 0; i < 10000000; i++ {
		stream.WriteInt(0)
		stream.WriteMore()
		stream.Flush()
	}
	stream.WriteInt(0)
	stream.WriteArrayEnd()

	// Confirm that the buffer didn't have to grow.
	should := require.New(t)

	// 512 is the internal buffer size set in NewEncoder
	//
	// Flush is called after each array element, so only the first 8 bytes of it
	// is ever used, and it is never extended. Capacity remains 512.
	should.Equal(512, writer.bufferSize)
}

func BenchmarkAlloc(b *testing.B) {

	inputs := []string{
		`1.1`, `1000`, `9223372036854775807`, `12.3`, `-12.3`, `720368.54775807`, `720368.547758075`,
		`1e1`, `1e+1`, `1e-1`, `1E1`, `1E+1`, `1E-1`, `-1e1`, `-1e+1`, `-1e-1`,
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			iter := ParseString(ConfigDefault, input+",")
			iter.readNumberAsStringAlloc()
		}
	}
}

func BenchmarkNoAlloc2(b *testing.B) {

	inputs := []string{
		`1.1`, `1000`, `9223372036854775807`, `12.3`, `-12.3`, `720368.54775807`, `720368.547758075`,
		`1e1`, `1e+1`, `1e-1`, `1E1`, `1E+1`, `1E-1`, `-1e1`, `-1e+1`, `-1e-1`,
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			iter := ParseString(ConfigDefault, input+",")
			iter.readNumberAsString2()
		}
	}
}

func BenchmarkNoAlloc(b *testing.B) {

	inputs := []string{
		`1.1`, `1000`, `9223372036854775807`, `12.3`, `-12.3`, `720368.54775807`, `720368.547758075`,
		`1e1`, `1e+1`, `1e-1`, `1E1`, `1E+1`, `1E-1`, `-1e1`, `-1e+1`, `-1e-1`,
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			iter := ParseString(ConfigDefault, input+",")
			iter.readNumberAsString()
		}
	}
}

func (iter *Iterator) readNumberAsStringAlloc() (ret string) {
	strBuf := [16]byte{}
	str := strBuf[0:0]
load_loop:
	for {
		for i := iter.head; i < iter.tail; i++ {
			c := iter.buf[i]
			switch c {
			case '+', '-', '.', 'e', 'E', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				str = append(str, c)
				continue
			default:
				iter.head = i
				break load_loop
			}
		}
		if !iter.loadMore() {
			break
		}
	}
	if iter.Error != nil && iter.Error != io.EOF {
		return
	}
	if len(str) == 0 {
		iter.ReportError("readNumberAsString", "invalid number")
	}
	return *(*string)(unsafe.Pointer(&str))
}

// Use underlying buffer
func (iter *Iterator) readNumberAsString2() (ret string) {
	var i int
	var str []byte
outer:
	for i = iter.head; i < iter.tail; i++ {
		c := iter.buf[i]
		switch c {
		case '+', '-', '.', 'e', 'E', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			continue
		default:
			str = iter.buf[iter.head:i]
			iter.head = i
			break outer
		}
	}
	if i >= iter.tail {
		readLen := iter.tail - iter.head
		copied := make([]byte, readLen, readLen*2)
		copy(copied, iter.buf[iter.head:iter.tail])
		iter.head = iter.tail
	outer2:
		for iter.Error == nil {
			c := iter.readByte()
			switch c {
			case '+', '-', '.', 'e', 'E', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
				copied = append(copied, c)
			default:
				iter.unreadByte()
				break outer2
			}
		}
		str = copied
	}
	if iter.Error != nil && iter.Error != io.EOF {
		return
	}
	if len(str) == 0 {
		iter.ReportError("readNumberAsString", "invalid number")
	}
	return *(*string)(unsafe.Pointer(&str))
}
