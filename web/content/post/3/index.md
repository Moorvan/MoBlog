---
tags:
- Go
- RPC

title: "Go RPC"
date: 2022-07-09T18:28:48+08:00
draft: true
---

# Go RPC

本文介绍使用 Go 标准库 net/rpc 简单实现 RPC。

# 调用实现

标准库中 RPC 实现主要依赖于 Http Serve，在 RPC 包中定义好了交流的协议。

## Server

先来实现 Server 端。本文中实现一个远程算术运算服务。

rpc 主要通过注册服务的方式来构建，使用：

```go
rpc.Register(any)
```

函数给 rpc 包中默认的 rpc Server 注册一个服务。传入参数是 any 类型。在 rpc Server 中将其方法导出提供给网络请求调用。因此对于其需要被可见的方法定义上会有一些要求。先看例子。

```go
type Arithmetic int

func (a *Arithmetic) Add(args Req, resp *Resp) error {
	log.Println("Adding Args: ", args.A, args.B)
	resp.State = "OK"
	resp.Result = args.A + args.B
	return nil
}
```

这里我们简单实现了一个 Arithmetic 对象，其上挂了一个 Add 方法，这个方法需要暴露给外界调用，该方法有两个参数，其定义为：

```go
type Req struct {
	A, B int
}

type Resp struct {
	State  string
	Result int
}
```

由于该方法以及参数需要被暴露导出，所以有具体要求：

1. 方法的类型本身需要是外界可见（首字母大写）
2. 方法需要外界可见
3. 方法的两个参数必须是外界可见，或者是 Go 预置类型
4. 方法的第二个参数必须是指针
5. 方法返回值为 error

基本上这就限定了方法的定义，也大概理解了其作用。

1. 传入两个参数，第一个参数可以理解为 RPC 过程的实际传入参数，第二个参数为返回值
2. 需要涉及到的类型全外部可见。

将对象注册到 rpc 默认 server 中，按照要求定义的方法就可以通过 RPC 调用了：

```go
func main() {
	arith := new(Arithmetic)
	err := rpc.Register(arith)
	if err != nil {
		log.Fatalln("rpc register err: ", err)
	}
	rpc.HandleHTTP()

	l, err := net.Listen("tcp", ":1234")
	if err != nil {
		log.Fatalln("Listen error: ", err)
	}
	log.Fatalln(http.Serve(l, nil))
}
```

可以看到，使用 rpc 包中的 Register 函数将需要的服务进行注册，调用 `rpc.HandleHttp()`函数，给 rpc 包中的默认 Server 注册 HttpHandler，之后调用 net 包中的监听 Listen，并最后调用 Serve 完成一个简单的服务端。（具体原理见后文，这里主要说明如何使用

## Client

客户端比较简单，使用 `rpc.DialHTTP`连接 rpc 服务：

```go
client, err := rpc.DialHTTP("tcp", ":1234")
if err != nil {
	log.Fatalln("dialing error: ", err)
}
```

在客户端也需要定义需要的数据结构：

```go
type Req struct {
	A, B int
}

type Resp struct {
	State  string
	Result int
}
```

实例化好传入参数，准备好接受 response：

```go
req := Req{A: 1, B: 2}
resp := Resp{}
err = client.Call("Arithmetic.Add", req, &resp)
if err != nil {
	log.Fatalln("call add error: ", err)
}
log.Printf("Result: %+v\n", resp)
```

调用可以使用 Call 进行同步调用，等待返回结果，传入的第一个参数即需要指定服务和方法，后面传入在 Server 端定义好的两个参数，即可得到结果存在 resp 中。

如果需要异步调用，那么直接看 Call 函数的实现即可知道：

```go
// Call invokes the named function, waits for it to complete, and returns its error status.
func (client *Client) Call(serviceMethod string, args any, reply any) error {
	call := <-client.Go(serviceMethod, args, reply, make(chan *Call, 1)).Done
	return call.Error
}
```

那么可以仿照这里，调用 client 的 Go 函数，相比 Call 增加了一个管道。

调用：

```go
call := client.Go("Arithmetic.Add", req, &resp, make(chan *rpc.Call, 1))

_ = <-call.Done
log.Printf("Result: %+v\n", resp)
```

通过管道 Done 来进行通知收到结果。

# 原理过程

## Server 端过程

调用 rpc 的 Register 方法，该方法即：

```go
// Register publishes the receiver's methods in the DefaultServer.
func Register(rcvr any) error { return DefaultServer.Register(rcvr) }
```

调用 rpc 包中 DefaultServer 的 Register 方法。

DefaultServer 定义：

```go
// NewServer returns a new Server.
func NewServer() *Server {
	return &Server{}
}

// DefaultServer is the default instance of *Server.
var DefaultServer = NewServer()
```

实例化的一个 Server，对于 rpc 包中的 Server：

```go
// Server represents an RPC Server.
type Server struct {
	serviceMap sync.Map   // map[string]*service
	reqLock    sync.Mutex // protects freeReq
	freeReq    *Request
	respLock   sync.Mutex // protects freeResp
	freeResp   *Response
}
```

其中主要存储了 serviceMap，从 string 到 service 到映射，联想到在客户端请求时是传入了 服务+方法 的字符串。

对于 DefaultServer 调用的 Register 方法：

```go
// Register publishes in the server the set of methods of the
// receiver value that satisfy the following conditions:
//	- exported method of exported type
//	- two arguments, both of exported type
//	- the second argument is a pointer
//	- one return value, of type error
// It returns an error if the receiver is not an exported type or has
// no suitable methods. It also logs the error using package log.
// The client accesses each method using a string of the form "Type.Method",
// where Type is the receiver's concrete type.
func (server *Server) Register(rcvr any) error {
	return server.register(rcvr, "", false)
}
```

调用了一个私有方法，增加了两个参数。注意到这里注释中说明了传入 any 类型的要求，即会暴露导出提供 RPC 服务的方法要求。可以看到实际上对外提供的注册方法是对 register 私有方法对封装，可以看到会有另一个 Register 的版本：

```go
// RegisterName is like Register but uses the provided name for the type
// instead of the receiver's concrete type.
func (server *Server) RegisterName(name string, rcvr any) error {
	return server.register(rcvr, name, true)
}
```

这个版本可以为该服务提供新的名字，而不是使用传入实例的类型名。接下来看 register 中：

```go
func (server *Server) register(rcvr any, name string, useName bool) error {
	s := new(service) // 实例化 server 类型：
	/*
	type service struct {
		name   string                 // name of service
		rcvr   reflect.Value          // receiver of methods for the service
		typ    reflect.Type           // type of the receiver
		method map[string]*methodType // registered methods
	}
	*/
	s.typ = reflect.TypeOf(rcvr) // 获取传入参数名
	s.rcvr = reflect.ValueOf(rcvr) // 值
	sname := reflect.Indirect(s.rcvr).Type().Name() // 获得结构体类型名
	if useName {
		sname = name
	} // 如果有给定名字 sname 为给定名称
	if sname == "" {
		s := "rpc.Register: no service name for type " + s.typ.String()
		log.Print(s)
		return errors.New(s)
	}
	if !token.IsExported(sname) && !useName { // 判断名称是否为 公开 Public
		s := "rpc.Register: type " + sname + " is not exported"
		log.Print(s)
		return errors.New(s)
	}
	s.name = sname

	// Install the methods
	s.method = suitableMethods(s.typ, logRegisterError) // 将合适的方法进行注册，存储到 method: map[string]*methodType 中：
	/*
	type methodType struct {
		sync.Mutex // protects counters
		method     reflect.Method
		ArgType    reflect.Type
		ReplyType  reflect.Type
		numCalls   uint
	}
	*/
	if len(s.method) == 0 { // 提示没有可用 RPC 方法
		str := ""

		// To help the user, see if a pointer receiver would work.
		method := suitableMethods(reflect.PointerTo(s.typ), false)
		if len(method) != 0 {
			str = "rpc.Register: type " + sname + " has no exported methods of suitable type (hint: pass a pointer to value of that type)"
		} else {
			str = "rpc.Register: type " + sname + " has no exported methods of suitable type"
		}
		log.Print(str)
		return errors.New(str)
	}

	if _, dup := server.serviceMap.LoadOrStore(sname, s); dup { // 将 sname + s 存入 service 的 serviceMap field 中
		return errors.New("rpc: service already defined: " + sname)
	}
	return nil
}
```

注意到这里可以展开的为 `suitableMethods` 函数，这里将该类型中符合要求的方法都抽出存放在 `map[string]*methodType` 中：

```go
// suitableMethods returns suitable Rpc methods of typ. It will log
// errors if logErr is true.
func suitableMethods(typ reflect.Type, logErr bool) map[string]*methodType
```

// TODO…

rpc 完成 register 过程后调用 rpc 包中的 `HandleHTTP` 函数：

```go
// HandleHTTP registers an HTTP handler for RPC messages to DefaultServer
// on DefaultRPCPath and a debugging handler on DefaultDebugPath.
// It is still necessary to invoke http.Serve(), typically in a go statement.
func HandleHTTP() {
	DefaultServer.HandleHTTP(DefaultRPCPath, DefaultDebugPath)
}
// ...
const (
	// Defaults used by HandleHTTP
	DefaultRPCPath   = "/_goRPC_"
	DefaultDebugPath = "/debug/rpc"
)
```

调用 DefaultServer 的 HandleHTTP 函数，传入疑似路由地址的字符串。

```go
// HandleHTTP registers an HTTP handler for RPC messages on rpcPath,
// and a debugging handler on debugPath.
// It is still necessary to invoke http.Serve(), typically in a go statement.
func (server *Server) HandleHTTP(rpcPath, debugPath string) {
	http.Handle(rpcPath, server)
	http.Handle(debugPath, debugHTTP{server})
}
```

果然，后面调用 http 包中的 Handle 函数，为 http 包中的 `DefaultServeMux` 指定路由，那么由此可见，server 是一个满足 http 包中 Handler 接口的类型。

```go
type Handler interface {
	ServeHTTP(ResponseWriter, *Request)
}
```

可以看到具体实现：

```go
// ServeHTTP implements an http.Handler that answers RPC requests.
func (server *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != "CONNECT" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		io.WriteString(w, "405 must CONNECT\n")
		return
	}
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Print("rpc hijacking ", req.RemoteAddr, ": ", err.Error())
		return
	}
	io.WriteString(conn, "HTTP/1.0 "+connected+"\n\n")
	server.ServeConn(conn)
}
```

至此，对于服务端相关调用过程分析结束。

## Client 端过程

在 Client 主要有两步，连接 + 调用。

首先调用 rpc 中的 DialHTTP 函数：

```go
// DialHTTP connects to an HTTP RPC server at the specified network address
// listening on the default HTTP RPC path.
func DialHTTP(network, address string) (*Client, error) {
	return DialHTTPPath(network, address, DefaultRPCPath)
}
```

传入两个 string，第一个为网络协议，第二个为目标地址，返回客户端和错误。

其中调用 DialHTTPPath，传入协议和地址，以及路由路径：

```go
// DialHTTPPath connects to an HTTP RPC server
// at the specified network address and path.
func DialHTTPPath(network, address, path string) (*Client, error) {
	conn, err := net.Dial(network, address) // 调用 net 包中的 Dial 通过 网络协议 + 地址进行连接
	if err != nil {
		return nil, err
	}
	io.WriteString(conn, "CONNECT "+path+" HTTP/1.0\n\n")

	// Require successful HTTP response
	// before switching to RPC protocol.
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "CONNECT"}) // HTTP 连接
	if err == nil && resp.Status == connected {
		return NewClient(conn), nil
	}
	if err == nil {
		err = errors.New("unexpected HTTP response: " + resp.Status)
	}
	conn.Close()
	return nil, &net.OpError{
		Op:   "dial-http",
		Net:  network + " " + address,
		Addr: nil,
		Err:  err,
	}
}
```

连接后调用 Call 或者是 Go 方法进行远程调用，对于 Call 方法：

```go
// Call invokes the named function, waits for it to complete, and returns its error status.
func (client *Client) Call(serviceMethod string, args any, reply any) error {
	call := <-client.Go(serviceMethod, args, reply, make(chan *Call, 1)).Done
	return call.Error
}
```

可以看到实际依然是调用了 Go 方法，Go 方法返回值中取出 Done 管道，该管道吐出东西时，异步调用结束，通过管道对 Go 方法进行封装，实现了同步对远程过程的调用。但是注意到这里同步会一直等待，如果需要设置超时之类的就需要手动调用 Go 方法了。传入参数为 调用服务方法名，调用参数，接收参数。

```go
// Go invokes the function asynchronously. It returns the Call structure representing
// the invocation. The done channel will signal when the call is complete by returning
// the same Call object. If done is nil, Go will allocate a new channel.
// If non-nil, done must be buffered or Go will deliberately crash.
func (client *Client) Go(serviceMethod string, args any, reply any, done chan *Call) *Call
```

Go 方法比 Call 方法传入多一个管道，完成异步调用，返回值为 *Call：

```go
// Call represents an active RPC.
type Call struct {
	ServiceMethod string     // The name of the service and method to call.
	Args          any        // The argument to the function (*struct).
	Reply         any        // The reply from the function (*struct).
	Error         error      // After completion, the error status.
	Done          chan *Call // Receives *Call when Go is complete.
}
```

继续看 Go 方法中实现：

```go
func (client *Client) Go(serviceMethod string, args any, reply any, done chan *Call) *Call {
	call := new(Call) // new 一个返回结构
	call.ServiceMethod = serviceMethod
	call.Args = args
	call.Reply = reply // 构造
	if done == nil {
		done = make(chan *Call, 10) // buffered.
	} else {
		// If caller passes done != nil, it must arrange that
		// done has enough buffer for the number of simultaneous
		// RPCs that will be using that channel. If the channel
		// is totally unbuffered, it's best not to run at all.
		if cap(done) == 0 {
			log.Panic("rpc: done channel is unbuffered")
		}
	}
	call.Done = done
	client.send(call)
	return call
}
```

可以看到，该方法中主要构造了 Call 结构体，并将其返回，其中如果传入 done 管道为 nil 则构建一个 10 buffer 的管道，该 buffer 大概是作为调用的次数缓存。

其中调用 client.send(call)，传入了 call 结构体，send 方法中进行请求发送。

# 写在最后

之后笔者将继续学习 rpc 相关领域，下一步去做谷歌的 gRPC 框架的学习，对分布式系统领域进行深入学习。当然，在编程语言上，目前和之后的选择均为 Go 语言优先。（挖坑