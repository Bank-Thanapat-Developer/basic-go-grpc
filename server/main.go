package main

import (
	"context"
	pb "grpc-server/grpc-hello-world/tutorialpb"
	"log"
	"net"

	"google.golang.org/grpc"
)

const (
	port = ":50051"
)

// server คือตัว implement service ฝั่ง backend
// ฝัง (embed) UnimplementedGreeterServiceServer ไว้เพื่อ:
//  1. ทำให้ struct เรา "satisfy" interface GreeterServiceServer ที่ protoc สร้างมา
//     (ทุก method ที่ยังไม่ได้ implement จะ default คืน Unimplemented error)
//  2. รองรับ forward-compatibility — ถ้าวันหลังเพิ่ม RPC ใหม่ใน .proto
//     โค้ดเดิมจะยังคอมไพล์ผ่าน (method ใหม่ใช้ Unimplemented แทน)
type server struct {
	pb.UnimplementedGreeterServiceServer
}

// SayHello คือ business logic ของ RPC ที่ประกาศไว้ใน helloworld.proto
//
//	service GreeterService {
//	  rpc SayHello(HelloRequest) returns (HelloResponse);
//	}
//
// arguments:
//
//	ctx — context พา deadline/cancel/metadata ของ request นี้
//	req — request object ที่ client ส่งมา (unmarshal จาก binary ให้แล้ว)
//
// returns:
//
//	*pb.HelloResponse — ผลลัพธ์ที่จะ marshal กลับไปหา client
//	error             — ถ้าไม่ใช่ nil จะถูกแปลงเป็น gRPC status code ส่งกลับ
func (s *server) SayHello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloResponse, error) {
	// log ชื่อที่ client ส่งมาเพื่อ debug
	log.Printf("Received request: %v", req.GetName())

	// สร้าง response แล้วคืนกลับ — gRPC จะ marshal เป็น Protobuf binary ให้เอง
	return &pb.HelloResponse{Message: "Hello, " + req.GetName()}, nil
}

func main() {
	//  1. เปิด TCP socket ที่ port 50051 รอรับการเชื่อมต่อ
	//     net.Listen คืน net.Listener — เป็น low-level transport ที่ gRPC จะมาใช้ต่อ
	//     ถ้า port ถูกใช้อยู่ หรือไม่มีสิทธิ์ bind จะ error ตรงนี้
	lis, err := net.Listen("tcp", port)
	if err != nil {
		// Fatalf = log แล้ว os.Exit(1) ทันที — เหมาะกับ startup error ที่กู้ไม่ได้
		log.Fatalf("failed to listen: %v", err)
	}

	//  2. ใช้ defer ปิด listener เมื่อ main() จบ
	//    (กรณีนี้ grpcServer.Serve จะ block ทำให้ defer ทำงานเฉพาะตอน server หยุด)
	defer lis.Close()

	//  3. สร้าง gRPC server เปล่าๆ (ยังไม่มี service ลงทะเบียน)
	//     ตรงนี้สามารถใส่ option เพิ่มได้ เช่น:
	//     grpc.NewServer(
	//     grpc.UnaryInterceptor(authInterceptor),   // middleware
	//     grpc.Creds(tlsCredentials),               // เปิด TLS
	//     grpc.MaxRecvMsgSize(10 * 1024 * 1024),    // จำกัด size
	//     )
	grpcServer := grpc.NewServer()

	//  4. ผูก implementation (&server{}) เข้ากับ grpcServer
	//     RegisterGreeterServiceServer เป็นฟังก์ชันที่ protoc-gen-go-grpc generate มาให้
	//     ภายในจะ map RPC name "SayHello" → s.SayHello() ของเรา
	//     ถ้ามีหลาย service สามารถ Register ต่อกันได้หลายครั้งบน grpcServer ตัวเดียวกัน
	pb.RegisterGreeterServiceServer(grpcServer, &server{})

	// 5) เริ่มรับ request — Serve() จะ block goroutine นี้ไว้ตลอด
	//    มันจะวน accept connection ใหม่ แล้วแยก goroutine ไป handle ต่อ
	//    คืน error เฉพาะตอนเกิดปัญหา fatal (เช่น listener ถูก close ผิดปกติ)
	log.Printf("server listening at %v", lis.Addr())
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
