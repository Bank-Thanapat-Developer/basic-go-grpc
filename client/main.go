package main

import (
	"context"
	"log"
	"time"

	pb "grpc-client/grpc-hello-world/tutorialpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	address = ":50051"
)

func main() {
	// 1 สร้าง context พร้อม deadline 5 วินาที
	//    ถ้า RPC ไม่เสร็จภายในเวลานี้ gRPC จะ cancel ให้อัตโนมัติ
	//    และคืน error เป็น status code DeadlineExceeded
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel() // คืน resource ของ context ตอน main() จบ

	// 2 สร้าง credential แบบ "ไม่เข้ารหัส" (plain TCP)
	//    เหมาะกับช่วง dev เท่านั้น — production ควรใช้ TLS
	//    เช่น credentials.NewTLS(...) หรือ credentials.NewClientTLSFromFile(...)
	cred := insecure.NewCredentials()

	// 3 สร้าง channel (connection) ไปยัง server
	//    grpc.NewClient ไม่ได้เปิด TCP ทันที — มันจะ lazy connect
	//    ตอนที่เรียก RPC ครั้งแรก channel จะจัดการ pool / reconnect ให้เอง
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(cred))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close() // ปิด connection ตอน main() จบ ป้องกัน resource leak

	// 4 สร้าง client stub จาก channel
	//    stub คือโค้ดที่ protoc-gen-go-grpc สร้างให้ ทำให้เรียก RPC ดูเหมือน
	//    เรียกฟังก์ชันปกติ ภายในจะ marshal/unmarshal Protobuf ให้อัตโนมัติ
	c := pb.NewGreeterServiceClient(conn)

	// 5 เรียก RPC SayHello ผ่าน stub
	//    ภายใต้ฉาก: marshal req → ส่งผ่าน HTTP/2 → server ประมวลผล
	//               → ส่ง response กลับ → unmarshal → ได้ resp object
	resp, err := c.SayHello(ctx, &pb.HelloRequest{Name: "Bank"})
	if err != nil {
		log.Fatalf("could not greet: %v", err)
	}

	// 6 อ่านค่าจาก response แล้ว log ออกมา
	//    ใช้ getter (GetMessage) แทนการเข้าถึง field ตรงๆ จะปลอดภัยกว่า
	//    (ถ้า resp เป็น nil getter จะคืน zero value แทน panic)
	log.Printf("Greeting: %s", resp.GetMessage())
}
