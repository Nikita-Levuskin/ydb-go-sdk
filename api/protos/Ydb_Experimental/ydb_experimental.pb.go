// Code generated by protoc-gen-go. DO NOT EDIT.
// source: ydb_experimental.proto

package Ydb_Experimental

import (
	Ydb "github.com/yandex-cloud/ydb-go-sdk/v2/api/protos/Ydb"
	Ydb_Issue "github.com/yandex-cloud/ydb-go-sdk/v2/api/protos/Ydb_Issue"
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

type ExecuteStreamQueryRequest_ProfileMode int32

const (
	ExecuteStreamQueryRequest_PROFILE_MODE_UNSPECIFIED ExecuteStreamQueryRequest_ProfileMode = 0
	ExecuteStreamQueryRequest_NONE                     ExecuteStreamQueryRequest_ProfileMode = 1
	ExecuteStreamQueryRequest_BASIC                    ExecuteStreamQueryRequest_ProfileMode = 2
	ExecuteStreamQueryRequest_FULL                     ExecuteStreamQueryRequest_ProfileMode = 3
)

var ExecuteStreamQueryRequest_ProfileMode_name = map[int32]string{
	0: "PROFILE_MODE_UNSPECIFIED",
	1: "NONE",
	2: "BASIC",
	3: "FULL",
}

var ExecuteStreamQueryRequest_ProfileMode_value = map[string]int32{
	"PROFILE_MODE_UNSPECIFIED": 0,
	"NONE":                     1,
	"BASIC":                    2,
	"FULL":                     3,
}

func (x ExecuteStreamQueryRequest_ProfileMode) String() string {
	return proto.EnumName(ExecuteStreamQueryRequest_ProfileMode_name, int32(x))
}

func (ExecuteStreamQueryRequest_ProfileMode) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_ac21a693e2c386a5, []int{0, 0}
}

type ExecuteStreamQueryRequest struct {
	YqlText              string                                `protobuf:"bytes,1,opt,name=yql_text,json=yqlText,proto3" json:"yql_text,omitempty"`
	Parameters           map[string]*Ydb.TypedValue            `protobuf:"bytes,2,rep,name=parameters,proto3" json:"parameters,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	ProfileMode          ExecuteStreamQueryRequest_ProfileMode `protobuf:"varint,3,opt,name=profile_mode,json=profileMode,proto3,enum=Ydb.Experimental.ExecuteStreamQueryRequest_ProfileMode" json:"profile_mode,omitempty"`
	Explain              bool                                  `protobuf:"varint,4,opt,name=explain,proto3" json:"explain,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                              `json:"-"`
	XXX_unrecognized     []byte                                `json:"-"`
	XXX_sizecache        int32                                 `json:"-"`
}

func (m *ExecuteStreamQueryRequest) Reset()         { *m = ExecuteStreamQueryRequest{} }
func (m *ExecuteStreamQueryRequest) String() string { return proto.CompactTextString(m) }
func (*ExecuteStreamQueryRequest) ProtoMessage()    {}
func (*ExecuteStreamQueryRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_ac21a693e2c386a5, []int{0}
}

func (m *ExecuteStreamQueryRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ExecuteStreamQueryRequest.Unmarshal(m, b)
}
func (m *ExecuteStreamQueryRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ExecuteStreamQueryRequest.Marshal(b, m, deterministic)
}
func (m *ExecuteStreamQueryRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ExecuteStreamQueryRequest.Merge(m, src)
}
func (m *ExecuteStreamQueryRequest) XXX_Size() int {
	return xxx_messageInfo_ExecuteStreamQueryRequest.Size(m)
}
func (m *ExecuteStreamQueryRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ExecuteStreamQueryRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ExecuteStreamQueryRequest proto.InternalMessageInfo

func (m *ExecuteStreamQueryRequest) GetYqlText() string {
	if m != nil {
		return m.YqlText
	}
	return ""
}

func (m *ExecuteStreamQueryRequest) GetParameters() map[string]*Ydb.TypedValue {
	if m != nil {
		return m.Parameters
	}
	return nil
}

func (m *ExecuteStreamQueryRequest) GetProfileMode() ExecuteStreamQueryRequest_ProfileMode {
	if m != nil {
		return m.ProfileMode
	}
	return ExecuteStreamQueryRequest_PROFILE_MODE_UNSPECIFIED
}

func (m *ExecuteStreamQueryRequest) GetExplain() bool {
	if m != nil {
		return m.Explain
	}
	return false
}

type ExecuteStreamQueryResponse struct {
	Status               Ydb.StatusIds_StatusCode  `protobuf:"varint,1,opt,name=status,proto3,enum=Ydb.StatusIds_StatusCode" json:"status,omitempty"`
	Issues               []*Ydb_Issue.IssueMessage `protobuf:"bytes,2,rep,name=issues,proto3" json:"issues,omitempty"`
	Result               *ExecuteStreamQueryResult `protobuf:"bytes,3,opt,name=result,proto3" json:"result,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                  `json:"-"`
	XXX_unrecognized     []byte                    `json:"-"`
	XXX_sizecache        int32                     `json:"-"`
}

func (m *ExecuteStreamQueryResponse) Reset()         { *m = ExecuteStreamQueryResponse{} }
func (m *ExecuteStreamQueryResponse) String() string { return proto.CompactTextString(m) }
func (*ExecuteStreamQueryResponse) ProtoMessage()    {}
func (*ExecuteStreamQueryResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_ac21a693e2c386a5, []int{1}
}

func (m *ExecuteStreamQueryResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ExecuteStreamQueryResponse.Unmarshal(m, b)
}
func (m *ExecuteStreamQueryResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ExecuteStreamQueryResponse.Marshal(b, m, deterministic)
}
func (m *ExecuteStreamQueryResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ExecuteStreamQueryResponse.Merge(m, src)
}
func (m *ExecuteStreamQueryResponse) XXX_Size() int {
	return xxx_messageInfo_ExecuteStreamQueryResponse.Size(m)
}
func (m *ExecuteStreamQueryResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_ExecuteStreamQueryResponse.DiscardUnknown(m)
}

var xxx_messageInfo_ExecuteStreamQueryResponse proto.InternalMessageInfo

func (m *ExecuteStreamQueryResponse) GetStatus() Ydb.StatusIds_StatusCode {
	if m != nil {
		return m.Status
	}
	return Ydb.StatusIds_STATUS_CODE_UNSPECIFIED
}

func (m *ExecuteStreamQueryResponse) GetIssues() []*Ydb_Issue.IssueMessage {
	if m != nil {
		return m.Issues
	}
	return nil
}

func (m *ExecuteStreamQueryResponse) GetResult() *ExecuteStreamQueryResult {
	if m != nil {
		return m.Result
	}
	return nil
}

type StreamQueryProgress struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *StreamQueryProgress) Reset()         { *m = StreamQueryProgress{} }
func (m *StreamQueryProgress) String() string { return proto.CompactTextString(m) }
func (*StreamQueryProgress) ProtoMessage()    {}
func (*StreamQueryProgress) Descriptor() ([]byte, []int) {
	return fileDescriptor_ac21a693e2c386a5, []int{2}
}

func (m *StreamQueryProgress) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_StreamQueryProgress.Unmarshal(m, b)
}
func (m *StreamQueryProgress) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_StreamQueryProgress.Marshal(b, m, deterministic)
}
func (m *StreamQueryProgress) XXX_Merge(src proto.Message) {
	xxx_messageInfo_StreamQueryProgress.Merge(m, src)
}
func (m *StreamQueryProgress) XXX_Size() int {
	return xxx_messageInfo_StreamQueryProgress.Size(m)
}
func (m *StreamQueryProgress) XXX_DiscardUnknown() {
	xxx_messageInfo_StreamQueryProgress.DiscardUnknown(m)
}

var xxx_messageInfo_StreamQueryProgress proto.InternalMessageInfo

type ExecuteStreamQueryResult struct {
	// Types that are valid to be assigned to Result:
	//	*ExecuteStreamQueryResult_ResultSet
	//	*ExecuteStreamQueryResult_Profile
	//	*ExecuteStreamQueryResult_Progress
	//	*ExecuteStreamQueryResult_QueryPlan
	Result               isExecuteStreamQueryResult_Result `protobuf_oneof:"result"`
	XXX_NoUnkeyedLiteral struct{}                          `json:"-"`
	XXX_unrecognized     []byte                            `json:"-"`
	XXX_sizecache        int32                             `json:"-"`
}

func (m *ExecuteStreamQueryResult) Reset()         { *m = ExecuteStreamQueryResult{} }
func (m *ExecuteStreamQueryResult) String() string { return proto.CompactTextString(m) }
func (*ExecuteStreamQueryResult) ProtoMessage()    {}
func (*ExecuteStreamQueryResult) Descriptor() ([]byte, []int) {
	return fileDescriptor_ac21a693e2c386a5, []int{3}
}

func (m *ExecuteStreamQueryResult) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ExecuteStreamQueryResult.Unmarshal(m, b)
}
func (m *ExecuteStreamQueryResult) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ExecuteStreamQueryResult.Marshal(b, m, deterministic)
}
func (m *ExecuteStreamQueryResult) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ExecuteStreamQueryResult.Merge(m, src)
}
func (m *ExecuteStreamQueryResult) XXX_Size() int {
	return xxx_messageInfo_ExecuteStreamQueryResult.Size(m)
}
func (m *ExecuteStreamQueryResult) XXX_DiscardUnknown() {
	xxx_messageInfo_ExecuteStreamQueryResult.DiscardUnknown(m)
}

var xxx_messageInfo_ExecuteStreamQueryResult proto.InternalMessageInfo

type isExecuteStreamQueryResult_Result interface {
	isExecuteStreamQueryResult_Result()
}

type ExecuteStreamQueryResult_ResultSet struct {
	ResultSet *Ydb.ResultSet `protobuf:"bytes,1,opt,name=result_set,json=resultSet,proto3,oneof"`
}

type ExecuteStreamQueryResult_Profile struct {
	Profile string `protobuf:"bytes,2,opt,name=profile,proto3,oneof"`
}

type ExecuteStreamQueryResult_Progress struct {
	Progress *StreamQueryProgress `protobuf:"bytes,3,opt,name=progress,proto3,oneof"`
}

type ExecuteStreamQueryResult_QueryPlan struct {
	QueryPlan string `protobuf:"bytes,4,opt,name=query_plan,json=queryPlan,proto3,oneof"`
}

func (*ExecuteStreamQueryResult_ResultSet) isExecuteStreamQueryResult_Result() {}

func (*ExecuteStreamQueryResult_Profile) isExecuteStreamQueryResult_Result() {}

func (*ExecuteStreamQueryResult_Progress) isExecuteStreamQueryResult_Result() {}

func (*ExecuteStreamQueryResult_QueryPlan) isExecuteStreamQueryResult_Result() {}

func (m *ExecuteStreamQueryResult) GetResult() isExecuteStreamQueryResult_Result {
	if m != nil {
		return m.Result
	}
	return nil
}

func (m *ExecuteStreamQueryResult) GetResultSet() *Ydb.ResultSet {
	if x, ok := m.GetResult().(*ExecuteStreamQueryResult_ResultSet); ok {
		return x.ResultSet
	}
	return nil
}

func (m *ExecuteStreamQueryResult) GetProfile() string {
	if x, ok := m.GetResult().(*ExecuteStreamQueryResult_Profile); ok {
		return x.Profile
	}
	return ""
}

func (m *ExecuteStreamQueryResult) GetProgress() *StreamQueryProgress {
	if x, ok := m.GetResult().(*ExecuteStreamQueryResult_Progress); ok {
		return x.Progress
	}
	return nil
}

func (m *ExecuteStreamQueryResult) GetQueryPlan() string {
	if x, ok := m.GetResult().(*ExecuteStreamQueryResult_QueryPlan); ok {
		return x.QueryPlan
	}
	return ""
}

// XXX_OneofWrappers is for the internal use of the proto package.
func (*ExecuteStreamQueryResult) XXX_OneofWrappers() []interface{} {
	return []interface{}{
		(*ExecuteStreamQueryResult_ResultSet)(nil),
		(*ExecuteStreamQueryResult_Profile)(nil),
		(*ExecuteStreamQueryResult_Progress)(nil),
		(*ExecuteStreamQueryResult_QueryPlan)(nil),
	}
}

func init() {
	proto.RegisterEnum("Ydb.Experimental.ExecuteStreamQueryRequest_ProfileMode", ExecuteStreamQueryRequest_ProfileMode_name, ExecuteStreamQueryRequest_ProfileMode_value)
	proto.RegisterType((*ExecuteStreamQueryRequest)(nil), "Ydb.Experimental.ExecuteStreamQueryRequest")
	proto.RegisterMapType((map[string]*Ydb.TypedValue)(nil), "Ydb.Experimental.ExecuteStreamQueryRequest.ParametersEntry")
	proto.RegisterType((*ExecuteStreamQueryResponse)(nil), "Ydb.Experimental.ExecuteStreamQueryResponse")
	proto.RegisterType((*StreamQueryProgress)(nil), "Ydb.Experimental.StreamQueryProgress")
	proto.RegisterType((*ExecuteStreamQueryResult)(nil), "Ydb.Experimental.ExecuteStreamQueryResult")
}

func init() { proto.RegisterFile("ydb_experimental.proto", fileDescriptor_ac21a693e2c386a5) }

var fileDescriptor_ac21a693e2c386a5 = []byte{
	// 585 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x94, 0x54, 0xd1, 0x6e, 0xd3, 0x3c,
	0x14, 0x6e, 0xda, 0xad, 0x6b, 0x4f, 0x7f, 0x6d, 0x95, 0x7f, 0x01, 0x59, 0x41, 0xa2, 0xaa, 0x34,
	0xa9, 0xe2, 0x22, 0x81, 0x82, 0x04, 0x82, 0x2b, 0xda, 0x65, 0x5a, 0xd1, 0xd6, 0x15, 0x77, 0x43,
	0x02, 0x2e, 0x22, 0xb7, 0x39, 0x4c, 0x51, 0x93, 0xc6, 0xb5, 0x1d, 0x94, 0x3c, 0x10, 0x6f, 0xc2,
	0x3b, 0xf0, 0x2a, 0x5c, 0xa2, 0x38, 0xe9, 0x88, 0xc6, 0x36, 0xc1, 0x4d, 0x64, 0x7f, 0x3e, 0xdf,
	0x77, 0xbe, 0xe3, 0x73, 0x1c, 0xb8, 0x9f, 0x7a, 0x73, 0x17, 0x13, 0x8e, 0xc2, 0x0f, 0x71, 0xa5,
	0x58, 0x60, 0x71, 0x11, 0xa9, 0x88, 0xb4, 0x3f, 0x7a, 0x73, 0xcb, 0x29, 0xe1, 0x9d, 0xa7, 0x4b,
	0x7f, 0xe9, 0x87, 0xc2, 0xe6, 0xf1, 0x3c, 0xf0, 0x17, 0x36, 0xe3, 0xbe, 0xad, 0x43, 0xa5, 0x9d,
	0x49, 0xf8, 0x52, 0xc6, 0xe8, 0x86, 0x28, 0x25, 0xbb, 0xc4, 0x5c, 0xa3, 0x63, 0xdf, 0xc9, 0x90,
	0x8a, 0xa9, 0x58, 0xba, 0x8b, 0xc8, 0x43, 0x59, 0x10, 0xfa, 0x77, 0x12, 0xbe, 0xb2, 0x20, 0x2e,
	0xa4, 0x7b, 0xdf, 0x6a, 0xb0, 0xef, 0x24, 0xb8, 0x88, 0x15, 0xce, 0x94, 0x40, 0x16, 0xbe, 0x8f,
	0x51, 0xa4, 0x14, 0xd7, 0x31, 0x4a, 0x45, 0xf6, 0xa1, 0x91, 0xae, 0x03, 0x57, 0x61, 0xa2, 0x4c,
	0xa3, 0x6b, 0xf4, 0x9b, 0x74, 0x27, 0x5d, 0x07, 0xe7, 0x98, 0x28, 0xf2, 0x19, 0x80, 0x33, 0xc1,
	0x42, 0x54, 0x28, 0xa4, 0x59, 0xed, 0xd6, 0xfa, 0xad, 0xc1, 0x1b, 0xeb, 0x7a, 0xb1, 0xd6, 0xad,
	0xda, 0xd6, 0xf4, 0x8a, 0xed, 0xac, 0x94, 0x48, 0x69, 0x49, 0x8e, 0x7c, 0x82, 0xff, 0xb8, 0x88,
	0xbe, 0xf8, 0x01, 0xba, 0x61, 0xe4, 0xa1, 0x59, 0xeb, 0x1a, 0xfd, 0xdd, 0xc1, 0xcb, 0x7f, 0x92,
	0xcf, 0xf9, 0xa7, 0x91, 0x87, 0xb4, 0xc5, 0x7f, 0x6f, 0x88, 0x09, 0x3b, 0x98, 0xf0, 0x80, 0xf9,
	0x2b, 0x73, 0xab, 0x6b, 0xf4, 0x1b, 0x74, 0xb3, 0xed, 0x4c, 0x60, 0xef, 0x9a, 0x29, 0xd2, 0x86,
	0xda, 0x12, 0xd3, 0xa2, 0xf6, 0x6c, 0x49, 0x0e, 0x60, 0x5b, 0xdf, 0x9f, 0x59, 0xed, 0x1a, 0xfd,
	0xd6, 0x60, 0x4f, 0x7b, 0x3a, 0x4f, 0x39, 0x7a, 0x1f, 0x32, 0x98, 0xe6, 0xa7, 0xaf, 0xab, 0xaf,
	0x8c, 0xde, 0x3b, 0x68, 0x95, 0x5c, 0x90, 0x47, 0x60, 0x4e, 0xe9, 0xd9, 0xd1, 0xf8, 0xc4, 0x71,
	0x4f, 0xcf, 0x0e, 0x1d, 0xf7, 0x62, 0x32, 0x9b, 0x3a, 0xa3, 0xf1, 0xd1, 0xd8, 0x39, 0x6c, 0x57,
	0x48, 0x03, 0xb6, 0x26, 0x67, 0x13, 0xa7, 0x6d, 0x90, 0x26, 0x6c, 0x0f, 0xdf, 0xce, 0xc6, 0xa3,
	0x76, 0x35, 0x03, 0x8f, 0x2e, 0x4e, 0x4e, 0xda, 0xb5, 0xde, 0x77, 0x03, 0x3a, 0x37, 0x15, 0x2b,
	0x79, 0xb4, 0x92, 0x48, 0x9e, 0x41, 0x3d, 0x1f, 0x03, 0x6d, 0x75, 0x77, 0xb0, 0xaf, 0x6d, 0xcd,
	0x34, 0x34, 0xf6, 0x64, 0xb1, 0x1a, 0x65, 0x97, 0x51, 0x04, 0x12, 0x1b, 0xea, 0x7a, 0xd6, 0x36,
	0xcd, 0x7b, 0xa0, 0x29, 0xe3, 0x0c, 0xca, 0xbf, 0xa7, 0xf9, 0x0c, 0xd2, 0x22, 0x8c, 0x0c, 0xa1,
	0x2e, 0x50, 0xc6, 0x81, 0xd2, 0xed, 0x68, 0x0d, 0x9e, 0xfc, 0x5d, 0x3b, 0x32, 0x06, 0x2d, 0x98,
	0xbd, 0x7b, 0xf0, 0x7f, 0xe9, 0x70, 0x2a, 0xa2, 0x4b, 0x81, 0x52, 0xf6, 0x7e, 0x18, 0x60, 0xde,
	0xc6, 0x25, 0x36, 0x40, 0xce, 0x76, 0x25, 0xe6, 0x63, 0xd8, 0x1a, 0xec, 0xea, 0xdc, 0x79, 0xc0,
	0x0c, 0xd5, 0x71, 0x85, 0x36, 0xc5, 0x66, 0x43, 0x3a, 0xb0, 0x53, 0x34, 0x5c, 0x37, 0xa9, 0x79,
	0x5c, 0xa1, 0x1b, 0x80, 0x8c, 0xa0, 0xc1, 0x8b, 0xac, 0x45, 0x19, 0x07, 0x7f, 0x96, 0x71, 0x83,
	0xc5, 0xe3, 0x0a, 0xbd, 0x22, 0x92, 0xc7, 0x00, 0xeb, 0xec, 0xd0, 0xe5, 0x01, 0xcb, 0xa7, 0x28,
	0xcb, 0xd1, 0xd4, 0xd8, 0x34, 0x60, 0xab, 0x61, 0x63, 0x73, 0x55, 0xc3, 0x17, 0xf0, 0x70, 0x11,
	0x85, 0x56, 0xca, 0x56, 0x1e, 0x26, 0x56, 0xea, 0xcd, 0xad, 0xf2, 0x3f, 0x62, 0x48, 0xca, 0x79,
	0xa7, 0xfa, 0x89, 0xfe, 0x34, 0x8c, 0x79, 0x5d, 0x3f, 0xce, 0xe7, 0xbf, 0x02, 0x00, 0x00, 0xff,
	0xff, 0xd3, 0xc0, 0xd0, 0xbe, 0x55, 0x04, 0x00, 0x00,
}