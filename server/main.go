package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	pb "grpc-server/grpc-hello-world/tutorialpb"

	"github.com/google/uuid"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	port = ":50051"
)

// ─────────────────────────────────────────────────────────────────────
// GreeterService (เดิม)
// ─────────────────────────────────────────────────────────────────────

// greeterServer คือตัว implement GreeterService ฝั่ง backend
// ฝัง UnimplementedGreeterServiceServer ไว้เพื่อ forward-compatibility
type greeterServer struct {
	pb.UnimplementedGreeterServiceServer
}

// SayHello — RPC แบบ unary ตัวอย่างเดิม
// ถ้า name ยาวกว่า 10 อักขระจะ return InvalidArgument
func (s *greeterServer) SayHello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloResponse, error) {
	log.Printf("[Greeter] Received: %v", req.GetName())

	if len(req.GetName()) > 10 {
		return nil, status.Errorf(codes.InvalidArgument, "Name must be less than 10 characters")
	}

	return &pb.HelloResponse{Message: "Hello, " + req.GetName()}, nil
}

// ─────────────────────────────────────────────────────────────────────
// UserService (ใหม่) — สาธิต data types หลากหลาย + rich error
// ─────────────────────────────────────────────────────────────────────

// userServer คือตัว implement UserService
// เก็บ user ใน memory ผ่าน sync.Map เพราะ gRPC handler ถูกเรียกแบบ concurrent
// (gRPC สร้าง goroutine ใหม่ทุก request → ต้อง thread-safe)
type userServer struct {
	pb.UnimplementedUserServiceServer

	mu    sync.RWMutex      // ปกป้อง field "users" จาก race condition
	users map[string]*pb.User // in-memory store: id → user
}

// newUserServer สร้าง userServer พร้อม in-memory storage ที่พร้อมใช้งาน
func newUserServer() *userServer {
	return &userServer{
		users: make(map[string]*pb.User),
	}
}

// CreateUser — สร้าง user ใหม่
//
// Flow:
//  1. validate input → ถ้าผิด return InvalidArgument + BadRequest details
//     (ทำให้ client เห็นว่า field ไหน error และเพราะอะไร)
//  2. สร้าง user (id auto-generate, created_at = now)
//  3. เก็บลง memory
func (s *userServer) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.User, error) {
	log.Printf("[User] CreateUser: name=%q email=%q", req.GetName(), req.GetEmail())

	// เรียก validator แยก function → handler อ่านง่าย + test ง่าย
	if err := validateCreateUser(req); err != nil {
		return nil, err
	}

	now := time.Now()
	user := &pb.User{
		Id:        uuid.NewString(),
		Name:      req.GetName(),
		Email:     req.GetEmail(),
		Age:       req.GetAge(),
		IsActive:  true,
		Role:      req.GetRole(),
		Tags:      req.GetTags(),
		Metadata:  req.GetMetadata(),
		CreatedAt: timestamppb.New(now),
	}

	s.mu.Lock()
	s.users[user.Id] = user
	s.mu.Unlock()

	return user, nil
}

// validateCreateUser ตรวจ field ทั้งหมดของ request
// ถ้าเจอข้อผิดพลาด รวบทุก field violation ใส่ใน BadRequest แล้ว return ครั้งเดียว
// (UX ดีกว่า return error ทีละ field เพราะ client ไม่ต้อง retry หลายรอบ)
func validateCreateUser(req *pb.CreateUserRequest) error {
	violations := []*errdetails.BadRequest_FieldViolation{}

	if strings.TrimSpace(req.GetName()) == "" {
		violations = append(violations, &errdetails.BadRequest_FieldViolation{
			Field:       "name",
			Description: "name is required",
		})
	} else if len(req.GetName()) > 50 {
		violations = append(violations, &errdetails.BadRequest_FieldViolation{
			Field:       "name",
			Description: "name must be at most 50 characters",
		})
	}

	if req.GetEmail() == "" || !strings.Contains(req.GetEmail(), "@") {
		violations = append(violations, &errdetails.BadRequest_FieldViolation{
			Field:       "email",
			Description: "email must be a valid email address",
		})
	}

	if req.GetAge() < 0 || req.GetAge() > 150 {
		violations = append(violations, &errdetails.BadRequest_FieldViolation{
			Field:       "age",
			Description: "age must be between 0 and 150",
		})
	}

	if req.GetRole() == pb.Role_ROLE_UNSPECIFIED {
		violations = append(violations, &errdetails.BadRequest_FieldViolation{
			Field:       "role",
			Description: "role must be specified (ROLE_ADMIN, ROLE_USER, or ROLE_GUEST)",
		})
	}

	if len(violations) == 0 {
		return nil
	}

	// แนบ BadRequest details เข้าไปใน status
	// ฝั่ง client จะ unpack ด้วย status.Details() → loop หา *errdetails.BadRequest
	st := status.New(codes.InvalidArgument, "validation failed")
	st, withErr := st.WithDetails(&errdetails.BadRequest{FieldViolations: violations})
	if withErr != nil {
		// ถ้า marshal details ไม่ได้ (rare) → fallback เป็น error ธรรมดา
		// ห่อด้วย %w เพื่อรักษา error chain (R4)
		return fmt.Errorf("failed to attach error details: %w", withErr)
	}
	return st.Err()
}

// GetUser — ดึง user ตาม id
// ถ้าไม่เจอ return NotFound พร้อม ResourceInfo (rich error อีกแบบ)
func (s *userServer) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
	log.Printf("[User] GetUser: id=%q", req.GetId())

	if req.GetId() == "" {
		st := status.New(codes.InvalidArgument, "id is required")
		st, _ = st.WithDetails(&errdetails.BadRequest{
			FieldViolations: []*errdetails.BadRequest_FieldViolation{
				{Field: "id", Description: "id must not be empty"},
			},
		})
		return nil, st.Err()
	}

	s.mu.RLock()
	user, ok := s.users[req.GetId()]
	s.mu.RUnlock()

	if !ok {
		st := status.New(codes.NotFound, fmt.Sprintf("user %q not found", req.GetId()))
		// ResourceInfo บอก client ว่า resource ประเภทอะไรที่หาไม่เจอ
		st, _ = st.WithDetails(&errdetails.ResourceInfo{
			ResourceType: "User",
			ResourceName: req.GetId(),
			Description:  "user does not exist in storage",
		})
		return nil, st.Err()
	}

	return user, nil
}

// ListUsers — คืนรายการ user
// ถ้า role_filter != ROLE_UNSPECIFIED จะ filter เฉพาะ role นั้น
func (s *userServer) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	log.Printf("[User] ListUsers: filter=%v", req.GetRoleFilter())

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]*pb.User, 0, len(s.users))
	for _, u := range s.users {
		if req.GetRoleFilter() != pb.Role_ROLE_UNSPECIFIED && u.GetRole() != req.GetRoleFilter() {
			continue
		}
		out = append(out, u)
	}

	return &pb.ListUsersResponse{
		Users: out,
		Total: int32(len(out)),
	}, nil
}

// DeleteUser — ลบ user ตาม id
// คืน google.protobuf.Empty (วิธีปกติเวลา response ไม่มี data)
func (s *userServer) DeleteUser(ctx context.Context, req *pb.DeleteUserRequest) (*emptypb.Empty, error) {
	log.Printf("[User] DeleteUser: id=%q", req.GetId())

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[req.GetId()]; !ok {
		return nil, status.Errorf(codes.NotFound, "user %q not found", req.GetId())
	}
	delete(s.users, req.GetId())
	return &emptypb.Empty{}, nil
}

// ─────────────────────────────────────────────────────────────────────
// main — bootstrap server
// ─────────────────────────────────────────────────────────────────────

func main() {
	// 1. เปิด TCP socket
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	// 2. สร้าง gRPC server (ในอนาคตใส่ interceptor ตรงนี้ได้)
	grpcServer := grpc.NewServer()

	// 3. ลงทะเบียน 2 services บน server เดียวกัน
	//    ทั้งสอง service ใช้ port + connection เดียวกัน
	//    client จะรู้ว่าเรียก service ไหนจาก method full name ของ HTTP/2 path
	pb.RegisterGreeterServiceServer(grpcServer, &greeterServer{})
	pb.RegisterUserServiceServer(grpcServer, newUserServer())

	// 4. เริ่มรับ request — block จนกว่า server จะหยุด
	log.Printf("server listening at %v (services: Greeter, User)", lis.Addr())
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
