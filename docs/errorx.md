# errorx：业务错误

`pkg/errorx` 定义 logic 与传输层共享的业务错误。它只描述「出了什么业务问题」，不负责写响应，也不知道 HTTP 之外的事情。

## 1. 设计原则

- 单向依赖：`errorx` 不 import `httpx`，由 `httpx.Err` 负责把错误映射成响应。
- 两层信息分离：HTTP 状态描述传输结果，`Code` 描述业务结果；业务码在换传输层时保持不变。
- 保留 cause：内部错误通过 `Unwrap` 暴露原因，`errors.Is` / `errors.As` 继续可用。
- 不泄漏细节：`Internal` 携带的 cause 只用于日志，响应里只返回通用文案。

包内没有全局可变状态，所有错误都是按需构造的值。

## 2. 错误码

| 常量 | 值 | 含义 |
| --- | --- | --- |
| `CodeOK` | `0` | 成功 |
| `CodeInvalidParam` | `40000` | 参数缺失或格式错误 |
| `CodeNotFound` | `40400` | 资源不存在 |
| `CodeConflict` | `40900` | 状态冲突，例如唯一键重复 |
| `CodeInternal` | `50000` | 未预期的内部错误 |

业务码与 HTTP 状态分开：客户端可以只依赖 `code` 做分支，服务端将来调整状态码不会破坏协议。

## 3. 类型与构造器

```go
type Error struct {
	Status int
	Code   int
	Msg    string
	Err    error
}
```

| 构造器 | HTTP 状态 | 业务码 | 用途 |
| --- | --- | --- | --- |
| `NewError(status, code, msg)` | 自定义 | 自定义 | 需要新分类时使用 |
| `InvalidParam(msg)` | 400 | 40000 | 请求参数不合法 |
| `NotFound(msg)` | 404 | 40400 | 资源不存在 |
| `Conflict(msg)` | 409 | 40900 | 唯一键或状态冲突 |
| `Internal(err)` | 500 | 50000 | 包装未预期错误 |

`Error()` 返回 `Msg`；如果存在 cause，则返回 `Msg + ": " + cause`，方便日志定位，但业务响应不会直接使用这个拼接结果。

## 4. 在 logic 中使用

repo 层返回领域错误，logic 负责翻译成业务错误：

```go
func mapUserError(err error) error {
	switch {
	case errors.Is(err, repo.ErrNotFound):
		return errorx.NotFound("user not found")
	case errors.Is(err, repo.ErrEmailExists):
		return errorx.Conflict("email already exists")
	default:
		return errorx.Internal(err)
	}
}
```

handler 不需要判断错误类型，直接交给 `httpx.Err`：

```go
u, err := l.svc.Users.Get(ctx, id)
if err != nil {
	httpx.Err(w, err)
	return
}
```

包装过的错误同样能被识别：

```go
err := fmt.Errorf("create user: %w", errorx.NotFound("user not found"))
var e *errorx.Error
errors.As(err, &e) // true
```

## 5. 与 httpx 的映射

`httpx.Err` 的规则：

| 输入 | 响应 |
| --- | --- |
| `*errorx.Error` | 使用它自己的 `Status`、`Code`、`Msg` |
| 5xx 的 `*errorx.Error` 且带 cause | 先记录 `slog.Error`，再返回业务响应 |
| 其他 error | 记录日志，返回 500 + `50000` + `internal server error` |
| `nil` | 返回 500 + `50000` + `internal server error` |

因此 logic 只需要构造正确的业务错误，不需要关心响应格式。

## 6. 测试

`pkg/errorx` 覆盖构造器的状态码、业务码与消息，以及 `Internal` 的 `errors.Is` / 包装后的 `errors.As` 行为。函数覆盖率为 100%，包含在 CI 的 `pkg/` 覆盖门禁中。
