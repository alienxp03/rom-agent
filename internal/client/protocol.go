package client

import (
	"bytes"
	"compress/zlib"
	"crypto/des"
	"encoding/binary"
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"
)

const (
	headLength      = 3
	idLength        = 2
	nonceHeadLength = 2
	subProtoID1     = 99
	subProtoID2     = 1
)

var desSecKey = []byte{95, 27, 5, 20, 131, 4, 8, 88}

// GameSend formats message data as NetManagerHelper.GameSend()
// Assumes EnableNonce=true, EnableEncrypt=false
func GameSend(id1, id2 byte, nonce []byte, data []byte) []byte {
	streamLength := headLength + idLength + len(data) + nonceHeadLength + len(nonce)
	contentLength := streamLength - headLength

	stream := make([]byte, streamLength)

	// Header (3 bytes): flag + content length (little endian 16-bit)
	stream[0] = 0
	stream[1] = byte(contentLength & 0xFF)
	stream[2] = byte((contentLength >> 8) & 0xFF)

	// Message IDs (2 bytes)
	stream[3] = id1
	stream[4] = id2

	// Nonce length (2 bytes, little endian)
	stream[5] = byte(len(nonce) & 0xFF)
	stream[6] = byte((len(nonce) >> 8) & 0xFF)

	// Nonce data
	copy(stream[headLength+idLength+nonceHeadLength:], nonce)

	// Protobuf data
	copy(stream[headLength+idLength+nonceHeadLength+len(nonce):], data)

	return stream
}

// GameReceive parses a single game message from raw bytes
// Returns id1, id2, payload data, and any error
func GameReceive(data []byte) (byte, byte, []byte, error) {
	if len(data) < headLength+idLength {
		return 0, 0, nil, fmt.Errorf("message too short: %d bytes", len(data))
	}

	// Parse header
	contentLength := int(binary.LittleEndian.Uint16(data[1:3]))
	expectedLength := headLength + contentLength

	if len(data) < expectedLength {
		return 0, 0, nil, fmt.Errorf("incomplete message: got %d bytes, expected %d", len(data), expectedLength)
	}

	// Extract message IDs
	id1 := data[headLength]
	id2 := data[headLength+1]

	// Extract payload (everything after the IDs)
	payload := data[headLength+idLength : expectedLength]

	return id1, id2, payload, nil
}

// GameFrame is a decoded TCP frame from the game socket.
type GameFrame struct {
	ID1     byte
	ID2     byte
	Payload []byte
}

// ParseGameStream parses as many complete protocol frames as possible from a stream buffer.
func ParseGameStream(data []byte) ([]GameFrame, []byte, error) {
	frames := make([]GameFrame, 0)
	offset := 0

	for {
		if len(data[offset:]) < headLength {
			break
		}

		flags := data[offset]
		bodyLength := int(binary.LittleEndian.Uint16(data[offset+1 : offset+3]))
		encryptedLen := bodyLength
		if flags&0x02 != 0 && bodyLength%8 != 0 {
			encryptedLen = bodyLength + (8 - (bodyLength % 8))
		}
		frameLen := headLength + encryptedLen
		if len(data[offset:]) < frameLen {
			break
		}

		body := append([]byte(nil), data[offset+headLength:offset+frameLen]...)
		if flags&0x02 != 0 {
			decrypted, err := decrypt(body)
			if err != nil {
				return nil, nil, err
			}
			body = decrypted[:bodyLength]
		}
		if flags&0x01 != 0 {
			decompressed, err := decompress(body[:bodyLength])
			if err != nil {
				return nil, nil, err
			}
			body = decompressed
		}

		parsed, err := unpackBody(body)
		if err != nil {
			return nil, nil, err
		}
		frames = append(frames, parsed...)
		offset += frameLen
	}

	remainder := append([]byte(nil), data[offset:]...)
	return frames, remainder, nil
}

func decrypt(encrypted []byte) ([]byte, error) {
	if len(encrypted)%des.BlockSize != 0 {
		return nil, fmt.Errorf("encrypted payload length %d is not a multiple of %d", len(encrypted), des.BlockSize)
	}

	block, err := des.NewCipher(desSecKey)
	if err != nil {
		return nil, err
	}

	out := make([]byte, len(encrypted))
	for i := 0; i < len(encrypted); i += des.BlockSize {
		block.Decrypt(out[i:i+des.BlockSize], encrypted[i:i+des.BlockSize])
	}
	return out, nil
}

func decompress(compressed []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func unpackBody(body []byte) ([]GameFrame, error) {
	if len(body) < idLength {
		return nil, nil
	}

	id1 := body[0]
	id2 := body[1]
	if id1 == subProtoID1 && id2 == subProtoID2 {
		return unpackSubProtocol(body)
	}

	return []GameFrame{{
		ID1:     id1,
		ID2:     id2,
		Payload: append([]byte(nil), body[idLength:]...),
	}}, nil
}

func unpackSubProtocol(body []byte) ([]GameFrame, error) {
	frames := make([]GameFrame, 0)
	offset := 4

	for offset+2 <= len(body) {
		n := int(binary.LittleEndian.Uint16(body[offset : offset+2]))
		offset += 2
		if n <= 0 || offset+n > len(body) {
			break
		}

		subFrames, err := unpackBody(body[offset : offset+n])
		if err != nil {
			return nil, err
		}
		frames = append(frames, subFrames...)
		offset += n
	}

	return frames, nil
}

// ParseProtobuf unmarshals protobuf data into the given message
func ParseProtobuf(data []byte, msg proto.Message) error {
	return proto.Unmarshal(data, msg)
}

// SerializeProtobuf marshals a protobuf message into bytes
func SerializeProtobuf(msg proto.Message) ([]byte, error) {
	return proto.Marshal(msg)
}
