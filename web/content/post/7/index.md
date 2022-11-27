---
tags:
- Go

title: "Go 中 Timer 的正确使用姿势"
date: 2022-11-27T16:53:04+08:00
draft: false 
---

# 写在前面

Go 语言中提供了 Timer 定时器，因此很容易为了满足超时处理的需求，写出如下代码：

```go
for {
	select {
	case v := <-c:
		handle(v)
	case <-time.After(1 * time.Minute):
		timeout()
	}
}
```

然而这里虽然 Timer 变量对应的部分资源会被 GC 回收，但是其实需要在 runtime 中维护了一个定期 wake up 的最小堆，需要彻底删除，则需要定时器到期或者 Stop 这个定时器。

因此这个问题被发现在资源被大量占用：[慎用time.After会造成内存泄漏（golang）](https://segmentfault.com/a/1190000024523708)

那么应该如何正确使用定时器呢？

# 正确使用 Timer

那么是否可以在每次进入 select 块的时候对 timer 进行 reset 呢，对于 timer 中的 reset 函数中，文档如下：

```go
// Reset changes the timer to expire after duration d.
// It returns true if the timer had been active, false if the timer had
// expired or been stopped.
//
// For a Timer created with NewTimer, Reset should be invoked only on
// stopped or expired timers with drained channels.
//
// If a program has already received a value from t.C, the timer is known
// to have expired and the channel drained, so t.Reset can be used directly.
// If a program has not yet received a value from t.C, however,
// the timer must be stopped and—if Stop reports that the timer expired
// before being stopped—the channel explicitly drained:
//
//	if !t.Stop() {
//		<-t.C
//	}
//	t.Reset(d)
//
// This should not be done concurrent to other receives from the Timer's
// channel.
//
// Note that it is not possible to use Reset's return value correctly, as there
// is a race condition between draining the channel and the new timer expiring.
// Reset should always be invoked on stopped or expired channels, as described above.
// The return value exists to preserve compatibility with existing programs.
//
// For a Timer created with AfterFunc(d, f), Reset either reschedules
// when f will run, in which case Reset returns true, or schedules f
// to run again, in which case it returns false.
// When Reset returns false, Reset neither waits for the prior f to
// complete before returning nor does it guarantee that the subsequent
// goroutine running f does not run concurrently with the prior
// one. If the caller needs to know whether the prior execution of
// f is completed, it must coordinate with f explicitly.
```

这里进行说明：

reset 函数会重置 timer，使其重新设定超时在 d 时间后。它有一个返回值，如果为 true，那么 timer 已经被激活，否则指的是 timer 已经过期或者被 stop。

对于创建的一个 timer，reset 应该仅当 stop 了或者 超时了，同时从管道 C 中取出了元素才会被调用。

如果进程中硬件从 t.C 中接收了值，定时器已经超时，channal 已经取空，那么 Reset 可以直接被调用。如果进程中还没有从 t.C 中接收值，timer 必须被 stop，如果已经超时，那么必须要从管道中把东西取了，代码如下：

```go
if !t.Stop() {
	<-t.C
}
t.Reset(d)
```

这不能并发用。

不可能正确使用 Reset 的返回值，因为在 t.C 接收和定时器过期之间有 race condition。

只有被 stop 了 或者 过期了才能调用 reset。

注意到这里如果 t.C 已经被取出了，那么就会重复取，然后陷入死锁了。

这里给出在顺序执行中 timer 正确使用的 pattern：

```go
timer := time.NewTimer(1 * time.Minute)
for {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(1 * time.Minute)
	select {
	case v := <-c:
		handle(v)
	case <-timer.C:
		timeout()
	}
}
```

注意这里对于 timer 这个资源来说，并发则不安全。对于并发，没有可以安全的办法。