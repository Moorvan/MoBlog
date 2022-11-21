---
tags:
- Go
- web

title: "当 trailing slash 遇到 CORS"
date: 2022-11-21T23:36:00+08:00
draft: true
---


# Intro

大佬室友的项目遇到了一个问题，这是一个前后端分离的 web 项目，后端采用的是 go 中的 gin 框架，采用了 cors 处理的中间件，但是在访问一些接口时出现了跨域问题，其返回的报头中并没有

```go
Access-Control-Allow-Origin: *
```

但是后端程序使用 cors 中间件。

同时注意到返回的状态码为 301.

后来我们发现，大佬写的前端 fetch 后端的路径并不匹配，前端程序中 fetch /xxx，然而后端的路由定义为 /xxx/，然后我们把前端服务中的路径修改为 /xxx/，这个问题就消失了，这是怎么回事呢？

# 相关知识

## 跨域问题

### 同源策略

在 Web 浏览器层面，运行某个网页脚本访问另一个网页的资源，但是要求这两个网页必须同源，这里的同源指的是两个网页具有相同的协议、主机名和端口号，其中对于路径没有要求。它能帮助阻隔恶意文档，减少可能被攻击的媒介。

可以通过 CORS 来允许跨源访问。

### CORS

跨源资源共享（Cross-origin resource sharing，CORS）是一种基于 HTTP 头的机制，从而允许服务器标示，除了自己外的其他 origin，使得浏览器允许这些 origin 访问加载自己的资源。

## trailing slash 与重定向

在 gin 框架中的默认配置中，有：

```go
RedirectTrailingSlash:  true,
// ....
```

即将 RedirectRailingSlash 置为 true. 该参数的含义为：

```go
type Engine struct {
	RouterGroup

	// Enables automatic redirection if the current route can't be matched but a
	// handler for the path with (without) the trailing slash exists.
	// For example if /foo/ is requested but a route only exists for /foo, the
	// client is redirected to /foo with http status code 301 for GET requests
	// and 307 for all other request methods.
	RedirectTrailingSlash bool
// ...
```

其实这里说明的很清楚了，当该参数为 true 时表示当当前的路有不能被匹配上时，但是有一个 with or without 结尾 slash 的路由存在，那么客户端会被重定向到匹配到路由上，对于 Get 请求返回 301 重定向，对于其他请求返回 307 状态码。

看代码逻辑：

```go
if httpMethod != "CONNECT" && rPath != "/" {
	if value.tsr && engine.RedirectTrailingSlash {
		redirectTrailingSlash(c)
		return
	}
	if engine.RedirectFixedPath && redirectFixedPath(c, root, engine.RedirectFixedPath) {
		return
	}
}
```

这里应该是当匹配失败时候，而且是因为末尾的 slash 多了或者少了导致匹配失败，调用 redirectTrailingSlash.

```go
func redirectTrailingSlash(c *Context) {
	req := c.Request
	p := req.URL.Path
	if prefix := path.Clean(c.Request.Header.Get("X-Forwarded-Prefix")); prefix != "." {
		p = prefix + "/" + req.URL.Path
	}
	req.URL.Path = p + "/"
	if length := len(p); length > 1 && p[length-1] == '/' {
		req.URL.Path = p[:length-1]
	}
	redirectRequest(c)
}
```

给 req.URL.Path 结尾加上或者删去 slash，然后调用 redirectRequest.

```go
func redirectRequest(c *Context) {
	req := c.Request
	rPath := req.URL.Path
	rURL := req.URL.String()

	code := http.StatusMovedPermanently // Permanent redirect, request with GET method
	if req.Method != http.MethodGet {
		code = http.StatusTemporaryRedirect
	}
	debugPrint("redirecting request %d: %s --> %s", code, rPath, rURL)
	http.Redirect(c.Writer, req, rURL, code)
	c.writermem.WriteHeaderNow()
}
```

直接调用 http 库中的 Redirect，返回重定向状态码。所以该过程中没有经过任何中间件，直接返回了重定向。

# 分析问题

所以大佬问题的原因就明了了，其过程为：

前端请求后端服务器时请求的路径由于 trailing slash 匹配失败，gin 默认的处理方式是不经过任何中间件调用 http 库中的 redirect，返回后的客户端按照重定向的地址继续进行请求，但是由于返回的报文中没有允许跨域，导致了之后的请求失败。因为重定向返回请求没有经过 cors 中间件对跨域进行处理，默认不允许跨域了。

# 不同框架中的处理策略

感觉这个问题很容易出现，而且不容易找到原因，因此搜索了一下，有什么比较好的规避方法，感觉应该是框架应该提供，或者帮忙解决了。

![Untitled](%E5%BD%93%20trailing%20slash%20%E9%81%87%E5%88%B0%20CORS%20e22587b998ea47a28cb156545359e64f/Untitled.png)

可见不同的框架可能都有这个问题。大多数框架都会对 trailing slash 进行默认处理，让客户端重定向，但是这时很可能就会遇到跨域的问题。

我是用 fiber 重新实现了类似的功能，但是没有遇到这个问题了，因此研究了一下 fiber 中是如何处理 trailing slash 的。

注意到在 fiber 中也有类似的处理配置：

```go
// When set to true, the router treats "/foo" and "/foo/" as different.
// By default this is disabled and both "/foo" and "/foo/" will execute the same handler.
//
// Default: false
StrictRouting bool `json:"strict_routing"`
```

这个参数配置默认是 false，即忽略定义和请求中的 trailing slash 进行匹配。

```go
// Strict routing, remove trailing slashes
if !app.config.StrictRouting && len(pathPretty) > 1 {
	pathPretty = utils.TrimRight(pathPretty, '/')
}
```

这段代码在 register 中，即进行路由注册的时候，如果 StrictRouting 为 false 时，将结尾 slash 自动去除；

在接受请求过程中：

```go
// Path returns the path part of the request URL.
// Optionally, you could override the path.
func (c *Ctx) Path(override ...string) string {
	if len(override) != 0 && c.path != override[0] {
		// Set new path to context
		c.pathOriginal = override[0]

		// Set new path to request context
		c.fasthttp.Request.URI().SetPath(c.pathOriginal)
		// Prettify path
		c.configDependentPaths()
	}
	return c.path
}
```

调用 configDependentPaths.

```go
// configDependentPaths set paths for route recognition and prepared paths for the user,
// here the features for caseSensitive, decoded paths, strict paths are evaluated
func (c *Ctx) configDependentPaths() {
	c.pathBuffer = append(c.pathBuffer[0:0], c.pathOriginal...)
	// If UnescapePath enabled, we decode the path and save it for the framework user
	if c.app.config.UnescapePath {
		c.pathBuffer = fasthttp.AppendUnquotedArg(c.pathBuffer[:0], c.pathBuffer)
	}
	c.path = c.app.getString(c.pathBuffer)

	// another path is specified which is for routing recognition only
	// use the path that was changed by the previous configuration flags
	c.detectionPathBuffer = append(c.detectionPathBuffer[0:0], c.pathBuffer...)
	// If CaseSensitive is disabled, we lowercase the original path
	if !c.app.config.CaseSensitive {
		c.detectionPathBuffer = utils.ToLowerBytes(c.detectionPathBuffer)
	}
	// If StrictRouting is disabled, we strip all trailing slashes
	if !c.app.config.StrictRouting && len(c.detectionPathBuffer) > 1 && c.detectionPathBuffer[len(c.detectionPathBuffer)-1] == '/' {
		c.detectionPathBuffer = utils.TrimRightBytes(c.detectionPathBuffer, '/')
	}
	c.detectionPath = c.app.getString(c.detectionPathBuffer)

	// Define the path for dividing routes into areas for fast tree detection, so that fewer routes need to be traversed,
	// since the first three characters area select a list of routes
	c.treePath = c.treePath[0:0]
	if len(c.detectionPath) >= 3 {
		c.treePath = c.detectionPath[:3]
	}
}
```

可以看到，在这里将 detectionPathBuffer 的结尾 slash 进行了去除。

由此可见 fiber 中对 path 的处理并没有通过重定向的方式，而是直接在注册路由和接受请求时对路径进行简单处理。