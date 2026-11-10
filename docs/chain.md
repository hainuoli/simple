# chain：HTTP 中间件链

`pkg/chain` 把一组 `func(http.Handler) http.Handler` 组合成一个 `http.Handler`，替代手写嵌套。实现基于 [alice](https://github.com/justinas/alice)（MIT），增加了「中间件返回 nil 立即 panic」和 `Prepend`。

## 1. 设计原则

- 不可变：`Append`、`Prepend`、`Extend` 都返回新链，原链保持可用，可以被多个路由复用。
- 零值可用：`var c chain.Chain` 就是合法的空链，不需要构造函数。
- 错误尽早暴露：中间件返回 `nil` 是编程错误，在 `Then` 阶段直接 panic，而不是等到请求进来才空指针。
- 无反射、无第三方依赖。

## 2. 核心类型

| 组件 | 作用 |
| --- | --- |
| `Middleware` | `func(http.Handler) http.Handler`，标准中间件签名 |
| `Chain` | 不可变的中间件序列 |
| `New` | 复制传入的中间件，构造链 |
| `Append` | 在末尾追加中间件 |
| `Prepend` | 在开头插入中间件 |
| `Extend` | 在末尾拼接另一条链 |
| `Then` | 把链套在 handler 外层 |
| `ThenFunc` | `Then` 的 `http.HandlerFunc` 版本 |

## 3. 使用

```go
handler := chain.New(middleware.Logger, middleware.Recover).Then(mux)
```

执行顺序与书写顺序一致：

```go
chain.New(m1, m2, m3).Then(h)
```

等价于：

```go
m1(m2(m3(h)))
```

请求先经过 `m1`，最后到达 `h`；响应按相反方向返回。

派生新链不会修改原链：

```go
base := chain.New(middleware.Logger)

// base 仍然是 Logger
public := base.Then(publicMux)

// AUTH 排在 Logger 之后
private := base.Append(middleware.Auth).Then(privateMux)

// REQUESTID 排在 Logger 之前
traced := base.Prepend(middleware.RequestID).Then(tracedMux)
```

`Extend` 用于合并两条链：

```go
std := chain.New(middleware.Logger, middleware.Recover)
api := std.Extend(chain.New(middleware.Auth))
```

## 4. 行为细节

- `Then(nil)` 使用 `http.DefaultServeMux`，与标准库保持一致。
- `ThenFunc` 会识别 typed nil（`var fn http.HandlerFunc`），同样回退到 `http.DefaultServeMux`，不会 panic。
- 任一中间件返回 `nil` 时，`Then` panic，错误信息为 `chain: middleware returned nil handler`。
- `New` 会复制调用方传入的切片，调用方之后修改原切片不会影响已构造的链。
- 链本身没有并发状态，`Then` 可以被多次调用，适合在启动阶段为不同路由生成不同组合。

## 5. 测试

`pkg/chain` 覆盖了执行顺序、链复用、`Append`/`Prepend`/`Extend` 的不可变性、零值空链、切片复制、typed nil、中间件返回 nil 的 panic，以及空链直接透传。函数覆盖率为 100%，包含在 CI 的 `pkg/` 覆盖门禁中。
