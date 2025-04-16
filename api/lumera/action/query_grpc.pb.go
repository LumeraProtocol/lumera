// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             (unknown)
// source: lumera/action/query.proto

package action

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	Query_Params_FullMethodName                   = "/lumera.action.Query/Params"
	Query_GetAction_FullMethodName                = "/lumera.action.Query/GetAction"
	Query_GetActionFee_FullMethodName             = "/lumera.action.Query/GetActionFee"
	Query_ListActions_FullMethodName              = "/lumera.action.Query/ListActions"
	Query_ListActionsBySuperNode_FullMethodName   = "/lumera.action.Query/ListActionsBySuperNode"
	Query_ListActionsByBlockHeight_FullMethodName = "/lumera.action.Query/ListActionsByBlockHeight"
	Query_ListExpiredActions_FullMethodName       = "/lumera.action.Query/ListExpiredActions"
	Query_QueryActionByMetadata_FullMethodName    = "/lumera.action.Query/QueryActionByMetadata"
)

// QueryClient is the client API for Query service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// Query defines the gRPC querier service.
type QueryClient interface {
	// Parameters queries the parameters of the module.
	Params(ctx context.Context, in *QueryParamsRequest, opts ...grpc.CallOption) (*QueryParamsResponse, error)
	// GetAction queries a single action by ID.
	GetAction(ctx context.Context, in *QueryGetActionRequest, opts ...grpc.CallOption) (*QueryGetActionResponse, error)
	// Queries a list of GetActionFee items.
	GetActionFee(ctx context.Context, in *QueryGetActionFeeRequest, opts ...grpc.CallOption) (*QueryGetActionFeeResponse, error)
	// List actions with optional type and state filters.
	ListActions(ctx context.Context, in *QueryListActionsRequest, opts ...grpc.CallOption) (*QueryListActionsResponse, error)
	// List actions for a specific supernode.
	ListActionsBySuperNode(ctx context.Context, in *QueryListActionsBySuperNodeRequest, opts ...grpc.CallOption) (*QueryListActionsResponse, error)
	// List actions created at a specific block height.
	ListActionsByBlockHeight(ctx context.Context, in *QueryListActionsByBlockHeightRequest, opts ...grpc.CallOption) (*QueryListActionsResponse, error)
	// List expired actions.
	ListExpiredActions(ctx context.Context, in *QueryListExpiredActionsRequest, opts ...grpc.CallOption) (*QueryListActionsResponse, error)
	// Query actions based on metadata.
	QueryActionByMetadata(ctx context.Context, in *QueryActionByMetadataRequest, opts ...grpc.CallOption) (*QueryListActionsResponse, error)
}

type queryClient struct {
	cc grpc.ClientConnInterface
}

func NewQueryClient(cc grpc.ClientConnInterface) QueryClient {
	return &queryClient{cc}
}

func (c *queryClient) Params(ctx context.Context, in *QueryParamsRequest, opts ...grpc.CallOption) (*QueryParamsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(QueryParamsResponse)
	err := c.cc.Invoke(ctx, Query_Params_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *queryClient) GetAction(ctx context.Context, in *QueryGetActionRequest, opts ...grpc.CallOption) (*QueryGetActionResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(QueryGetActionResponse)
	err := c.cc.Invoke(ctx, Query_GetAction_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *queryClient) GetActionFee(ctx context.Context, in *QueryGetActionFeeRequest, opts ...grpc.CallOption) (*QueryGetActionFeeResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(QueryGetActionFeeResponse)
	err := c.cc.Invoke(ctx, Query_GetActionFee_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *queryClient) ListActions(ctx context.Context, in *QueryListActionsRequest, opts ...grpc.CallOption) (*QueryListActionsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(QueryListActionsResponse)
	err := c.cc.Invoke(ctx, Query_ListActions_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *queryClient) ListActionsBySuperNode(ctx context.Context, in *QueryListActionsBySuperNodeRequest, opts ...grpc.CallOption) (*QueryListActionsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(QueryListActionsResponse)
	err := c.cc.Invoke(ctx, Query_ListActionsBySuperNode_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *queryClient) ListActionsByBlockHeight(ctx context.Context, in *QueryListActionsByBlockHeightRequest, opts ...grpc.CallOption) (*QueryListActionsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(QueryListActionsResponse)
	err := c.cc.Invoke(ctx, Query_ListActionsByBlockHeight_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *queryClient) ListExpiredActions(ctx context.Context, in *QueryListExpiredActionsRequest, opts ...grpc.CallOption) (*QueryListActionsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(QueryListActionsResponse)
	err := c.cc.Invoke(ctx, Query_ListExpiredActions_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *queryClient) QueryActionByMetadata(ctx context.Context, in *QueryActionByMetadataRequest, opts ...grpc.CallOption) (*QueryListActionsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(QueryListActionsResponse)
	err := c.cc.Invoke(ctx, Query_QueryActionByMetadata_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// QueryServer is the server API for Query service.
// All implementations must embed UnimplementedQueryServer
// for forward compatibility.
//
// Query defines the gRPC querier service.
type QueryServer interface {
	// Parameters queries the parameters of the module.
	Params(context.Context, *QueryParamsRequest) (*QueryParamsResponse, error)
	// GetAction queries a single action by ID.
	GetAction(context.Context, *QueryGetActionRequest) (*QueryGetActionResponse, error)
	// Queries a list of GetActionFee items.
	GetActionFee(context.Context, *QueryGetActionFeeRequest) (*QueryGetActionFeeResponse, error)
	// List actions with optional type and state filters.
	ListActions(context.Context, *QueryListActionsRequest) (*QueryListActionsResponse, error)
	// List actions for a specific supernode.
	ListActionsBySuperNode(context.Context, *QueryListActionsBySuperNodeRequest) (*QueryListActionsResponse, error)
	// List actions created at a specific block height.
	ListActionsByBlockHeight(context.Context, *QueryListActionsByBlockHeightRequest) (*QueryListActionsResponse, error)
	// List expired actions.
	ListExpiredActions(context.Context, *QueryListExpiredActionsRequest) (*QueryListActionsResponse, error)
	// Query actions based on metadata.
	QueryActionByMetadata(context.Context, *QueryActionByMetadataRequest) (*QueryListActionsResponse, error)
	mustEmbedUnimplementedQueryServer()
}

// UnimplementedQueryServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedQueryServer struct{}

func (UnimplementedQueryServer) Params(context.Context, *QueryParamsRequest) (*QueryParamsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Params not implemented")
}
func (UnimplementedQueryServer) GetAction(context.Context, *QueryGetActionRequest) (*QueryGetActionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetAction not implemented")
}
func (UnimplementedQueryServer) GetActionFee(context.Context, *QueryGetActionFeeRequest) (*QueryGetActionFeeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetActionFee not implemented")
}
func (UnimplementedQueryServer) ListActions(context.Context, *QueryListActionsRequest) (*QueryListActionsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListActions not implemented")
}
func (UnimplementedQueryServer) ListActionsBySuperNode(context.Context, *QueryListActionsBySuperNodeRequest) (*QueryListActionsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListActionsBySuperNode not implemented")
}
func (UnimplementedQueryServer) ListActionsByBlockHeight(context.Context, *QueryListActionsByBlockHeightRequest) (*QueryListActionsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListActionsByBlockHeight not implemented")
}
func (UnimplementedQueryServer) ListExpiredActions(context.Context, *QueryListExpiredActionsRequest) (*QueryListActionsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListExpiredActions not implemented")
}
func (UnimplementedQueryServer) QueryActionByMetadata(context.Context, *QueryActionByMetadataRequest) (*QueryListActionsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method QueryActionByMetadata not implemented")
}
func (UnimplementedQueryServer) mustEmbedUnimplementedQueryServer() {}
func (UnimplementedQueryServer) testEmbeddedByValue()               {}

// UnsafeQueryServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to QueryServer will
// result in compilation errors.
type UnsafeQueryServer interface {
	mustEmbedUnimplementedQueryServer()
}

func RegisterQueryServer(s grpc.ServiceRegistrar, srv QueryServer) {
	// If the following call pancis, it indicates UnimplementedQueryServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&Query_ServiceDesc, srv)
}

func _Query_Params_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryParamsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).Params(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Query_Params_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).Params(ctx, req.(*QueryParamsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_GetAction_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryGetActionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).GetAction(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Query_GetAction_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).GetAction(ctx, req.(*QueryGetActionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_GetActionFee_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryGetActionFeeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).GetActionFee(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Query_GetActionFee_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).GetActionFee(ctx, req.(*QueryGetActionFeeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_ListActions_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryListActionsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).ListActions(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Query_ListActions_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).ListActions(ctx, req.(*QueryListActionsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_ListActionsBySuperNode_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryListActionsBySuperNodeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).ListActionsBySuperNode(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Query_ListActionsBySuperNode_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).ListActionsBySuperNode(ctx, req.(*QueryListActionsBySuperNodeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_ListActionsByBlockHeight_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryListActionsByBlockHeightRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).ListActionsByBlockHeight(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Query_ListActionsByBlockHeight_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).ListActionsByBlockHeight(ctx, req.(*QueryListActionsByBlockHeightRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_ListExpiredActions_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryListExpiredActionsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).ListExpiredActions(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Query_ListExpiredActions_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).ListExpiredActions(ctx, req.(*QueryListExpiredActionsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Query_QueryActionByMetadata_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryActionByMetadataRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(QueryServer).QueryActionByMetadata(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: Query_QueryActionByMetadata_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(QueryServer).QueryActionByMetadata(ctx, req.(*QueryActionByMetadataRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Query_ServiceDesc is the grpc.ServiceDesc for Query service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Query_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "lumera.action.Query",
	HandlerType: (*QueryServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Params",
			Handler:    _Query_Params_Handler,
		},
		{
			MethodName: "GetAction",
			Handler:    _Query_GetAction_Handler,
		},
		{
			MethodName: "GetActionFee",
			Handler:    _Query_GetActionFee_Handler,
		},
		{
			MethodName: "ListActions",
			Handler:    _Query_ListActions_Handler,
		},
		{
			MethodName: "ListActionsBySuperNode",
			Handler:    _Query_ListActionsBySuperNode_Handler,
		},
		{
			MethodName: "ListActionsByBlockHeight",
			Handler:    _Query_ListActionsByBlockHeight_Handler,
		},
		{
			MethodName: "ListExpiredActions",
			Handler:    _Query_ListExpiredActions_Handler,
		},
		{
			MethodName: "QueryActionByMetadata",
			Handler:    _Query_QueryActionByMetadata_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "lumera/action/query.proto",
}
