// Code generated by protoc-gen-go.
// source: mes_wrapper.proto
// DO NOT EDIT!

/*
Package swarm is a generated protocol buffer package.

It is generated from these files:
	mes_wrapper.proto

It has these top-level messages:
	PBWrapper
*/
package swarm

import proto "code.google.com/p/goprotobuf/proto"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = math.Inf

type PBWrapper_MessageType int32

const (
	PBWrapper_TEST        PBWrapper_MessageType = 0
	PBWrapper_DHT_MESSAGE PBWrapper_MessageType = 1
)

var PBWrapper_MessageType_name = map[int32]string{
	0: "TEST",
	1: "DHT_MESSAGE",
}
var PBWrapper_MessageType_value = map[string]int32{
	"TEST":        0,
	"DHT_MESSAGE": 1,
}

func (x PBWrapper_MessageType) Enum() *PBWrapper_MessageType {
	p := new(PBWrapper_MessageType)
	*p = x
	return p
}
func (x PBWrapper_MessageType) String() string {
	return proto.EnumName(PBWrapper_MessageType_name, int32(x))
}
func (x *PBWrapper_MessageType) UnmarshalJSON(data []byte) error {
	value, err := proto.UnmarshalJSONEnum(PBWrapper_MessageType_value, data, "PBWrapper_MessageType")
	if err != nil {
		return err
	}
	*x = PBWrapper_MessageType(value)
	return nil
}

type PBWrapper struct {
	Type             *PBWrapper_MessageType `protobuf:"varint,1,req,enum=swarm.PBWrapper_MessageType" json:"Type,omitempty"`
	Message          []byte                 `protobuf:"bytes,2,req" json:"Message,omitempty"`
	XXX_unrecognized []byte                 `json:"-"`
}

func (m *PBWrapper) Reset()         { *m = PBWrapper{} }
func (m *PBWrapper) String() string { return proto.CompactTextString(m) }
func (*PBWrapper) ProtoMessage()    {}

func (m *PBWrapper) GetType() PBWrapper_MessageType {
	if m != nil && m.Type != nil {
		return *m.Type
	}
	return PBWrapper_TEST
}

func (m *PBWrapper) GetMessage() []byte {
	if m != nil {
		return m.Message
	}
	return nil
}

func init() {
	proto.RegisterEnum("swarm.PBWrapper_MessageType", PBWrapper_MessageType_name, PBWrapper_MessageType_value)
}
