# 15 · 一次请求穿过哪些边界

> 一句话模型：Server 管连接与超时，Mux 管路由，Handler 管一次请求；谁拥有资源谁负责关闭。

## 先看一个现象

客户端发来 `DELETE /tasks/42`，服务端确实注册了 `/tasks/{id}`，你以为会返回 200 或者"找不到"的 404，实际拿到的是 **405 Method Not Allowed**。换个请求 `GET /unknown/404`，同样是"没有这个资源"，这次却返回 404。

```go
mux.HandleFunc("GET /tasks/{id}", getTask)
mux.HandleFunc("POST /tasks", createTask)
```

两个请求都没被业务代码碰到，响应却不同。这说明在 Handler 之前已经有一层做了判断，而且它看的不只是路径，还有方法。再看一个更隐蔽的故障：Handler 里启动了一个后台任务就返回，连接被回收了，那个任务还在跑——谁该为它负责？

一次请求要穿过好几道边界，每道边界只回答自己的问题。把责任放错层，就会出现"路径对了却 405""超时设了却仍卡死"这类看着像业务的传输问题。最小对照是这样的：

```text
GET    /healthz       -> 200
DELETE /tasks/42      -> 405   路径命中，方法不允许
GET    /unknown/404   -> 404   没有任何模式命中
```

三个请求都没碰到业务代码，状态码却已经分出高下——判断发生在 Handler 之前。

## 先建立直觉

:::figure 15-http-boundary|请求流水线与四类超时的位置
主流水线从左到右：Server 接受连接并解析请求，ServeMux 按方法和路径选 Handler，Handler 处理这一次请求，ResponseWriter 写回状态和响应体。橙色标签钉住四类超时各自的位置：两个读超时在 Server 入口一侧，写超时在响应一侧，空闲超时管连接回收。下半部分点出两件事：404 与 405 的分界在 Mux，资源关闭的责任跟着所有权走。
:::

图要读出一条原则：**每一层只做自己那一格的事。** Server 不认识你的任务，Mux 不查询数据库，Handler 不去监听端口。分层不是为了目录好看，而是为了让某一层的问题能被单独观察和替换。

## 机制拆成四步

### 1. Server 管连接与超时：它服务很多请求，不认识你的业务

`http.Server` 负责监听、接受连接、解析 HTTP、在请求结束后回收或复用连接。一个 Server 服务大量请求，因此它的配置都是**连接级**的，不是业务级的：

```go
server := &http.Server{
	Addr:              "127.0.0.1:8080",
	Handler:           newHandler(),
	ReadHeaderTimeout: 5 * time.Second,
	IdleTimeout:       60 * time.Second,
}
```

为什么要单独设 `ReadHeaderTimeout`？因为"连上但不发完整请求头"是一种廉价攻击：客户端几乎不花成本，服务端却要为它占着一个连接和一个 Goroutine。读超时要分两段，是因为"慢请求头"和"慢请求体"是两种不同的故障，一刀切会误伤正常的大文件上传。

四类超时各封一种故障，别指望一个数字全包：

| 配置 | 封住什么 | 封不住什么 |
| --- | --- | --- |
| `ReadHeaderTimeout` | 连上后迟迟不发完整请求头 | 请求体传输慢 |
| `ReadTimeout` | 请求体读取得太慢 | Handler 内部执行太久 |
| `WriteTimeout` | 响应写不出去或客户端不接收 | 数据库查询卡住 |
| `IdleTimeout` | Keep-Alive 连接长期空占 | 活跃请求的处理时限 |

### 2. Mux 管路由：方法也是路由的一部分

Go 1.22 起，模式可以把方法写进去：

```go
mux.HandleFunc("GET /tasks/{id}", getTask)
mux.HandleFunc("POST /tasks", createTask)
```

`GET /tasks/42` 命中第一个，`POST /tasks` 命中第二个。那 `DELETE /tasks/42` 呢？路径 `/tasks/{id}` 是命中的，但方法对不上，Mux 返回 405，并通过 `Allow` 头告知允许的方法；`GET /unknown/404` 则没有任何模式匹配，返回 404。

为什么要把方法放进路由，而不是在 Handler 里 `if r.Method != "GET"`？因为让 Mux 更早拒绝不匹配请求，Handler 就能只处理自己该处理的事；同时 404 与 405 的语义也自动正确了——路径存在而方法不对，本就该是 405。

路径变量 `{id}` 只匹配一个路径段，`{rest...}` 才匹配多级。取出来的 `r.PathValue("id")` 是客户端输入，不是可信标识；路由匹配不能替代认证与授权。

### 3. Handler 管一次请求：不要把请求状态放进字段

`Handler` 的接口只有一个方法：

```go
type Handler interface {
	ServeHTTP(w ResponseWriter, r *Request)
}
```

它只表达"收到请求、写响应"，不表达监听和连接回收。为什么这点关键？因为**同一个 Handler 实例会被并发调用**。把标题、用户身份、临时结果存成 Handler 的字段，就会在多个请求之间串扰，还产生数据竞争。请求数据应放在局部变量或 `r.Context()` 里：

```go
func bad(w http.ResponseWriter, r *http.Request) {
	h.user = r.Header.Get("User") // 错：并发请求互相覆盖
}

func good(w http.ResponseWriter, r *http.Request) {
	user := r.Header.Get("User") // 对：每次请求一份局部变量
}
```

这也解释了为什么 Handler 可以单独测试：用 `httptest.NewRequest` 造请求、`httptest.NewRecorder` 接响应，直接调 `ServeHTTP`，不需要监听端口、不需要网络。demo 正是这么做的。

### 4. 谁拥有资源谁关闭：超时封不住业务

标准 `net/http` Server 通常会在 Handler 返回后关闭请求体，但 demo 仍显式写 `defer r.Body.Close()`。为什么？因为显式释放让责任在代码里可见；当你替换 Server、包装 Body 或换用测试替身时，这份可见性就是安全网。

`WriteTimeout` 尤其容易被误解。它限制的是"写响应"这一步，不会在数据库查询卡住时主动取消查询。要让业务也能退出，必须把 `r.Context()` 的取消信号传到下游——数据库调用、出站 HTTP 请求都要真正监听它。Handler 若启动了后台 Goroutine，还得自己负责停机和等待；Handler 返回不代表后台任务结束。

## 动手验证

:::demo 15-http-boundary|用 httptest 跑五种请求并验证 Body 关闭
先预测：`DELETE /tasks/42` 和 `GET /unknown/404` 分别返回什么？为什么一个 405 一个 404？POST 的 Body 是谁关掉的？写下答案再运行。
验证：`cd examples/minimal/15-http-boundary && go run .`。输出列出五行状态码：`GET /healthz`、`GET /tasks/42` 为 200，`POST /tasks` 为 201，`DELETE /tasks/42` 为 405，`GET /unknown/404` 为 404。最后一行 `body closed = true` 证明 `defer r.Body.Close()` 真的执行了。把 `taskID` 改成别的值，`GET` 与 `DELETE` 的状态码不变——因为路径结构没变；把 `unknownID` 所在的路径改成 `/tasks/xxx`，404 就会变成 200，说明"路径是否存在"由 Mux 的结构决定，与 ID 的业务含义无关。
:::

demo 用 `httptest.NewRecorder` 而不是 `httptest.NewServer`，所以不需要端口、不会起真实网络服务。这正是"Server 与 Handler 可以分开验证"的直接体现。

## 容易误判的地方

- **"路径注册了就该返回 200 或 404"** → 方法也是路由的一部分。路径命中、方法不支持是 405；只有没有任何模式命中才是 404。
- **"`WriteTimeout` 能中断卡住的 Handler"** → 它只管写响应阶段。业务卡住要靠 `r.Context()` 把取消信号传到下游，且下游必须真的监听它。
- **"标准库会关 Body，我不用写 Close"** → 常见路径下确实会关，但显式 `defer` 让所有权在代码里可见，换掉 Server 或包装 Body 时不会悄悄失效。

## 自己再做一次

1. 注册一条 `HEAD /tasks/{id}`，然后请求 `GET /tasks/42`。**预期结果**：仍返回 200，因为 `GET` 模式匹配 GET 与 HEAD，新增更具体的 HEAD 模式不影响 GET；请求 HEAD 时才走新 handler。
2. 把 `GET /tasks/{id}` 改成 `GET /tasks/{rest...}`，请求 `/tasks/a/b`。**预期结果**：返回 200，且 `r.PathValue("rest")` 得到 `a/b`，证明多段通配能跨越路径分隔符。
3. 删掉 `createTask` 里的 `defer r.Body.Close()`，重新运行。**预期结果**：最后一行变成 `body closed = false`，状态码仍是 201——说明关闭动作来自你的代码，而不影响响应本身。

> 下一篇：[事务的边界](16-database-transaction.md)
