package main

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "grpc-client/grpc-hello-world/tutorialpb"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const (
	address = ":50051"
)

func main() {
	// 1 สร้าง context พร้อม deadline 10 วินาที (ครอบทุก RPC call)
	//   ถ้า RPC ไม่เสร็จภายในเวลานี้ gRPC จะ cancel ให้อัตโนมัติ
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// 2 สร้าง credential แบบไม่เข้ารหัส (dev เท่านั้น)
	cred := insecure.NewCredentials()

	// 3 สร้าง channel ไปยัง server (lazy connect — ยังไม่ dial)
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(cred))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	// 4 สร้าง stub ของทั้ง 2 services จาก channel เดียวกัน
	//   (ทั้ง 2 service ใช้ TCP connection เดียวกัน — ประหยัด resource)
	greeter := pb.NewGreeterServiceClient(conn)
	users := pb.NewUserServiceClient(conn)

	// 5 รัน demo ทีละ section
	demoGreeter(ctx, greeter)
	demoUserHappyPath(ctx, users)
	demoUserValidationError(ctx, users)
	demoUserNotFound(ctx, users)
}

// ─────────────────────────────────────────────────────────────────────
// Demo 1 — Greeter (เดิม)
// ─────────────────────────────────────────────────────────────────────

func demoGreeter(ctx context.Context, c pb.GreeterServiceClient) {
	log.Println("─── Demo: GreeterService ───")

	resp, err := c.SayHello(ctx, &pb.HelloRequest{Name: "Bank"})
	if err != nil {
		log.Printf("SayHello error: %v", err)
		return
	}
	log.Printf("Greeting: %s", resp.GetMessage())
}

// ─────────────────────────────────────────────────────────────────────
// Demo 2 — UserService happy path
//   สร้าง 2 users แล้วเรียก ListUsers ดูผลลัพธ์
// ─────────────────────────────────────────────────────────────────────

func demoUserHappyPath(ctx context.Context, c pb.UserServiceClient) {
	log.Println("─── Demo: UserService (happy path) ───")

	// สร้าง user คนที่ 1
	u1, err := c.CreateUser(ctx, &pb.CreateUserRequest{
		Name:  "Bank Thanapat",
		Email: "bank@example.com",
		Age:   28,
		Role:  pb.Role_ROLE_ADMIN,
		Tags:  []string{"vip", "beta-tester"},
		Metadata: map[string]string{
			"team":     "platform",
			"location": "bangkok",
		},
	})
	if err != nil {
		log.Printf("CreateUser failed: %v", err)
		return
	}
	log.Printf("Created user: id=%s name=%s role=%s", u1.GetId(), u1.GetName(), u1.GetRole())

	// สร้าง user คนที่ 2
	u2, err := c.CreateUser(ctx, &pb.CreateUserRequest{
		Name:  "Alice",
		Email: "alice@example.com",
		Age:   31,
		Role:  pb.Role_ROLE_USER,
	})
	if err != nil {
		log.Printf("CreateUser failed: %v", err)
		return
	}
	log.Printf("Created user: id=%s name=%s role=%s", u2.GetId(), u2.GetName(), u2.GetRole())

	// list user — ไม่ filter
	all, err := c.ListUsers(ctx, &pb.ListUsersRequest{})
	if err != nil {
		log.Printf("ListUsers failed: %v", err)
		return
	}
	log.Printf("Total users: %d", all.GetTotal())

	// list user — filter เฉพาะ ADMIN
	admins, err := c.ListUsers(ctx, &pb.ListUsersRequest{
		RoleFilter: pb.Role_ROLE_ADMIN,
	})
	if err != nil {
		log.Printf("ListUsers(admin) failed: %v", err)
		return
	}
	log.Printf("Admins only: %d", admins.GetTotal())
}

// ─────────────────────────────────────────────────────────────────────
// Demo 3 — UserService validation error
//   ส่ง request ที่ผิดหลายจุด → server return BadRequest ที่มี
//   FieldViolations หลายรายการ → client unpack ออกมา log แต่ละ field
// ─────────────────────────────────────────────────────────────────────

func demoUserValidationError(ctx context.Context, c pb.UserServiceClient) {
	log.Println("─── Demo: UserService (validation error) ───")

	_, err := c.CreateUser(ctx, &pb.CreateUserRequest{
		Name:  "",            // จะ fail: required
		Email: "not-an-email", // จะ fail: ไม่มี @
		Age:   -5,            // จะ fail: < 0
		Role:  pb.Role_ROLE_UNSPECIFIED, // จะ fail: ไม่ระบุ role
	})
	if err == nil {
		log.Println("⚠ คาดว่าควรจะ error แต่ผ่านไปได้")
		return
	}

	// extract structured error
	printRichError(err)
}

// ─────────────────────────────────────────────────────────────────────
// Demo 4 — UserService not found
//   เรียก GetUser ด้วย id ที่ไม่มีในระบบ → server return NotFound
//   พร้อม ResourceInfo บอกว่า resource ประเภทอะไรหาย
// ─────────────────────────────────────────────────────────────────────

func demoUserNotFound(ctx context.Context, c pb.UserServiceClient) {
	log.Println("─── Demo: UserService (not found) ───")

	_, err := c.GetUser(ctx, &pb.GetUserRequest{Id: "does-not-exist"})
	if err == nil {
		log.Println("⚠ คาดว่าควรจะ error แต่ผ่านไปได้")
		return
	}

	printRichError(err)
}

// printRichError แกะ error จาก gRPC แล้ว log ออกเป็นโครงสร้างที่อ่านง่าย
//   - status.Code()    คือ canonical code (ใช้ตัดสินใจ retry/handle)
//   - status.Message() คือ message สำหรับมนุษย์
//   - status.Details() คือ list ของ rich details (BadRequest, ResourceInfo, ฯลฯ)
func printRichError(err error) {
	// แปลง error → *status.Status — ถ้าไม่ใช่ gRPC error จะได้ Unknown
	st, ok := status.FromError(err)
	if !ok {
		log.Printf("non-grpc error: %v", err)
		return
	}

	log.Printf("✗ code=%s message=%q", st.Code(), st.Message())

	// loop หาทุก detail ที่แนบมา — type-assert เพื่อเช็คว่าเป็นชนิดไหน
	for _, d := range st.Details() {
		switch info := d.(type) {
		case *errdetails.BadRequest:
			for _, fv := range info.GetFieldViolations() {
				log.Printf("  • field=%q: %s", fv.GetField(), fv.GetDescription())
			}
		case *errdetails.ResourceInfo:
			log.Printf("  • resource_type=%q resource_name=%q: %s",
				info.GetResourceType(), info.GetResourceName(), info.GetDescription())
		default:
			log.Printf("  • unknown detail type: %T", info)
		}
	}

	// ตัวอย่างวิธี handle ตาม code — ใช้ใน production จริง
	switch st.Code() {
	case 3: // codes.InvalidArgument
		fmt.Println("    → handle: แสดง form error ให้ user แก้ไข input")
	case 5: // codes.NotFound
		fmt.Println("    → handle: แจ้งผู้ใช้ว่า resource ไม่มีอยู่จริง")
	}
}
