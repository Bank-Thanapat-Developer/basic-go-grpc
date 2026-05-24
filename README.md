# Basic gRPC — คู่มืออธิบายแนวคิดพื้นฐาน

เอกสารนี้อธิบายแนวคิดหลักของ **gRPC** สำหรับผู้เริ่มต้น โดยเน้นคำศัพท์ที่เจอบ่อย เช่น
**Contract, Stub, Protocol Buffers, Channel, Server, Client** พร้อมตัวอย่างประกอบให้
เข้าใจการทำงานแบบ end-to-end

> โครงสร้างโปรเจกต์เริ่มต้น
>
> ```text
> basic-grpc/
> ├── client/   # โค้ดฝั่ง Client (ผู้เรียกใช้บริการ)
> ├── server/   # โค้ดฝั่ง Server (ผู้ให้บริการ)
> └── README.md
> ```

---

## 1. gRPC คืออะไร

**gRPC** (gRPC Remote Procedure Call) คือ framework สำหรับเรียกใช้ฟังก์ชันข้าม
process / ข้าม service ผ่านเครือข่าย โดยพัฒนาโดย Google มีจุดเด่นคือ

- ใช้ **HTTP/2** เป็น transport — รองรับ multiplexing, streaming, header compression
- ใช้ **Protocol Buffers (Protobuf)** เป็นรูปแบบข้อมูล — เล็ก เร็ว และ strongly-typed
- รองรับหลายภาษา (Go, Java, Python, Node.js, C++, C#, ฯลฯ) โดยใช้ contract เดียวกัน
- รองรับ 4 รูปแบบการสื่อสาร: Unary, Server streaming, Client streaming, Bidirectional streaming

ลองเปรียบเทียบกับ REST API:

| หัวข้อ            | REST (JSON over HTTP/1.1) | gRPC (Protobuf over HTTP/2) |
| ----------------- | ------------------------- | --------------------------- |
| รูปแบบข้อมูล      | JSON (text)               | Protobuf (binary)           |
| ขนาด payload      | ใหญ่กว่า                  | เล็กกว่า ~3-10 เท่า         |
| ความเร็ว          | ช้ากว่า                   | เร็วกว่า                    |
| Schema/Contract   | ไม่บังคับ (OpenAPI ช่วย)  | บังคับผ่าน `.proto`         |
| Streaming         | ทำได้ยาก                  | รองรับ native               |
| Browser รองรับ    | รองรับเต็มที่             | ต้องใช้ gRPC-Web            |

---

## 2. Protocol Buffers (Protobuf) และไฟล์ `.proto`

Protobuf คือภาษาสำหรับ **กำหนด schema ของข้อมูลและบริการ** ไฟล์นามสกุล `.proto`
จะถูก compile เป็นโค้ดในภาษาเป้าหมาย (Go, Python, ฯลฯ) เพื่อใช้งานต่อ

ตัวอย่างไฟล์ `greeter.proto`:

```proto
syntax = "proto3";

package greeter;

option go_package = "github.com/example/basic-grpc/proto;greeterpb";

// ข้อความ request
message HelloRequest {
  string name = 1;
}

// ข้อความ response
message HelloReply {
  string message = 1;
}

// นิยาม service และ RPC methods
service Greeter {
  rpc SayHello (HelloRequest) returns (HelloReply);
}
```

จุดสำคัญ:

- `message` = โครงสร้างข้อมูล (เหมือน struct/class)
- `service` = กลุ่มของ method ที่ server จะให้บริการ
- ตัวเลขหลัง `= 1`, `= 2` คือ **field number** ใช้ระบุ field ใน binary format
  ห้ามเปลี่ยนหลัง production แล้ว เพราะจะทำให้ backward compatibility พัง

---

## 3. Contract (สัญญา/ข้อตกลง)

**Contract** ในบริบทของ gRPC หมายถึง **ข้อตกลงร่วมกันระหว่าง client กับ server**
ว่า service มี method อะไร รับ-ส่งข้อมูลรูปแบบไหน ซึ่งถูกกำหนดในไฟล์ `.proto`

แนวคิด **Contract-First Development**:

1. ออกแบบ `.proto` ก่อน (contract)
2. Compile เป็นโค้ดทั้งฝั่ง client และ server
3. ทั้งสองฝั่งพัฒนาแยกกันได้ ตราบใดที่ยังเคารพ contract เดียวกัน

ข้อดี:

- ลด miscommunication ระหว่างทีม frontend/backend
- IDE auto-complete + type checking ตั้งแต่ compile time
- เปลี่ยนภาษาฝั่งใดฝั่งหนึ่งได้โดยไม่ต้องแก้อีกฝั่ง

---

## 4. Stub (Client Stub / Server Stub)

**Stub** คือ **โค้ดที่ถูก generate อัตโนมัติ** จากไฟล์ `.proto` โดยเครื่องมือเช่น
`protoc`, `protoc-gen-go-grpc` หน้าที่ของ stub คือทำให้การเรียก RPC ดูเหมือน
เรียกฟังก์ชันปกติในภาษานั้นๆ

### 4.1 Client Stub

เป็นตัวแทนของ remote service ในฝั่ง client ทำหน้าที่:

1. **Marshal** request เป็น binary (Protobuf)
2. ส่งผ่าน HTTP/2 ไปยัง server
3. รอ response แล้ว **unmarshal** กลับเป็น object

ตัวอย่าง (Go):

```go
conn, _ := grpc.Dial("localhost:50051", grpc.WithInsecure())
defer conn.Close()

client := greeterpb.NewGreeterClient(conn) // ← นี่คือ client stub

resp, err := client.SayHello(ctx, &greeterpb.HelloRequest{Name: "Kom"})
fmt.Println(resp.GetMessage())
```

จากภายนอกดูเหมือนเรียก method ปกติ แต่ภายในคือการคุยข้ามเครือข่าย

### 4.2 Server Stub (Service Skeleton)

เป็น interface ที่ฝั่ง server ต้อง **implement** เพื่อให้บริการ method ที่ประกาศไว้
ใน `.proto`

ตัวอย่าง (Go):

```go
type server struct {
    greeterpb.UnimplementedGreeterServer
}

func (s *server) SayHello(ctx context.Context, req *greeterpb.HelloRequest) (*greeterpb.HelloReply, error) {
    return &greeterpb.HelloReply{
        Message: "Hello, " + req.GetName(),
    }, nil
}
```

> หมายเหตุ: คำว่า "stub" ในตำราเก่ามักหมายถึงฝั่ง client โดยฝั่ง server เรียก
> **skeleton** แต่ในเอกสาร gRPC สมัยใหม่ มักใช้คำว่า stub กับทั้งสองฝั่ง

---

## 5. Channel (ช่องทางการเชื่อมต่อ)

**Channel** คือ abstraction ของการเชื่อมต่อจาก client ไปยัง server มีหน้าที่:

- จัดการ connection pool ภายใต้ HTTP/2
- ทำ **load balancing** ระหว่างหลาย server replicas
- จัดการ **reconnection** เมื่อ connection หลุด
- รองรับ TLS / authentication

```go
conn, err := grpc.Dial(
    "localhost:50051",
    grpc.WithTransportCredentials(insecure.NewCredentials()),
)
```

ในภาษา Go ใช้ `*grpc.ClientConn`, ในภาษา Python/Java ใช้ `Channel` โดยตรง

ควรสร้าง channel **ครั้งเดียวแล้วใช้ซ้ำ** อย่าสร้างใหม่ทุกครั้งที่เรียก RPC

---

## 6. Server (gRPC Server)

ฝั่ง server มีหน้าที่:

1. สร้าง gRPC server instance
2. **Register service implementation** ที่ implement server stub
3. Listen บน port แล้วรอรับ request

```go
lis, _ := net.Listen("tcp", ":50051")
s := grpc.NewServer()
greeterpb.RegisterGreeterServer(s, &server{})
log.Println("listening on :50051")
s.Serve(lis)
```

---

## 7. รูปแบบการสื่อสาร (RPC Types) 4 แบบ

gRPC รองรับ 4 รูปแบบ ขึ้นอยู่กับการประกาศใน `.proto`:

| รูปแบบ                  | Client ส่ง | Server ตอบ | ตัวอย่างการใช้งาน           |
| ----------------------- | ---------- | ---------- | --------------------------- |
| **Unary**               | 1 ครั้ง    | 1 ครั้ง    | Login, Get user             |
| **Server streaming**    | 1 ครั้ง    | หลายครั้ง  | Subscribe ราคาหุ้น          |
| **Client streaming**    | หลายครั้ง  | 1 ครั้ง    | Upload file เป็น chunk      |
| **Bidirectional**       | หลายครั้ง  | หลายครั้ง  | Chat, real-time game        |

ตัวอย่างประกาศใน `.proto`:

```proto
service ChatService {
  rpc SendMessage (Message) returns (Ack);                       // Unary
  rpc Subscribe (Topic) returns (stream Message);                // Server streaming
  rpc Upload (stream Chunk) returns (UploadResult);              // Client streaming
  rpc Chat (stream Message) returns (stream Message);            // Bidirectional
}
```

---

## 8. ภาพรวมการทำงาน (Flow)

```text
┌──────────────┐                                  ┌──────────────┐
│   Client     │                                  │   Server     │
│              │                                  │              │
│  business    │                                  │  business    │
│  code        │                                  │  logic       │
│    │         │                                  │     ▲        │
│    ▼         │                                  │     │        │
│  Client Stub │── Protobuf binary over HTTP/2 ──▶│ Server Stub  │
│  (generated) │◀── Protobuf binary over HTTP/2 ──│ (generated)  │
│    │         │                                  │     ▲        │
│    ▼         │                                  │     │        │
│   Channel    │──────── TCP / TLS ──────────────▶│   Listener   │
└──────────────┘                                  └──────────────┘
        ▲                                                 ▲
        └─────────── ใช้ contract เดียวกัน ───────────────┘
                       (ไฟล์ .proto)
```

ขั้นตอนเมื่อ client เรียก `SayHello`:

1. Client เรียก `client.SayHello(ctx, req)` ผ่าน **client stub**
2. Stub ทำ **serialization** (marshal) `req` เป็น Protobuf binary
3. ส่งผ่าน **channel** → HTTP/2 frame → network
4. Server รับ frame → **server stub** ทำ **deserialization** (unmarshal)
5. เรียก method ที่ implement ไว้
6. ผลลัพธ์ถูก marshal กลับ → ส่งกลับเป็น HTTP/2 response
7. Client stub unmarshal เป็น object ส่งคืนให้ business code

---

## 9. คำศัพท์อื่นที่ควรรู้

| คำศัพท์            | ความหมาย                                                                  |
| ------------------ | ------------------------------------------------------------------------- |
| **Marshal**        | แปลง object → binary เพื่อส่งผ่านเครือข่าย                                |
| **Unmarshal**      | แปลง binary → object เพื่อใช้งานในโค้ด                                    |
| **Metadata**       | Key-value แนบไปกับ request/response คล้าย HTTP headers (เช่น auth token)  |
| **Deadline**       | กำหนดเวลาที่ RPC ต้องเสร็จ ถ้าเกินจะ cancel อัตโนมัติ                     |
| **Context**        | object พา deadline, cancellation, metadata ไปกับ request                  |
| **Interceptor**    | middleware ของ gRPC สำหรับ logging, auth, metrics                         |
| **Status code**    | รหัสผลลัพธ์ของ RPC เช่น `OK`, `NOT_FOUND`, `UNAUTHENTICATED`              |
| **Reflection**     | ความสามารถให้ client query schema จาก server ตอน runtime (debug ได้ง่าย) |
| **gRPC-Web**       | gRPC variant ที่รันบน browser ผ่าน proxy เช่น Envoy                       |

---

## 10. ขั้นตอนการเริ่มต้นโปรเจกต์ gRPC

1. ติดตั้ง `protoc` (Protocol Buffer compiler)
2. ติดตั้ง plugin ของภาษาที่ใช้ เช่น `protoc-gen-go`, `protoc-gen-go-grpc`
3. เขียนไฟล์ `.proto` (contract)
4. Generate code:

   ```bash
   protoc --go_out=. --go_opt=paths=source_relative \
          --go-grpc_out=. --go-grpc_opt=paths=source_relative \
          proto/greeter.proto
   ```

5. Implement server (`server/`)
6. เขียน client เรียก stub (`client/`)
7. รัน server แล้วทดสอบด้วย client หรือเครื่องมือ เช่น `grpcurl`, `Postman`

---

## 11. สรุปแบบสั้นๆ

- **Contract** = ไฟล์ `.proto` ที่ทั้งสองฝั่งใช้ร่วมกัน
- **Protobuf** = format ของข้อมูล (binary, schema-based)
- **Stub** = โค้ดที่ generate จาก `.proto` ใช้แทน remote function
- **Channel** = ตัวจัดการการเชื่อมต่อระหว่าง client ↔ server
- **Server** = process ที่ implement service และรอรับ request
- **Client** = process ที่ใช้ stub เรียก remote method ผ่าน channel

> เมื่อเข้าใจ 6 คำนี้แล้ว ที่เหลือคือการเลือกใช้รูปแบบการสื่อสาร (unary/streaming),
> จัดการ error, ทำ auth และ observability เพิ่มเติม

---

## เอกสารอ้างอิง

- [gRPC Official Docs](https://grpc.io/docs/)
- [Protocol Buffers Language Guide (proto3)](https://protobuf.dev/programming-guides/proto3/)
- [gRPC Concepts](https://grpc.io/docs/what-is-grpc/core-concepts/)
