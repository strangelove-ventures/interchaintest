// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: penumbra/crypto/decaf377_frost/v1/decaf377_frost.proto

package decaf377_frostv1

import (
	fmt "fmt"
	proto "github.com/cosmos/gogoproto/proto"
	io "io"
	math "math"
	math_bits "math/bits"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

// A commitment to a polynomial, as a list of group elements.
type VerifiableSecretSharingCommitment struct {
	// Each of these bytes should be the serialization of a group element.
	Elements [][]byte `protobuf:"bytes,1,rep,name=elements,proto3" json:"elements,omitempty"`
}

func (m *VerifiableSecretSharingCommitment) Reset()         { *m = VerifiableSecretSharingCommitment{} }
func (m *VerifiableSecretSharingCommitment) String() string { return proto.CompactTextString(m) }
func (*VerifiableSecretSharingCommitment) ProtoMessage()    {}
func (*VerifiableSecretSharingCommitment) Descriptor() ([]byte, []int) {
	return fileDescriptor_b4822bfeb2663db2, []int{0}
}
func (m *VerifiableSecretSharingCommitment) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *VerifiableSecretSharingCommitment) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_VerifiableSecretSharingCommitment.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *VerifiableSecretSharingCommitment) XXX_Merge(src proto.Message) {
	xxx_messageInfo_VerifiableSecretSharingCommitment.Merge(m, src)
}
func (m *VerifiableSecretSharingCommitment) XXX_Size() int {
	return m.Size()
}
func (m *VerifiableSecretSharingCommitment) XXX_DiscardUnknown() {
	xxx_messageInfo_VerifiableSecretSharingCommitment.DiscardUnknown(m)
}

var xxx_messageInfo_VerifiableSecretSharingCommitment proto.InternalMessageInfo

func (m *VerifiableSecretSharingCommitment) GetElements() [][]byte {
	if m != nil {
		return m.Elements
	}
	return nil
}

// The public package sent in round 1 of the DKG protocol.
type DKGRound1Package struct {
	// A commitment to the polynomial for secret sharing.
	Commitment *VerifiableSecretSharingCommitment `protobuf:"bytes,1,opt,name=commitment,proto3" json:"commitment,omitempty"`
	// A proof of knowledge of the underlying secret being shared.
	ProofOfKnowledge []byte `protobuf:"bytes,2,opt,name=proof_of_knowledge,json=proofOfKnowledge,proto3" json:"proof_of_knowledge,omitempty"`
}

func (m *DKGRound1Package) Reset()         { *m = DKGRound1Package{} }
func (m *DKGRound1Package) String() string { return proto.CompactTextString(m) }
func (*DKGRound1Package) ProtoMessage()    {}
func (*DKGRound1Package) Descriptor() ([]byte, []int) {
	return fileDescriptor_b4822bfeb2663db2, []int{1}
}
func (m *DKGRound1Package) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *DKGRound1Package) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_DKGRound1Package.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *DKGRound1Package) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DKGRound1Package.Merge(m, src)
}
func (m *DKGRound1Package) XXX_Size() int {
	return m.Size()
}
func (m *DKGRound1Package) XXX_DiscardUnknown() {
	xxx_messageInfo_DKGRound1Package.DiscardUnknown(m)
}

var xxx_messageInfo_DKGRound1Package proto.InternalMessageInfo

func (m *DKGRound1Package) GetCommitment() *VerifiableSecretSharingCommitment {
	if m != nil {
		return m.Commitment
	}
	return nil
}

func (m *DKGRound1Package) GetProofOfKnowledge() []byte {
	if m != nil {
		return m.ProofOfKnowledge
	}
	return nil
}

// A share of the final signing key.
type SigningShare struct {
	// These bytes should be a valid scalar.
	Scalar []byte `protobuf:"bytes,1,opt,name=scalar,proto3" json:"scalar,omitempty"`
}

func (m *SigningShare) Reset()         { *m = SigningShare{} }
func (m *SigningShare) String() string { return proto.CompactTextString(m) }
func (*SigningShare) ProtoMessage()    {}
func (*SigningShare) Descriptor() ([]byte, []int) {
	return fileDescriptor_b4822bfeb2663db2, []int{2}
}
func (m *SigningShare) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *SigningShare) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_SigningShare.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *SigningShare) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SigningShare.Merge(m, src)
}
func (m *SigningShare) XXX_Size() int {
	return m.Size()
}
func (m *SigningShare) XXX_DiscardUnknown() {
	xxx_messageInfo_SigningShare.DiscardUnknown(m)
}

var xxx_messageInfo_SigningShare proto.InternalMessageInfo

func (m *SigningShare) GetScalar() []byte {
	if m != nil {
		return m.Scalar
	}
	return nil
}

// The per-participant package sent in round 2 of the DKG protocol.
type DKGRound2Package struct {
	// This is the share we're sending to that participant.
	SigningShare *SigningShare `protobuf:"bytes,1,opt,name=signing_share,json=signingShare,proto3" json:"signing_share,omitempty"`
}

func (m *DKGRound2Package) Reset()         { *m = DKGRound2Package{} }
func (m *DKGRound2Package) String() string { return proto.CompactTextString(m) }
func (*DKGRound2Package) ProtoMessage()    {}
func (*DKGRound2Package) Descriptor() ([]byte, []int) {
	return fileDescriptor_b4822bfeb2663db2, []int{3}
}
func (m *DKGRound2Package) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *DKGRound2Package) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_DKGRound2Package.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *DKGRound2Package) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DKGRound2Package.Merge(m, src)
}
func (m *DKGRound2Package) XXX_Size() int {
	return m.Size()
}
func (m *DKGRound2Package) XXX_DiscardUnknown() {
	xxx_messageInfo_DKGRound2Package.DiscardUnknown(m)
}

var xxx_messageInfo_DKGRound2Package proto.InternalMessageInfo

func (m *DKGRound2Package) GetSigningShare() *SigningShare {
	if m != nil {
		return m.SigningShare
	}
	return nil
}

// Represents a commitment to a nonce value.
type NonceCommitment struct {
	// These bytes should be a valid group element.
	Element []byte `protobuf:"bytes,1,opt,name=element,proto3" json:"element,omitempty"`
}

func (m *NonceCommitment) Reset()         { *m = NonceCommitment{} }
func (m *NonceCommitment) String() string { return proto.CompactTextString(m) }
func (*NonceCommitment) ProtoMessage()    {}
func (*NonceCommitment) Descriptor() ([]byte, []int) {
	return fileDescriptor_b4822bfeb2663db2, []int{4}
}
func (m *NonceCommitment) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *NonceCommitment) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_NonceCommitment.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *NonceCommitment) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NonceCommitment.Merge(m, src)
}
func (m *NonceCommitment) XXX_Size() int {
	return m.Size()
}
func (m *NonceCommitment) XXX_DiscardUnknown() {
	xxx_messageInfo_NonceCommitment.DiscardUnknown(m)
}

var xxx_messageInfo_NonceCommitment proto.InternalMessageInfo

func (m *NonceCommitment) GetElement() []byte {
	if m != nil {
		return m.Element
	}
	return nil
}

// Represents the commitments to nonces needed for signing.
type SigningCommitments struct {
	// One nonce to hide them.
	Hiding *NonceCommitment `protobuf:"bytes,1,opt,name=hiding,proto3" json:"hiding,omitempty"`
	// Another to bind them.
	Binding *NonceCommitment `protobuf:"bytes,2,opt,name=binding,proto3" json:"binding,omitempty"`
}

func (m *SigningCommitments) Reset()         { *m = SigningCommitments{} }
func (m *SigningCommitments) String() string { return proto.CompactTextString(m) }
func (*SigningCommitments) ProtoMessage()    {}
func (*SigningCommitments) Descriptor() ([]byte, []int) {
	return fileDescriptor_b4822bfeb2663db2, []int{5}
}
func (m *SigningCommitments) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *SigningCommitments) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_SigningCommitments.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *SigningCommitments) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SigningCommitments.Merge(m, src)
}
func (m *SigningCommitments) XXX_Size() int {
	return m.Size()
}
func (m *SigningCommitments) XXX_DiscardUnknown() {
	xxx_messageInfo_SigningCommitments.DiscardUnknown(m)
}

var xxx_messageInfo_SigningCommitments proto.InternalMessageInfo

func (m *SigningCommitments) GetHiding() *NonceCommitment {
	if m != nil {
		return m.Hiding
	}
	return nil
}

func (m *SigningCommitments) GetBinding() *NonceCommitment {
	if m != nil {
		return m.Binding
	}
	return nil
}

// A share of the final signature. These get aggregated to make the actual thing.
type SignatureShare struct {
	// These bytes should be a valid scalar.
	Scalar []byte `protobuf:"bytes,1,opt,name=scalar,proto3" json:"scalar,omitempty"`
}

func (m *SignatureShare) Reset()         { *m = SignatureShare{} }
func (m *SignatureShare) String() string { return proto.CompactTextString(m) }
func (*SignatureShare) ProtoMessage()    {}
func (*SignatureShare) Descriptor() ([]byte, []int) {
	return fileDescriptor_b4822bfeb2663db2, []int{6}
}
func (m *SignatureShare) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *SignatureShare) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_SignatureShare.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *SignatureShare) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SignatureShare.Merge(m, src)
}
func (m *SignatureShare) XXX_Size() int {
	return m.Size()
}
func (m *SignatureShare) XXX_DiscardUnknown() {
	xxx_messageInfo_SignatureShare.DiscardUnknown(m)
}

var xxx_messageInfo_SignatureShare proto.InternalMessageInfo

func (m *SignatureShare) GetScalar() []byte {
	if m != nil {
		return m.Scalar
	}
	return nil
}

func init() {
	proto.RegisterType((*VerifiableSecretSharingCommitment)(nil), "penumbra.crypto.decaf377_frost.v1.VerifiableSecretSharingCommitment")
	proto.RegisterType((*DKGRound1Package)(nil), "penumbra.crypto.decaf377_frost.v1.DKGRound1Package")
	proto.RegisterType((*SigningShare)(nil), "penumbra.crypto.decaf377_frost.v1.SigningShare")
	proto.RegisterType((*DKGRound2Package)(nil), "penumbra.crypto.decaf377_frost.v1.DKGRound2Package")
	proto.RegisterType((*NonceCommitment)(nil), "penumbra.crypto.decaf377_frost.v1.NonceCommitment")
	proto.RegisterType((*SigningCommitments)(nil), "penumbra.crypto.decaf377_frost.v1.SigningCommitments")
	proto.RegisterType((*SignatureShare)(nil), "penumbra.crypto.decaf377_frost.v1.SignatureShare")
}

func init() {
	proto.RegisterFile("penumbra/crypto/decaf377_frost/v1/decaf377_frost.proto", fileDescriptor_b4822bfeb2663db2)
}

var fileDescriptor_b4822bfeb2663db2 = []byte{
	// 520 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x9c, 0x94, 0x4f, 0x8b, 0xd3, 0x40,
	0x18, 0xc6, 0x77, 0xb2, 0xd0, 0x95, 0xb1, 0xea, 0x32, 0x07, 0x29, 0x1e, 0x42, 0x37, 0xa2, 0x14,
	0x5c, 0x13, 0xd2, 0x05, 0x57, 0xe2, 0x41, 0x68, 0x8b, 0x0b, 0xae, 0x7f, 0x42, 0x2a, 0x3d, 0x48,
	0xa1, 0x4c, 0x27, 0x6f, 0xd2, 0x71, 0x9b, 0x99, 0x32, 0x99, 0x56, 0xfc, 0x16, 0x7e, 0x06, 0x0f,
	0x1e, 0x3c, 0xf8, 0x39, 0xc4, 0xd3, 0x1e, 0x3d, 0x4a, 0x7b, 0xf3, 0x53, 0x48, 0xda, 0x64, 0xfb,
	0xe7, 0x60, 0x16, 0x4f, 0xed, 0xfb, 0xe6, 0x79, 0x9e, 0xf9, 0xbd, 0x2f, 0x99, 0xe0, 0x27, 0x13,
	0x10, 0xd3, 0x64, 0xa8, 0xa8, 0xc3, 0xd4, 0xa7, 0x89, 0x96, 0x4e, 0x08, 0x8c, 0x46, 0x27, 0xa7,
	0xa7, 0x83, 0x48, 0xc9, 0x54, 0x3b, 0x33, 0x77, 0xa7, 0x63, 0x4f, 0x94, 0xd4, 0x92, 0x1c, 0x15,
	0x3e, 0x7b, 0xe5, 0xb3, 0x77, 0x54, 0x33, 0xd7, 0x7a, 0x8e, 0x8f, 0x7a, 0xa0, 0x78, 0xc4, 0xe9,
	0x70, 0x0c, 0x5d, 0x60, 0x0a, 0x74, 0x77, 0x44, 0x15, 0x17, 0x71, 0x5b, 0x26, 0x09, 0xd7, 0x09,
	0x08, 0x4d, 0xee, 0xe1, 0x1b, 0x30, 0x86, 0xec, 0x6f, 0x5a, 0x43, 0xf5, 0xfd, 0x46, 0x35, 0xb8,
	0xaa, 0xad, 0xaf, 0x08, 0x1f, 0x76, 0xce, 0xcf, 0x02, 0x39, 0x15, 0xa1, 0xeb, 0x53, 0x76, 0x41,
	0x63, 0x20, 0x21, 0xc6, 0xec, 0xca, 0x5e, 0x43, 0x75, 0xd4, 0xb8, 0xd9, 0xec, 0xd8, 0xa5, 0x34,
	0x76, 0x29, 0x4a, 0xb0, 0x91, 0x4b, 0x8e, 0x31, 0x99, 0x28, 0x29, 0xa3, 0x81, 0x8c, 0x06, 0x17,
	0x42, 0x7e, 0x1c, 0x43, 0x18, 0x43, 0xcd, 0xa8, 0xa3, 0x46, 0x35, 0x38, 0x5c, 0x3e, 0x79, 0x1b,
	0x9d, 0x17, 0x7d, 0xeb, 0x21, 0xae, 0x76, 0x79, 0x2c, 0xb8, 0x88, 0xb3, 0x54, 0x20, 0x77, 0x71,
	0x25, 0x65, 0x74, 0x4c, 0xd5, 0x92, 0xaf, 0x1a, 0xe4, 0x95, 0x35, 0x5a, 0xcf, 0xd3, 0x2c, 0xe6,
	0x79, 0x87, 0x6f, 0xa5, 0x2b, 0xef, 0x20, 0xcd, 0xcc, 0xf9, 0x48, 0xce, 0x35, 0x46, 0xda, 0x3c,
	0x33, 0xa8, 0xa6, 0x1b, 0x95, 0xf5, 0x08, 0xdf, 0x79, 0x23, 0x05, 0x83, 0x8d, 0x4d, 0xd7, 0xf0,
	0x41, 0xbe, 0xd9, 0x9c, 0xaa, 0x28, 0xad, 0xef, 0x08, 0x93, 0x3c, 0x6b, 0xad, 0x4f, 0xc9, 0x4b,
	0x5c, 0x19, 0xf1, 0x90, 0x8b, 0x38, 0x47, 0x6a, 0x5e, 0x03, 0x69, 0xe7, 0xd0, 0x20, 0x4f, 0x20,
	0xaf, 0xf0, 0xc1, 0x90, 0x8b, 0x65, 0x98, 0xf1, 0xdf, 0x61, 0x45, 0x84, 0xd5, 0xc0, 0xb7, 0x33,
	0x5e, 0xaa, 0xa7, 0x0a, 0xfe, 0xb9, 0xf1, 0xd6, 0xdc, 0xf8, 0x31, 0x37, 0xd1, 0xe5, 0xdc, 0x44,
	0xbf, 0xe7, 0x26, 0xfa, 0xbc, 0x30, 0xf7, 0x2e, 0x17, 0xe6, 0xde, 0xaf, 0x85, 0xb9, 0x87, 0x1f,
	0x30, 0x99, 0x94, 0x43, 0xb4, 0x48, 0x27, 0x6f, 0xbd, 0xc8, 0x3a, 0x7e, 0xf6, 0xf2, 0xfb, 0xe8,
	0xfd, 0x87, 0x98, 0xeb, 0xd1, 0x74, 0x68, 0x33, 0x99, 0x38, 0xa9, 0x56, 0x54, 0xc4, 0x30, 0x96,
	0x33, 0x78, 0x3c, 0x03, 0x91, 0x41, 0xa5, 0x0e, 0x17, 0x1a, 0x14, 0x1b, 0xd1, 0xec, 0x37, 0xbb,
	0x46, 0x4f, 0x9d, 0x65, 0xe1, 0x94, 0x5e, 0xb7, 0x67, 0xdb, 0x9d, 0x99, 0xfb, 0xc5, 0xd8, 0xf7,
	0xdb, 0x9d, 0x6f, 0x46, 0xdd, 0x2f, 0x58, 0xdb, 0x2b, 0xd6, 0x2d, 0x30, 0xbb, 0xe7, 0xfe, 0x5c,
	0x4b, 0xfa, 0x2b, 0x49, 0x7f, 0x4b, 0xd2, 0xef, 0xb9, 0x73, 0xe3, 0xb8, 0x4c, 0xd2, 0x3f, 0xf3,
	0x5b, 0xaf, 0x41, 0xd3, 0x90, 0x6a, 0xfa, 0xc7, 0xb8, 0x5f, 0xc8, 0x3d, 0x6f, 0xa5, 0xf7, 0xbc,
	0x2d, 0x83, 0xe7, 0xf5, 0xdc, 0x61, 0x65, 0xf9, 0x49, 0x38, 0xf9, 0x1b, 0x00, 0x00, 0xff, 0xff,
	0x39, 0x20, 0x1b, 0x74, 0x4c, 0x04, 0x00, 0x00,
}

func (m *VerifiableSecretSharingCommitment) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *VerifiableSecretSharingCommitment) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *VerifiableSecretSharingCommitment) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Elements) > 0 {
		for iNdEx := len(m.Elements) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.Elements[iNdEx])
			copy(dAtA[i:], m.Elements[iNdEx])
			i = encodeVarintDecaf377Frost(dAtA, i, uint64(len(m.Elements[iNdEx])))
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func (m *DKGRound1Package) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *DKGRound1Package) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *DKGRound1Package) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.ProofOfKnowledge) > 0 {
		i -= len(m.ProofOfKnowledge)
		copy(dAtA[i:], m.ProofOfKnowledge)
		i = encodeVarintDecaf377Frost(dAtA, i, uint64(len(m.ProofOfKnowledge)))
		i--
		dAtA[i] = 0x12
	}
	if m.Commitment != nil {
		{
			size, err := m.Commitment.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintDecaf377Frost(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *SigningShare) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *SigningShare) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *SigningShare) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Scalar) > 0 {
		i -= len(m.Scalar)
		copy(dAtA[i:], m.Scalar)
		i = encodeVarintDecaf377Frost(dAtA, i, uint64(len(m.Scalar)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *DKGRound2Package) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *DKGRound2Package) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *DKGRound2Package) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.SigningShare != nil {
		{
			size, err := m.SigningShare.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintDecaf377Frost(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *NonceCommitment) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *NonceCommitment) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *NonceCommitment) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Element) > 0 {
		i -= len(m.Element)
		copy(dAtA[i:], m.Element)
		i = encodeVarintDecaf377Frost(dAtA, i, uint64(len(m.Element)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *SigningCommitments) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *SigningCommitments) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *SigningCommitments) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Binding != nil {
		{
			size, err := m.Binding.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintDecaf377Frost(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x12
	}
	if m.Hiding != nil {
		{
			size, err := m.Hiding.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintDecaf377Frost(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *SignatureShare) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *SignatureShare) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *SignatureShare) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Scalar) > 0 {
		i -= len(m.Scalar)
		copy(dAtA[i:], m.Scalar)
		i = encodeVarintDecaf377Frost(dAtA, i, uint64(len(m.Scalar)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func encodeVarintDecaf377Frost(dAtA []byte, offset int, v uint64) int {
	offset -= sovDecaf377Frost(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *VerifiableSecretSharingCommitment) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.Elements) > 0 {
		for _, b := range m.Elements {
			l = len(b)
			n += 1 + l + sovDecaf377Frost(uint64(l))
		}
	}
	return n
}

func (m *DKGRound1Package) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Commitment != nil {
		l = m.Commitment.Size()
		n += 1 + l + sovDecaf377Frost(uint64(l))
	}
	l = len(m.ProofOfKnowledge)
	if l > 0 {
		n += 1 + l + sovDecaf377Frost(uint64(l))
	}
	return n
}

func (m *SigningShare) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Scalar)
	if l > 0 {
		n += 1 + l + sovDecaf377Frost(uint64(l))
	}
	return n
}

func (m *DKGRound2Package) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.SigningShare != nil {
		l = m.SigningShare.Size()
		n += 1 + l + sovDecaf377Frost(uint64(l))
	}
	return n
}

func (m *NonceCommitment) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Element)
	if l > 0 {
		n += 1 + l + sovDecaf377Frost(uint64(l))
	}
	return n
}

func (m *SigningCommitments) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Hiding != nil {
		l = m.Hiding.Size()
		n += 1 + l + sovDecaf377Frost(uint64(l))
	}
	if m.Binding != nil {
		l = m.Binding.Size()
		n += 1 + l + sovDecaf377Frost(uint64(l))
	}
	return n
}

func (m *SignatureShare) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Scalar)
	if l > 0 {
		n += 1 + l + sovDecaf377Frost(uint64(l))
	}
	return n
}

func sovDecaf377Frost(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozDecaf377Frost(x uint64) (n int) {
	return sovDecaf377Frost(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *VerifiableSecretSharingCommitment) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowDecaf377Frost
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
			return fmt.Errorf("proto: VerifiableSecretSharingCommitment: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: VerifiableSecretSharingCommitment: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Elements", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDecaf377Frost
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthDecaf377Frost
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthDecaf377Frost
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Elements = append(m.Elements, make([]byte, postIndex-iNdEx))
			copy(m.Elements[len(m.Elements)-1], dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipDecaf377Frost(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthDecaf377Frost
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *DKGRound1Package) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowDecaf377Frost
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
			return fmt.Errorf("proto: DKGRound1Package: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: DKGRound1Package: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Commitment", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDecaf377Frost
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
				return ErrInvalidLengthDecaf377Frost
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthDecaf377Frost
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Commitment == nil {
				m.Commitment = &VerifiableSecretSharingCommitment{}
			}
			if err := m.Commitment.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ProofOfKnowledge", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDecaf377Frost
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthDecaf377Frost
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthDecaf377Frost
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.ProofOfKnowledge = append(m.ProofOfKnowledge[:0], dAtA[iNdEx:postIndex]...)
			if m.ProofOfKnowledge == nil {
				m.ProofOfKnowledge = []byte{}
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipDecaf377Frost(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthDecaf377Frost
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *SigningShare) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowDecaf377Frost
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
			return fmt.Errorf("proto: SigningShare: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: SigningShare: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Scalar", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDecaf377Frost
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthDecaf377Frost
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthDecaf377Frost
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Scalar = append(m.Scalar[:0], dAtA[iNdEx:postIndex]...)
			if m.Scalar == nil {
				m.Scalar = []byte{}
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipDecaf377Frost(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthDecaf377Frost
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *DKGRound2Package) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowDecaf377Frost
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
			return fmt.Errorf("proto: DKGRound2Package: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: DKGRound2Package: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field SigningShare", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDecaf377Frost
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
				return ErrInvalidLengthDecaf377Frost
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthDecaf377Frost
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.SigningShare == nil {
				m.SigningShare = &SigningShare{}
			}
			if err := m.SigningShare.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipDecaf377Frost(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthDecaf377Frost
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *NonceCommitment) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowDecaf377Frost
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
			return fmt.Errorf("proto: NonceCommitment: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: NonceCommitment: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Element", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDecaf377Frost
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthDecaf377Frost
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthDecaf377Frost
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Element = append(m.Element[:0], dAtA[iNdEx:postIndex]...)
			if m.Element == nil {
				m.Element = []byte{}
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipDecaf377Frost(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthDecaf377Frost
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *SigningCommitments) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowDecaf377Frost
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
			return fmt.Errorf("proto: SigningCommitments: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: SigningCommitments: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Hiding", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDecaf377Frost
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
				return ErrInvalidLengthDecaf377Frost
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthDecaf377Frost
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Hiding == nil {
				m.Hiding = &NonceCommitment{}
			}
			if err := m.Hiding.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Binding", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDecaf377Frost
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
				return ErrInvalidLengthDecaf377Frost
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthDecaf377Frost
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Binding == nil {
				m.Binding = &NonceCommitment{}
			}
			if err := m.Binding.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipDecaf377Frost(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthDecaf377Frost
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *SignatureShare) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowDecaf377Frost
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
			return fmt.Errorf("proto: SignatureShare: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: SignatureShare: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Scalar", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowDecaf377Frost
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthDecaf377Frost
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthDecaf377Frost
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Scalar = append(m.Scalar[:0], dAtA[iNdEx:postIndex]...)
			if m.Scalar == nil {
				m.Scalar = []byte{}
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipDecaf377Frost(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthDecaf377Frost
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipDecaf377Frost(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowDecaf377Frost
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
					return 0, ErrIntOverflowDecaf377Frost
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowDecaf377Frost
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
				return 0, ErrInvalidLengthDecaf377Frost
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupDecaf377Frost
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthDecaf377Frost
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthDecaf377Frost        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowDecaf377Frost          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupDecaf377Frost = fmt.Errorf("proto: unexpected end of group")
)
