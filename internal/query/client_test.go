package query

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ydb-platform/ydb-go-genproto/Ydb_Query_V1"
	"github.com/ydb-platform/ydb-go-genproto/protos/Ydb"
	"github.com/ydb-platform/ydb-go-genproto/protos/Ydb_Operations"
	"github.com/ydb-platform/ydb-go-genproto/protos/Ydb_Query"
	"github.com/ydb-platform/ydb-go-genproto/protos/Ydb_TableStats"
	"go.uber.org/mock/gomock"
	grpcCodes "google.golang.org/grpc/codes"
	grpcStatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/ydb-platform/ydb-go-sdk/v3/internal/pool"
	"github.com/ydb-platform/ydb-go-sdk/v3/internal/query/config"
	"github.com/ydb-platform/ydb-go-sdk/v3/internal/query/options"
	"github.com/ydb-platform/ydb-go-sdk/v3/internal/xerrors"
	"github.com/ydb-platform/ydb-go-sdk/v3/internal/xtest"
	"github.com/ydb-platform/ydb-go-sdk/v3/query"
	"github.com/ydb-platform/ydb-go-sdk/v3/trace"
)

func TestClient(t *testing.T) {
	ctx := xtest.Context(t)
	t.Run("CreateSession", func(t *testing.T) {
		t.Run("HappyWay", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			attachStream := NewMockQueryService_AttachSessionClient(ctrl)
			attachStream.EXPECT().Recv().Return(&Ydb_Query.SessionState{
				Status: Ydb.StatusIds_SUCCESS,
			}, nil).AnyTimes()
			service := NewMockQueryServiceClient(ctrl)
			service.EXPECT().CreateSession(gomock.Any(), gomock.Any()).Return(&Ydb_Query.CreateSessionResponse{
				Status:    Ydb.StatusIds_SUCCESS,
				SessionId: "test",
			}, nil)
			service.EXPECT().AttachSession(gomock.Any(), gomock.Any()).Return(attachStream, nil)
			service.EXPECT().DeleteSession(gomock.Any(), gomock.Any()).Return(&Ydb_Query.DeleteSessionResponse{
				Status: Ydb.StatusIds_SUCCESS,
			}, nil)
			attached := 0
			s, err := createSession(ctx, service, config.New(config.WithTrace(
				&trace.Query{
					OnSessionAttach: func(info trace.QuerySessionAttachStartInfo) func(info trace.QuerySessionAttachDoneInfo) {
						return func(info trace.QuerySessionAttachDoneInfo) {
							if info.Error == nil {
								attached++
							}
						}
					},
					OnSessionDelete: func(info trace.QuerySessionDeleteStartInfo) func(info trace.QuerySessionDeleteDoneInfo) {
						attached--

						return nil
					},
				},
			)))
			require.NoError(t, err)
			require.EqualValues(t, "test", s.id)
			require.EqualValues(t, 1, attached)
			err = s.Close(ctx)
			require.NoError(t, err)
			require.EqualValues(t, 0, attached)
		})
		t.Run("TransportError", func(t *testing.T) {
			t.Run("OnCall", func(t *testing.T) {
				ctrl := gomock.NewController(t)
				service := NewMockQueryServiceClient(ctrl)
				service.EXPECT().CreateSession(gomock.Any(), gomock.Any()).Return(nil, grpcStatus.Error(grpcCodes.Unavailable, ""))
				_, err := createSession(ctx, service, config.New())
				require.Error(t, err)
				require.True(t, xerrors.IsTransportError(err, grpcCodes.Unavailable))
			})
			t.Run("OnAttach", func(t *testing.T) {
				ctrl := gomock.NewController(t)
				service := NewMockQueryServiceClient(ctrl)
				service.EXPECT().CreateSession(gomock.Any(), gomock.Any()).Return(&Ydb_Query.CreateSessionResponse{
					Status:    Ydb.StatusIds_SUCCESS,
					SessionId: "test",
				}, nil)
				service.EXPECT().AttachSession(gomock.Any(), gomock.Any()).Return(nil, grpcStatus.Error(grpcCodes.Unavailable, ""))
				service.EXPECT().DeleteSession(gomock.Any(), gomock.Any()).Return(nil, grpcStatus.Error(grpcCodes.Unavailable, ""))
				_, err := createSession(ctx, service, config.New())
				require.Error(t, err)
				require.True(t, xerrors.IsTransportError(err, grpcCodes.Unavailable))
			})
			t.Run("OnRecv", func(t *testing.T) {
				ctrl := gomock.NewController(t)
				attachStream := NewMockQueryService_AttachSessionClient(ctrl)
				attachStream.EXPECT().Recv().Return(nil, grpcStatus.Error(grpcCodes.Unavailable, "")).AnyTimes()
				service := NewMockQueryServiceClient(ctrl)
				service.EXPECT().CreateSession(gomock.Any(), gomock.Any()).Return(&Ydb_Query.CreateSessionResponse{
					Status:    Ydb.StatusIds_SUCCESS,
					SessionId: "test",
				}, nil)
				service.EXPECT().AttachSession(gomock.Any(), gomock.Any()).Return(attachStream, nil)
				service.EXPECT().DeleteSession(gomock.Any(), gomock.Any()).Return(&Ydb_Query.DeleteSessionResponse{
					Status: Ydb.StatusIds_SUCCESS,
				}, nil)
				_, err := createSession(ctx, service, config.New())
				require.Error(t, err)
				require.True(t, xerrors.IsTransportError(err, grpcCodes.Unavailable))
			})
		})
		t.Run("OperationError", func(t *testing.T) {
			t.Run("OnCall", func(t *testing.T) {
				ctrl := gomock.NewController(t)
				service := NewMockQueryServiceClient(ctrl)
				service.EXPECT().CreateSession(gomock.Any(), gomock.Any()).Return(nil,
					xerrors.Operation(xerrors.WithStatusCode(Ydb.StatusIds_UNAVAILABLE)),
				)
				_, err := createSession(ctx, service, config.New())
				require.Error(t, err)
				require.True(t, xerrors.IsOperationError(err, Ydb.StatusIds_UNAVAILABLE))
			})
			t.Run("OnRecv", func(t *testing.T) {
				ctrl := gomock.NewController(t)
				attachStream := NewMockQueryService_AttachSessionClient(ctrl)
				attachStream.EXPECT().Recv().Return(nil,
					xerrors.Operation(xerrors.WithStatusCode(Ydb.StatusIds_UNAVAILABLE)),
				)
				service := NewMockQueryServiceClient(ctrl)
				service.EXPECT().CreateSession(gomock.Any(), gomock.Any()).Return(&Ydb_Query.CreateSessionResponse{
					Status:    Ydb.StatusIds_SUCCESS,
					SessionId: "test",
				}, nil)
				service.EXPECT().AttachSession(gomock.Any(), gomock.Any()).Return(attachStream, nil)
				service.EXPECT().DeleteSession(gomock.Any(), gomock.Any()).Return(&Ydb_Query.DeleteSessionResponse{
					Status: Ydb.StatusIds_SUCCESS,
				}, nil)
				_, err := createSession(ctx, service, config.New())
				require.Error(t, err)
				require.True(t, xerrors.IsOperationError(err, Ydb.StatusIds_UNAVAILABLE))
			})
		})
	})
	t.Run("Do", func(t *testing.T) {
		t.Run("HappyWay", func(t *testing.T) {
			var visited bool
			err := do(ctx, testPool(ctx, func(ctx context.Context) (*Session, error) {
				return newTestSession("123"), nil
			}), func(ctx context.Context, s *Session) error {
				visited = true

				return nil
			})
			require.NoError(t, err)
			require.True(t, visited)
		})
		t.Run("RetryableError", func(t *testing.T) {
			counter := 0
			err := do(ctx, testPool(ctx, func(ctx context.Context) (*Session, error) {
				return newTestSession("123"), nil
			}), func(ctx context.Context, s *Session) error {
				counter++
				if counter < 10 {
					return xerrors.Retryable(errors.New(""))
				}

				return nil
			})
			require.NoError(t, err)
			require.Equal(t, 10, counter)
		})
	})
	t.Run("DoTx", func(t *testing.T) {
		t.Run("HappyWay", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			client := NewMockQueryServiceClient(ctrl)
			stream := NewMockQueryService_ExecuteQueryClient(ctrl)
			stream.EXPECT().Recv().Return(&Ydb_Query.ExecuteQueryResponsePart{
				Status: Ydb.StatusIds_SUCCESS,
				TxMeta: &Ydb_Query.TransactionMeta{
					Id: "456",
				},
				ResultSetIndex: 0,
				ResultSet: &Ydb.ResultSet{
					Columns: []*Ydb.Column{
						{
							Name: "a",
							Type: &Ydb.Type{
								Type: &Ydb.Type_TypeId{
									TypeId: Ydb.Type_UINT64,
								},
							},
						},
						{
							Name: "b",
							Type: &Ydb.Type{
								Type: &Ydb.Type_TypeId{
									TypeId: Ydb.Type_UTF8,
								},
							},
						},
					},
					Rows: []*Ydb.Value{
						{
							Items: []*Ydb.Value{{
								Value: &Ydb.Value_Uint64Value{
									Uint64Value: 1,
								},
							}, {
								Value: &Ydb.Value_TextValue{
									TextValue: "1",
								},
							}},
						},
						{
							Items: []*Ydb.Value{{
								Value: &Ydb.Value_Uint64Value{
									Uint64Value: 2,
								},
							}, {
								Value: &Ydb.Value_TextValue{
									TextValue: "2",
								},
							}},
						},
						{
							Items: []*Ydb.Value{{
								Value: &Ydb.Value_Uint64Value{
									Uint64Value: 3,
								},
							}, {
								Value: &Ydb.Value_TextValue{
									TextValue: "3",
								},
							}},
						},
					},
				},
			}, nil)
			stream.EXPECT().Recv().Return(nil, io.EOF)
			client.EXPECT().ExecuteQuery(gomock.Any(), gomock.Any()).Return(stream, nil)
			client.EXPECT().CommitTransaction(gomock.Any(), gomock.Any()).Return(&Ydb_Query.CommitTransactionResponse{
				Status: Ydb.StatusIds_SUCCESS,
			}, nil)
			err := doTx(ctx, testPool(ctx, func(ctx context.Context) (*Session, error) {
				return newTestSessionWithClient("123", client), nil
			}), func(ctx context.Context, tx query.TxActor) error {
				defer func() {
					require.Equal(t, "456", tx.ID())
				}()

				return tx.Exec(ctx, "")
			}, &trace.Query{})
			require.NoError(t, err)
		})
		t.Run("RetryableError", func(t *testing.T) {
			counter := 0
			ctrl := gomock.NewController(t)
			client := NewMockQueryServiceClient(ctrl)
			client.EXPECT().BeginTransaction(gomock.Any(), gomock.Any()).Return(&Ydb_Query.BeginTransactionResponse{
				Status: Ydb.StatusIds_SUCCESS,
			}, nil).AnyTimes()
			client.EXPECT().RollbackTransaction(gomock.Any(), gomock.Any()).Return(&Ydb_Query.RollbackTransactionResponse{
				Status: Ydb.StatusIds_SUCCESS,
			}, nil).AnyTimes()
			client.EXPECT().CommitTransaction(gomock.Any(), gomock.Any()).Return(&Ydb_Query.CommitTransactionResponse{
				Status: Ydb.StatusIds_SUCCESS,
			}, nil).AnyTimes()
			err := doTx(ctx, testPool(ctx, func(ctx context.Context) (*Session, error) {
				return newTestSessionWithClient("123", client), nil
			}), func(ctx context.Context, tx query.TxActor) error {
				counter++
				if counter < 10 {
					return xerrors.Retryable(errors.New(""))
				}

				return nil
			}, &trace.Query{})
			require.NoError(t, err)
			require.Equal(t, 10, counter)
		})
	})
	t.Run("Exec", func(t *testing.T) {
		t.Run("HappyWay", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			err := clientExec(ctx, testPool(ctx, func(ctx context.Context) (*Session, error) {
				stream := NewMockQueryService_ExecuteQueryClient(ctrl)
				stream.EXPECT().Recv().Return(&Ydb_Query.ExecuteQueryResponsePart{
					Status: Ydb.StatusIds_SUCCESS,
					TxMeta: &Ydb_Query.TransactionMeta{
						Id: "456",
					},
					ResultSetIndex: 0,
					ResultSet: &Ydb.ResultSet{
						Columns: []*Ydb.Column{
							{
								Name: "a",
								Type: &Ydb.Type{
									Type: &Ydb.Type_TypeId{
										TypeId: Ydb.Type_UINT64,
									},
								},
							},
							{
								Name: "b",
								Type: &Ydb.Type{
									Type: &Ydb.Type_TypeId{
										TypeId: Ydb.Type_UTF8,
									},
								},
							},
						},
						Rows: []*Ydb.Value{
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 1,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "1",
									},
								}},
							},
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 2,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "2",
									},
								}},
							},
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 3,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "3",
									},
								}},
							},
						},
					},
				}, nil)
				stream.EXPECT().Recv().Return(&Ydb_Query.ExecuteQueryResponsePart{
					Status:         Ydb.StatusIds_SUCCESS,
					ResultSetIndex: 0,
					ResultSet: &Ydb.ResultSet{
						Rows: []*Ydb.Value{
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 4,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "4",
									},
								}},
							},
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 5,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "5",
									},
								}},
							},
						},
					},
				}, nil)
				stream.EXPECT().Recv().Return(&Ydb_Query.ExecuteQueryResponsePart{
					Status:         Ydb.StatusIds_SUCCESS,
					ResultSetIndex: 1,
					ResultSet: &Ydb.ResultSet{
						Columns: []*Ydb.Column{
							{
								Name: "c",
								Type: &Ydb.Type{
									Type: &Ydb.Type_TypeId{
										TypeId: Ydb.Type_UINT64,
									},
								},
							},
							{
								Name: "d",
								Type: &Ydb.Type{
									Type: &Ydb.Type_TypeId{
										TypeId: Ydb.Type_UTF8,
									},
								},
							},
							{
								Name: "e",
								Type: &Ydb.Type{
									Type: &Ydb.Type_TypeId{
										TypeId: Ydb.Type_BOOL,
									},
								},
							},
						},
						Rows: []*Ydb.Value{
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 1,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "1",
									},
								}, {
									Value: &Ydb.Value_BoolValue{
										BoolValue: true,
									},
								}},
							},
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 2,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "2",
									},
								}, {
									Value: &Ydb.Value_BoolValue{
										BoolValue: false,
									},
								}},
							},
						},
					},
				}, nil)
				stream.EXPECT().Recv().Return(nil, io.EOF)
				client := NewMockQueryServiceClient(ctrl)
				client.EXPECT().ExecuteQuery(gomock.Any(), gomock.Any()).Return(stream, nil)

				return newTestSessionWithClient("123", client), nil
			}), "")
			require.NoError(t, err)
		})
	})
	t.Run("Query", func(t *testing.T) {
		t.Run("HappyWay", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			r, err := clientQuery(ctx, testPool(ctx, func(ctx context.Context) (*Session, error) {
				stream := NewMockQueryService_ExecuteQueryClient(ctrl)
				stream.EXPECT().Recv().Return(&Ydb_Query.ExecuteQueryResponsePart{
					Status: Ydb.StatusIds_SUCCESS,
					TxMeta: &Ydb_Query.TransactionMeta{
						Id: "456",
					},
					ResultSetIndex: 0,
					ResultSet: &Ydb.ResultSet{
						Columns: []*Ydb.Column{
							{
								Name: "a",
								Type: &Ydb.Type{
									Type: &Ydb.Type_TypeId{
										TypeId: Ydb.Type_UINT64,
									},
								},
							},
							{
								Name: "b",
								Type: &Ydb.Type{
									Type: &Ydb.Type_TypeId{
										TypeId: Ydb.Type_UTF8,
									},
								},
							},
						},
						Rows: []*Ydb.Value{
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 1,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "1",
									},
								}},
							},
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 2,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "2",
									},
								}},
							},
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 3,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "3",
									},
								}},
							},
						},
					},
				}, nil)
				stream.EXPECT().Recv().Return(&Ydb_Query.ExecuteQueryResponsePart{
					Status:         Ydb.StatusIds_SUCCESS,
					ResultSetIndex: 0,
					ResultSet: &Ydb.ResultSet{
						Rows: []*Ydb.Value{
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 4,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "4",
									},
								}},
							},
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 5,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "5",
									},
								}},
							},
						},
					},
				}, nil)
				stream.EXPECT().Recv().Return(&Ydb_Query.ExecuteQueryResponsePart{
					Status:         Ydb.StatusIds_SUCCESS,
					ResultSetIndex: 1,
					ResultSet: &Ydb.ResultSet{
						Columns: []*Ydb.Column{
							{
								Name: "c",
								Type: &Ydb.Type{
									Type: &Ydb.Type_TypeId{
										TypeId: Ydb.Type_UINT64,
									},
								},
							},
							{
								Name: "d",
								Type: &Ydb.Type{
									Type: &Ydb.Type_TypeId{
										TypeId: Ydb.Type_UTF8,
									},
								},
							},
							{
								Name: "e",
								Type: &Ydb.Type{
									Type: &Ydb.Type_TypeId{
										TypeId: Ydb.Type_BOOL,
									},
								},
							},
						},
						Rows: []*Ydb.Value{
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 1,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "1",
									},
								}, {
									Value: &Ydb.Value_BoolValue{
										BoolValue: true,
									},
								}},
							},
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 2,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "2",
									},
								}, {
									Value: &Ydb.Value_BoolValue{
										BoolValue: false,
									},
								}},
							},
						},
					},
				}, nil)
				stream.EXPECT().Recv().Return(nil, io.EOF)
				client := NewMockQueryServiceClient(ctrl)
				client.EXPECT().ExecuteQuery(gomock.Any(), gomock.Any()).Return(stream, nil)

				return newTestSessionWithClient("123", client), nil
			}), "")
			require.NoError(t, err)
			{
				rs, err := r.NextResultSet(ctx)
				require.NoError(t, err)
				r1, err := rs.NextRow(ctx)
				require.NoError(t, err)
				var (
					a uint64
					b string
				)
				err = r1.Scan(&a, &b)
				require.NoError(t, err)
				require.EqualValues(t, 1, a)
				require.EqualValues(t, "1", b)
				r2, err := rs.NextRow(ctx)
				require.NoError(t, err)
				err = r2.Scan(&a, &b)
				require.NoError(t, err)
				require.EqualValues(t, 2, a)
				require.EqualValues(t, "2", b)
				r3, err := rs.NextRow(ctx)
				require.NoError(t, err)
				err = r3.Scan(&a, &b)
				require.NoError(t, err)
				require.EqualValues(t, 3, a)
				require.EqualValues(t, "3", b)
				r4, err := rs.NextRow(ctx)
				require.NoError(t, err)
				err = r4.Scan(&a, &b)
				require.NoError(t, err)
				require.EqualValues(t, 4, a)
				require.EqualValues(t, "4", b)
				r5, err := rs.NextRow(ctx)
				require.NoError(t, err)
				err = r5.Scan(&a, &b)
				require.NoError(t, err)
				require.EqualValues(t, 5, a)
				require.EqualValues(t, "5", b)
				r6, err := rs.NextRow(ctx)
				require.ErrorIs(t, err, io.EOF)
				require.Nil(t, r6)
			}
			{
				rs, err := r.NextResultSet(ctx)
				require.NoError(t, err)
				r1, err := rs.NextRow(ctx)
				require.NoError(t, err)
				var (
					a uint64
					b string
					c bool
				)
				err = r1.Scan(&a, &b, &c)
				require.NoError(t, err)
				require.EqualValues(t, 1, a)
				require.EqualValues(t, "1", b)
				require.EqualValues(t, true, c)
				r2, err := rs.NextRow(ctx)
				require.NoError(t, err)
				err = r2.Scan(&a, &b, &c)
				require.NoError(t, err)
				require.EqualValues(t, 2, a)
				require.EqualValues(t, "2", b)
				require.EqualValues(t, false, c)
				r3, err := rs.NextRow(ctx)
				require.ErrorIs(t, err, io.EOF)
				require.Nil(t, r3)
			}
		})
	})
	t.Run("QueryResultSet", func(t *testing.T) {
		t.Run("HappyWay", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			rs, err := clientQueryResultSet(ctx, testPool(ctx, func(ctx context.Context) (*Session, error) {
				stream := NewMockQueryService_ExecuteQueryClient(ctrl)
				stream.EXPECT().Recv().Return(&Ydb_Query.ExecuteQueryResponsePart{
					Status: Ydb.StatusIds_SUCCESS,
					TxMeta: &Ydb_Query.TransactionMeta{
						Id: "456",
					},
					ResultSetIndex: 0,
					ResultSet: &Ydb.ResultSet{
						Columns: []*Ydb.Column{
							{
								Name: "a",
								Type: &Ydb.Type{
									Type: &Ydb.Type_TypeId{
										TypeId: Ydb.Type_UINT64,
									},
								},
							},
							{
								Name: "b",
								Type: &Ydb.Type{
									Type: &Ydb.Type_TypeId{
										TypeId: Ydb.Type_UTF8,
									},
								},
							},
						},
						Rows: []*Ydb.Value{
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 1,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "1",
									},
								}},
							},
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 2,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "2",
									},
								}},
							},
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 3,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "3",
									},
								}},
							},
						},
					},
				}, nil)
				stream.EXPECT().Recv().Return(&Ydb_Query.ExecuteQueryResponsePart{
					Status:         Ydb.StatusIds_SUCCESS,
					ResultSetIndex: 0,
					ResultSet: &Ydb.ResultSet{
						Rows: []*Ydb.Value{
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 4,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "4",
									},
								}},
							},
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 5,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "5",
									},
								}},
							},
						},
					},
				}, nil)
				stream.EXPECT().Recv().Return(nil, io.EOF)
				client := NewMockQueryServiceClient(ctrl)
				client.EXPECT().ExecuteQuery(gomock.Any(), gomock.Any()).Return(stream, nil)

				return newTestSessionWithClient("123", client), nil
			}), "", options.ExecuteSettings())
			require.NoError(t, err)
			require.NotNil(t, rs)
			{
				require.NoError(t, err)
				r1, err := rs.NextRow(ctx)
				require.NoError(t, err)
				var (
					a uint64
					b string
				)
				err = r1.Scan(&a, &b)
				require.NoError(t, err)
				require.EqualValues(t, 1, a)
				require.EqualValues(t, "1", b)
				r2, err := rs.NextRow(ctx)
				require.NoError(t, err)
				err = r2.Scan(&a, &b)
				require.NoError(t, err)
				require.EqualValues(t, 2, a)
				require.EqualValues(t, "2", b)
				r3, err := rs.NextRow(ctx)
				require.NoError(t, err)
				err = r3.Scan(&a, &b)
				require.NoError(t, err)
				require.EqualValues(t, 3, a)
				require.EqualValues(t, "3", b)
				r4, err := rs.NextRow(ctx)
				require.NoError(t, err)
				err = r4.Scan(&a, &b)
				require.NoError(t, err)
				require.EqualValues(t, 4, a)
				require.EqualValues(t, "4", b)
				r5, err := rs.NextRow(ctx)
				require.NoError(t, err)
				err = r5.Scan(&a, &b)
				require.NoError(t, err)
				require.EqualValues(t, 5, a)
				require.EqualValues(t, "5", b)
				r6, err := rs.NextRow(ctx)
				require.ErrorIs(t, err, io.EOF)
				require.Nil(t, r6)
			}
		})
		t.Run("MoreThanOneResultSet", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			rs, err := clientQueryResultSet(ctx, testPool(ctx, func(ctx context.Context) (*Session, error) {
				stream := NewMockQueryService_ExecuteQueryClient(ctrl)
				stream.EXPECT().Recv().Return(&Ydb_Query.ExecuteQueryResponsePart{
					Status: Ydb.StatusIds_SUCCESS,
					TxMeta: &Ydb_Query.TransactionMeta{
						Id: "456",
					},
					ResultSetIndex: 0,
					ResultSet: &Ydb.ResultSet{
						Columns: []*Ydb.Column{
							{
								Name: "a",
								Type: &Ydb.Type{
									Type: &Ydb.Type_TypeId{
										TypeId: Ydb.Type_UINT64,
									},
								},
							},
							{
								Name: "b",
								Type: &Ydb.Type{
									Type: &Ydb.Type_TypeId{
										TypeId: Ydb.Type_UTF8,
									},
								},
							},
						},
						Rows: []*Ydb.Value{
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 1,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "1",
									},
								}},
							},
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 2,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "2",
									},
								}},
							},
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 3,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "3",
									},
								}},
							},
						},
					},
				}, nil)
				stream.EXPECT().Recv().Return(&Ydb_Query.ExecuteQueryResponsePart{
					Status:         Ydb.StatusIds_SUCCESS,
					ResultSetIndex: 0,
					ResultSet: &Ydb.ResultSet{
						Rows: []*Ydb.Value{
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 4,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "4",
									},
								}},
							},
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 5,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "5",
									},
								}},
							},
						},
					},
				}, nil)
				stream.EXPECT().Recv().Return(&Ydb_Query.ExecuteQueryResponsePart{
					Status:         Ydb.StatusIds_SUCCESS,
					ResultSetIndex: 1,
					ResultSet: &Ydb.ResultSet{
						Columns: []*Ydb.Column{
							{
								Name: "c",
								Type: &Ydb.Type{
									Type: &Ydb.Type_TypeId{
										TypeId: Ydb.Type_UINT64,
									},
								},
							},
							{
								Name: "d",
								Type: &Ydb.Type{
									Type: &Ydb.Type_TypeId{
										TypeId: Ydb.Type_UTF8,
									},
								},
							},
							{
								Name: "e",
								Type: &Ydb.Type{
									Type: &Ydb.Type_TypeId{
										TypeId: Ydb.Type_BOOL,
									},
								},
							},
						},
						Rows: []*Ydb.Value{
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 1,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "1",
									},
								}, {
									Value: &Ydb.Value_BoolValue{
										BoolValue: true,
									},
								}},
							},
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 2,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "2",
									},
								}, {
									Value: &Ydb.Value_BoolValue{
										BoolValue: false,
									},
								}},
							},
						},
					},
				}, nil)
				stream.EXPECT().Recv().Return(nil, io.EOF)
				client := NewMockQueryServiceClient(ctrl)
				client.EXPECT().ExecuteQuery(gomock.Any(), gomock.Any()).Return(stream, nil)

				return newTestSessionWithClient("123", client), nil
			}), "", options.ExecuteSettings())
			require.ErrorIs(t, err, errMoreThanOneResultSet)
			require.Nil(t, rs)
		})
	})
	t.Run("QueryRow", func(t *testing.T) {
		t.Run("HappyWay", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			row, err := clientQueryRow(ctx, testPool(ctx, func(ctx context.Context) (*Session, error) {
				stream := NewMockQueryService_ExecuteQueryClient(ctrl)
				stream.EXPECT().Recv().Return(&Ydb_Query.ExecuteQueryResponsePart{
					Status: Ydb.StatusIds_SUCCESS,
					TxMeta: &Ydb_Query.TransactionMeta{
						Id: "456",
					},
					ResultSetIndex: 0,
					ResultSet: &Ydb.ResultSet{
						Columns: []*Ydb.Column{
							{
								Name: "a",
								Type: &Ydb.Type{
									Type: &Ydb.Type_TypeId{
										TypeId: Ydb.Type_UINT64,
									},
								},
							},
							{
								Name: "b",
								Type: &Ydb.Type{
									Type: &Ydb.Type_TypeId{
										TypeId: Ydb.Type_UTF8,
									},
								},
							},
						},
						Rows: []*Ydb.Value{
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 1,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "1",
									},
								}},
							},
						},
					},
				}, nil)
				stream.EXPECT().Recv().Return(nil, io.EOF)
				client := NewMockQueryServiceClient(ctrl)
				client.EXPECT().ExecuteQuery(gomock.Any(), gomock.Any()).Return(stream, nil)

				return newTestSessionWithClient("123", client), nil
			}), "", options.ExecuteSettings())
			require.NoError(t, err)
			require.NotNil(t, row)
			{
				var (
					a uint64
					b string
				)
				err = row.Scan(&a, &b)
				require.NoError(t, err)
				require.EqualValues(t, 1, a)
				require.EqualValues(t, "1", b)
			}
		})
		t.Run("MoreThanOneRow", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			row, err := clientQueryRow(ctx, testPool(ctx, func(ctx context.Context) (*Session, error) {
				stream := NewMockQueryService_ExecuteQueryClient(ctrl)
				stream.EXPECT().Recv().Return(&Ydb_Query.ExecuteQueryResponsePart{
					Status: Ydb.StatusIds_SUCCESS,
					TxMeta: &Ydb_Query.TransactionMeta{
						Id: "456",
					},
					ResultSetIndex: 0,
					ResultSet: &Ydb.ResultSet{
						Columns: []*Ydb.Column{
							{
								Name: "a",
								Type: &Ydb.Type{
									Type: &Ydb.Type_TypeId{
										TypeId: Ydb.Type_UINT64,
									},
								},
							},
							{
								Name: "b",
								Type: &Ydb.Type{
									Type: &Ydb.Type_TypeId{
										TypeId: Ydb.Type_UTF8,
									},
								},
							},
						},
						Rows: []*Ydb.Value{
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 1,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "1",
									},
								}},
							},
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 2,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "2",
									},
								}},
							},
							{
								Items: []*Ydb.Value{{
									Value: &Ydb.Value_Uint64Value{
										Uint64Value: 3,
									},
								}, {
									Value: &Ydb.Value_TextValue{
										TextValue: "3",
									},
								}},
							},
						},
					},
				}, nil)
				stream.EXPECT().Recv().Return(nil, io.EOF)
				client := NewMockQueryServiceClient(ctrl)
				client.EXPECT().ExecuteQuery(gomock.Any(), gomock.Any()).Return(stream, nil)

				return newTestSessionWithClient("123", client), nil
			}), "", options.ExecuteSettings())
			require.ErrorIs(t, err, errMoreThanOneRow)
			require.Nil(t, row)
		})
	})
}

func newTestSession(id string) *Session {
	return &Session{
		id:         id,
		statusCode: statusIdle,
		cfg:        config.New(),
	}
}

func newTestSessionWithClient(id string, client Ydb_Query_V1.QueryServiceClient) *Session {
	return &Session{
		id:                 id,
		queryServiceClient: client,
		statusCode:         statusIdle,
		cfg:                config.New(),
	}
}

func testPool(
	ctx context.Context,
	createSession func(ctx context.Context) (*Session, error),
) *pool.Pool[*Session, Session] {
	return pool.New[*Session, Session](ctx,
		pool.WithLimit[*Session, Session](1),
		pool.WithCreateItemFunc(createSession),
	)
}

func TestQueryScript(t *testing.T) {
	ctx := xtest.Context(t)
	t.Run("HappyWay", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		service := NewMockQueryServiceClient(ctrl)
		service.EXPECT().ExecuteScript(gomock.Any(), gomock.Any()).Return(&Ydb_Operations.Operation{
			Id:     "123",
			Ready:  true,
			Status: Ydb.StatusIds_SUCCESS,
			Metadata: xtest.Must(anypb.New(&Ydb_Query.ExecuteScriptMetadata{
				ExecutionId: "123",
				ExecStatus:  Ydb_Query.ExecStatus_EXEC_STATUS_STARTING,
				ScriptContent: &Ydb_Query.QueryContent{
					Syntax: Ydb_Query.Syntax_SYNTAX_YQL_V1,
					Text:   "SELECT 1 AS a, 2 AS b",
				},
				ResultSetsMeta: []*Ydb_Query.ResultSetMeta{
					{
						Columns: []*Ydb.Column{
							{
								Name: "a",
								Type: &Ydb.Type{
									Type: &Ydb.Type_TypeId{TypeId: Ydb.Type_INT32},
								},
							},
							{
								Name: "b",
								Type: &Ydb.Type{
									Type: &Ydb.Type_TypeId{TypeId: Ydb.Type_INT32},
								},
							},
						},
					},
				},
				ExecMode: Ydb_Query.ExecMode_EXEC_MODE_EXECUTE,
				ExecStats: &Ydb_TableStats.QueryStats{
					QueryPhases:      nil,
					Compilation:      nil,
					ProcessCpuTimeUs: 0,
					QueryPlan:        "",
					QueryAst:         "",
					TotalDurationUs:  0,
					TotalCpuTimeUs:   0,
				},
			})),
			CostInfo: nil,
		}, nil)
		service.EXPECT().FetchScriptResults(gomock.Any(), gomock.Any()).Return(&Ydb_Query.FetchScriptResultsResponse{
			Status:         Ydb.StatusIds_SUCCESS,
			ResultSetIndex: 0,
			ResultSet: &Ydb.ResultSet{
				Columns: []*Ydb.Column{
					{
						Name: "a",
						Type: &Ydb.Type{
							Type: &Ydb.Type_TypeId{TypeId: Ydb.Type_INT32},
						},
					},
					{
						Name: "b",
						Type: &Ydb.Type{
							Type: &Ydb.Type_TypeId{TypeId: Ydb.Type_INT32},
						},
					},
				},
				Rows: []*Ydb.Value{
					{
						Items: []*Ydb.Value{
							{
								Value: &Ydb.Value_Int32Value{
									Int32Value: 1,
								},
								VariantIndex: 0,
							},
							{
								Value: &Ydb.Value_Int32Value{
									Int32Value: 2,
								},
								VariantIndex: 0,
							},
						},
					},
				},
				Truncated: false,
			},
			NextFetchToken: "456",
		}, nil)
		op, err := executeScript(ctx, service, &Ydb_Query.ExecuteScriptRequest{})
		require.NoError(t, err)
		require.EqualValues(t, "123", op.ID)
		r, err := fetchScriptResults(ctx, service, op.ID)
		require.NoError(t, err)
		require.EqualValues(t, 0, r.ResultSetIndex)
		require.Equal(t, "456", r.NextToken)
		require.NotNil(t, r.ResultSet)
		row, err := r.ResultSet.NextRow(ctx)
		require.NoError(t, err)
		var (
			a int
			b int
		)
		err = row.Scan(&a, &b)
		require.NoError(t, err)
		require.EqualValues(t, 1, a)
		require.EqualValues(t, 2, b)
	})
	t.Run("Error", func(t *testing.T) {
		t.Run("OnExecute", func(t *testing.T) {
		})
		t.Run("OnFetch", func(t *testing.T) {
		})
	})
}
