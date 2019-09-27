// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: raft_internal.proto

package etcdserverpb

import (
	fmt "fmt"
	io "io"
	math "math"

	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/golang/protobuf/proto"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type RequestHeader struct {
	ID uint64 `protobuf:"varint,1,opt,name=ID,proto3" json:"ID,omitempty"`
	// username is a username that is associated with an auth token of gRPC connection
	Username string `protobuf:"bytes,2,opt,name=username,proto3" json:"username,omitempty"`
	// auth_revision is a revision number of auth.authStore. It is not related to mvcc
	AuthRevision         uint64   `protobuf:"varint,3,opt,name=auth_revision,json=authRevision,proto3" json:"auth_revision,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *RequestHeader) Reset()         { *m = RequestHeader{} }
func (m *RequestHeader) String() string { return proto.CompactTextString(m) }
func (*RequestHeader) ProtoMessage()    {}
func (*RequestHeader) Descriptor() ([]byte, []int) {
	return fileDescriptor_b4c9a9be0cfca103, []int{0}
}
func (m *RequestHeader) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *RequestHeader) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_RequestHeader.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalTo(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *RequestHeader) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RequestHeader.Merge(m, src)
}
func (m *RequestHeader) XXX_Size() int {
	return m.Size()
}
func (m *RequestHeader) XXX_DiscardUnknown() {
	xxx_messageInfo_RequestHeader.DiscardUnknown(m)
}

var xxx_messageInfo_RequestHeader proto.InternalMessageInfo

// An InternalRaftRequest is the union of all requests which can be
// sent via raft.
type InternalRaftRequest struct {
	Header *RequestHeader `protobuf:"bytes,100,opt,name=header,proto3" json:"header,omitempty"`
	ID     uint64         `protobuf:"varint,1,opt,name=ID,proto3" json:"ID,omitempty"`
	V2     *Request       `protobuf:"bytes,2,opt,name=v2,proto3" json:"v2,omitempty"`
	// RangeRequest range = 3;
	Put                  *PutRequest `protobuf:"bytes,4,opt,name=put,proto3" json:"put,omitempty"`
	XXX_NoUnkeyedLiteral struct{}    `json:"-"`
	XXX_unrecognized     []byte      `json:"-"`
	XXX_sizecache        int32       `json:"-"`
}

func (m *InternalRaftRequest) Reset()         { *m = InternalRaftRequest{} }
func (m *InternalRaftRequest) String() string { return proto.CompactTextString(m) }
func (*InternalRaftRequest) ProtoMessage()    {}
func (*InternalRaftRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_b4c9a9be0cfca103, []int{1}
}
func (m *InternalRaftRequest) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *InternalRaftRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_InternalRaftRequest.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalTo(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *InternalRaftRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_InternalRaftRequest.Merge(m, src)
}
func (m *InternalRaftRequest) XXX_Size() int {
	return m.Size()
}
func (m *InternalRaftRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_InternalRaftRequest.DiscardUnknown(m)
}

var xxx_messageInfo_InternalRaftRequest proto.InternalMessageInfo

func init() {
	proto.RegisterType((*RequestHeader)(nil), "etcdserverpb.RequestHeader")
	proto.RegisterType((*InternalRaftRequest)(nil), "etcdserverpb.InternalRaftRequest")
}

func init() { proto.RegisterFile("raft_internal.proto", fileDescriptor_b4c9a9be0cfca103) }

var fileDescriptor_b4c9a9be0cfca103 = []byte{
	// 270 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x12, 0x2e, 0x4a, 0x4c, 0x2b,
	0x89, 0xcf, 0xcc, 0x2b, 0x49, 0x2d, 0xca, 0x4b, 0xcc, 0xd1, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17,
	0xe2, 0x49, 0x2d, 0x49, 0x4e, 0x29, 0x4e, 0x2d, 0x2a, 0x4b, 0x2d, 0x2a, 0x48, 0x92, 0x12, 0x49,
	0xcf, 0x4f, 0xcf, 0x07, 0x4b, 0xe8, 0x83, 0x58, 0x10, 0x35, 0x52, 0x9c, 0x45, 0x05, 0xc9, 0x50,
	0xa6, 0x00, 0x42, 0x39, 0x44, 0x44, 0x29, 0x81, 0x8b, 0x37, 0x28, 0xb5, 0xb0, 0x34, 0xb5, 0xb8,
	0xc4, 0x23, 0x35, 0x31, 0x25, 0xb5, 0x48, 0x88, 0x8f, 0x8b, 0xc9, 0xd3, 0x45, 0x82, 0x51, 0x81,
	0x51, 0x83, 0x25, 0x88, 0xc9, 0xd3, 0x45, 0x48, 0x8a, 0x8b, 0xa3, 0xb4, 0x18, 0x64, 0x65, 0x6e,
	0xaa, 0x04, 0x93, 0x02, 0xa3, 0x06, 0x67, 0x10, 0x9c, 0x2f, 0xa4, 0xcc, 0xc5, 0x9b, 0x58, 0x5a,
	0x92, 0x11, 0x5f, 0x94, 0x5a, 0x96, 0x59, 0x9c, 0x99, 0x9f, 0x27, 0xc1, 0x0c, 0xd6, 0xc6, 0x03,
	0x12, 0x0c, 0x82, 0x8a, 0x29, 0xad, 0x65, 0xe4, 0x12, 0xf6, 0x84, 0xba, 0x3a, 0x28, 0x31, 0xad,
	0x04, 0x6a, 0x9d, 0x90, 0x31, 0x17, 0x5b, 0x06, 0xd8, 0x4a, 0x89, 0x14, 0x05, 0x46, 0x0d, 0x6e,
	0x23, 0x69, 0x3d, 0x64, 0xbf, 0xe8, 0xa1, 0xb8, 0x2a, 0x08, 0xaa, 0x14, 0xc3, 0x75, 0xaa, 0x5c,
	0x4c, 0x65, 0x46, 0x60, 0x77, 0x71, 0x1b, 0x89, 0x62, 0x35, 0x20, 0x88, 0xa9, 0xcc, 0x48, 0x48,
	0x8b, 0x8b, 0xb9, 0xa0, 0xb4, 0x44, 0x82, 0x05, 0xac, 0x4e, 0x02, 0x55, 0x5d, 0x40, 0x29, 0xcc,
	0x49, 0x41, 0x20, 0x45, 0x4e, 0x22, 0x27, 0x1e, 0xca, 0x31, 0x9c, 0x78, 0x24, 0xc7, 0x78, 0xe1,
	0x91, 0x1c, 0xe3, 0x83, 0x47, 0x72, 0x8c, 0x33, 0x1e, 0xcb, 0x31, 0x24, 0xb1, 0x81, 0x83, 0xcb,
	0x18, 0x10, 0x00, 0x00, 0xff, 0xff, 0x3d, 0x3a, 0x2f, 0x53, 0x86, 0x01, 0x00, 0x00,
}

func (m *RequestHeader) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *RequestHeader) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.ID != 0 {
		dAtA[i] = 0x8
		i++
		i = encodeVarintRaftInternal(dAtA, i, uint64(m.ID))
	}
	if len(m.Username) > 0 {
		dAtA[i] = 0x12
		i++
		i = encodeVarintRaftInternal(dAtA, i, uint64(len(m.Username)))
		i += copy(dAtA[i:], m.Username)
	}
	if m.AuthRevision != 0 {
		dAtA[i] = 0x18
		i++
		i = encodeVarintRaftInternal(dAtA, i, uint64(m.AuthRevision))
	}
	if m.XXX_unrecognized != nil {
		i += copy(dAtA[i:], m.XXX_unrecognized)
	}
	return i, nil
}

func (m *InternalRaftRequest) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *InternalRaftRequest) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.ID != 0 {
		dAtA[i] = 0x8
		i++
		i = encodeVarintRaftInternal(dAtA, i, uint64(m.ID))
	}
	if m.V2 != nil {
		dAtA[i] = 0x12
		i++
		i = encodeVarintRaftInternal(dAtA, i, uint64(m.V2.Size()))
		n1, err := m.V2.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n1
	}
	if m.Put != nil {
		dAtA[i] = 0x22
		i++
		i = encodeVarintRaftInternal(dAtA, i, uint64(m.Put.Size()))
		n2, err := m.Put.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n2
	}
	if m.Header != nil {
		dAtA[i] = 0xa2
		i++
		dAtA[i] = 0x6
		i++
		i = encodeVarintRaftInternal(dAtA, i, uint64(m.Header.Size()))
		n3, err := m.Header.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n3
	}
	if m.XXX_unrecognized != nil {
		i += copy(dAtA[i:], m.XXX_unrecognized)
	}
	return i, nil
}

func encodeVarintRaftInternal(dAtA []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return offset + 1
}
func (m *RequestHeader) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.ID != 0 {
		n += 1 + sovRaftInternal(uint64(m.ID))
	}
	l = len(m.Username)
	if l > 0 {
		n += 1 + l + sovRaftInternal(uint64(l))
	}
	if m.AuthRevision != 0 {
		n += 1 + sovRaftInternal(uint64(m.AuthRevision))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *InternalRaftRequest) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.ID != 0 {
		n += 1 + sovRaftInternal(uint64(m.ID))
	}
	if m.V2 != nil {
		l = m.V2.Size()
		n += 1 + l + sovRaftInternal(uint64(l))
	}
	if m.Put != nil {
		l = m.Put.Size()
		n += 1 + l + sovRaftInternal(uint64(l))
	}
	if m.Header != nil {
		l = m.Header.Size()
		n += 2 + l + sovRaftInternal(uint64(l))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func sovRaftInternal(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozRaftInternal(x uint64) (n int) {
	return sovRaftInternal(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *RequestHeader) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowRaftInternal
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: RequestHeader: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: RequestHeader: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field ID", wireType)
			}
			m.ID = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowRaftInternal
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.ID |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Username", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowRaftInternal
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthRaftInternal
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthRaftInternal
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Username = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field AuthRevision", wireType)
			}
			m.AuthRevision = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowRaftInternal
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.AuthRevision |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipRaftInternal(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthRaftInternal
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthRaftInternal
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *InternalRaftRequest) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowRaftInternal
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: InternalRaftRequest: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: InternalRaftRequest: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field ID", wireType)
			}
			m.ID = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowRaftInternal
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.ID |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field V2", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowRaftInternal
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthRaftInternal
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthRaftInternal
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.V2 == nil {
				m.V2 = &Request{}
			}
			if err := m.V2.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Put", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowRaftInternal
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthRaftInternal
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthRaftInternal
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Put == nil {
				m.Put = &PutRequest{}
			}
			if err := m.Put.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 100:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Header", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowRaftInternal
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthRaftInternal
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthRaftInternal
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Header == nil {
				m.Header = &RequestHeader{}
			}
			if err := m.Header.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipRaftInternal(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthRaftInternal
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthRaftInternal
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipRaftInternal(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowRaftInternal
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowRaftInternal
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
			return iNdEx, nil
		case 1:
			iNdEx += 8
			return iNdEx, nil
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowRaftInternal
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthRaftInternal
			}
			iNdEx += length
			if iNdEx < 0 {
				return 0, ErrInvalidLengthRaftInternal
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowRaftInternal
					}
					if iNdEx >= l {
						return 0, io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					innerWire |= (uint64(b) & 0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				innerWireType := int(innerWire & 0x7)
				if innerWireType == 4 {
					break
				}
				next, err := skipRaftInternal(dAtA[start:])
				if err != nil {
					return 0, err
				}
				iNdEx = start + next
				if iNdEx < 0 {
					return 0, ErrInvalidLengthRaftInternal
				}
			}
			return iNdEx, nil
		case 4:
			return iNdEx, nil
		case 5:
			iNdEx += 4
			return iNdEx, nil
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
	}
	panic("unreachable")
}

var (
	ErrInvalidLengthRaftInternal = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowRaftInternal   = fmt.Errorf("proto: integer overflow")
)
