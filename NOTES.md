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
│  Server main()                                                   │
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

┌──────────────────────────────────────────────────────────────────┐
│  Client main()                                                   │
│                                                                  │
│  ① context.WithTimeout(ctx, 5s)                                  │
│                          ──▶  สร้าง ctx + deadline 5 วินาที         │
│                               (defer cancel() เพื่อคืน resource)    │
│                                                                  │
│  ② insecure.NewCredentials()                                     │
│                          ──▶  credential แบบไม่เข้ารหัส (dev only) │
│                                                                  │
│  ③ grpc.NewClient(":50051", WithTransportCredentials(cred))      │
│                          ──▶  สร้าง channel (connection) ไป server │
│                                                                  │
│  ④ pb.NewGreeterServiceClient(conn)                              │
│                          ──▶  สร้าง client stub จาก channel        │
│                                                                  │
│  ⑤ c.SayHello(ctx, &pb.HelloRequest{Name: "Bank"})              │
│                          ──▶  เรียก RPC ผ่าน stub                  │
│                                                                  │
│              ภายใต้ stub (ขั้นตอนอัตโนมัติ):                         │
│              marshal req → HTTP/2 → server                       │
│              server SayHello() → reply                           │
│              ← HTTP/2 ← unmarshal resp ← stub                    │
│                                                                  │
│  ⑥ resp.GetMessage()    ──▶  อ่านค่า "Hello, Bank" ออกมา log       │
└──────────────────────────────────────────────────────────────────┘

═══════════════════════════════════════════════════════════════════════
UserService (เพิ่มใหม่) — สาธิต data types หลากหลาย + Rich Errors
═══════════════════════════════════════════════════════════════════════

▍ Data types ที่ใช้ใน user.proto

  | ประเภท                       | ตัวอย่างใน proto                          |
  | ---------------------------- | ----------------------------------------- |
  | scalar (string/int32/bool)   | id, name, age, is_active                  |
  | enum                         | Role { ROLE_UNSPECIFIED, ADMIN, USER, … } |
  | repeated (list)              | repeated string tags                      |
  | map<K, V>                    | map<string, string> metadata              |
  | well-known type              | google.protobuf.Timestamp created_at      |
  | google.protobuf.Empty        | return ของ DeleteUser                     |

▍ Rich Error 2 รูปแบบที่สาธิต

  1. errdetails.BadRequest  → ใช้ตอน validate input
     - แนบ FieldViolations หลายตัวพร้อมกัน (1 RPC = หลาย error)
     - client unpack ด้วย status.Details() แล้ว type-assert เป็น *BadRequest

  2. errdetails.ResourceInfo → ใช้ตอนหา resource ไม่เจอ
     - บอกว่า resource ประเภทอะไร, id อะไร, ทำไมไม่เจอ

▍ ขั้นตอนการเพิ่ม service ใหม่ (template ทำซ้ำได้)

  ① เขียน .proto (server/proto/user.proto + client/proto/user.proto)
  ② run protoc → ได้ user.pb.go + user_grpc.pb.go
  ③ implement xxxServer struct ฝัง UnimplementedXxxServer
  ④ pb.RegisterXxxServiceServer(grpcServer, impl) ใน main()
  ⑤ client ใช้ pb.NewXxxServiceClient(conn) สร้าง stub แล้วเรียก

┌──────────────────────────────────────────────────────────────────┐
│  UserService Server-side Validation Flow                         │
│                                                                  │
│  CreateUser(req)                                                 │
│       │                                                          │
│       ▼                                                          │
│  validateCreateUser(req)                                         │
│       │                                                          │
│       ├── ตรวจ name (required, ≤ 50)                              │
│       ├── ตรวจ email (มี @)                                       │
│       ├── ตรวจ age (0-150)                                       │
│       └── ตรวจ role (≠ UNSPECIFIED)                              │
│                                                                  │
│  ถ้ามี violation:                                                 │
│       status.New(InvalidArgument, "validation failed")           │
│         .WithDetails(&BadRequest{FieldViolations: [...]})        │
│       → return err                                               │
│                                                                  │
│  ถ้าผ่าน:                                                         │
│       เก็บลง sync.Map → return *User                              │
└──────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────┐
│  Client-side Rich Error Unpacking                                │
│                                                                  │
│  err := c.CreateUser(ctx, badReq)                                │
│       │                                                          │
│       ▼                                                          │
│  st, ok := status.FromError(err)                                 │
│       │                                                          │
│       ├── st.Code()    → InvalidArgument                         │
│       ├── st.Message() → "validation failed"                     │
│       └── st.Details() → []proto.Message                         │
│                  │                                               │
│                  ▼ for _, d := range details                     │
│            switch d.(type):                                      │
│              case *BadRequest    → loop FieldViolations          │
│              case *ResourceInfo  → log resource_type/name        │
└──────────────────────────────────────────────────────────────────┘

▍ Commands สำหรับโปรเจกต์นี้

  # generate code (รันใน server/ หรือ client/)
  protoc --go_out=. --go-grpc_out=. ./proto/user.proto

  # รัน server
  cd server && go run .

  # รัน client (สาธิตทั้ง happy path + error cases)
  cd client && go run .
