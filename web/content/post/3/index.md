---
tags:
- Go
- RPC

title: "Go RPC"
date: 2022-07-09T18:28:48+08:00
draft: true
---

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

// TODO…