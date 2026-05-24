- สร้าง contact ใน floder proto โดย ตั้งชื่อ file_name.proto และใช้ command  `protoc --go_out=. --go-grpc_out=. ./proto/helloworld.proto`

ส่วนที่ 1 : protoc  คือ ตัว compiler หลักของ Protocol Buffers

ส่วนที่ 2 : --go_out=. คือ บอกให้ generate โค้ด message structs ของ Go ออกมาที่ directory . (ปัจจุบัน)
    โดยจะสร้างไฟล์ helloworld.pb.go → struct HelloRequest, HelloResponse + ฟังก์ชัน marshal

ส่วนที่ 3 : --go-grpc_out=. คือ บอกให้ generate โค้ด gRPC service (stub) ของ Go ออกมาที่ directory .
    โดยจะสร้างไฟล์ helloworld_grpc.pb.go → interface GreeterServiceServer, client stub

ส่วนที่ 4 :./proto/helloworld.proto คือ ไฟล์ input — contract ที่จะใช้ generate

server/
└── proto/
    ├── helloworld.proto                 # ไฟล์เดิม (contract)
    ├── helloworld.pb.go                 # ← จาก --go_out (messages)
    └── helloworld_grpc.pb.go            # ← จาก --go-grpc_out (gRPC service)

┌──────────────────────────────────────────────────────────────────┐
│  main()                                                          │
│                                                                  │
│  ① net.Listen(":50051")  ──▶  เปิด TCP socket                    │
│                                                                  │
│  ② grpc.NewServer()      ──▶  สร้าง gRPC server (empty)          │
│                                                                  │
│  ③ RegisterGreeterServiceServer(grpcServer, &server{})          │
│                          ──▶  ผูก SayHello() เข้ากับ grpcServer     │
│                                                                  │
│  ④ grpcServer.Serve(lis) ──▶  เริ่มรับ request (block ตลอด)        │
│                                                                  │
│              เมื่อ client เรียก SayHello:                           │
│              gRPC → unmarshal → s.SayHello() → marshal → reply   │
└──────────────────────────────────────────────────────────────────┘