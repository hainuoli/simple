# httpx：请求绑定与响应

`pkg/httpx` 负责两件事：把 HTTP 请求绑定到打了标签的结构体，以及输出统一的 JSON 响应。它不包含业务判断，也不决定错误码，那部分属于 `pkg/errorx`。

## 1. 设计原则

- 显式绑定：字段只有带上对应来源的标签才会被填充，不做隐式猜测。
- 顺序固定：JSON、表单、查询参数、Header、路径依次解析，后面的来源覆盖前面的同名字段。
- 错误可定位：绑定失败会带上是哪个来源、哪个字段，例如 `query "page": ...`。
- 响应统一：成功和失败都使用同一个信封，客户端只需要一套解析逻辑。

## 2. 请求绑定

### 2.1 Parse

`Parse(r, v)` 是入口，按固定顺序执行：

| 顺序 | 函数 | 触发条件 | 使用的标签 |
| --- | --- | --- | --- |
| 1 | `ParseJSON` | `Content-Type` 为 `application/json` 或以 `+json` 结尾 | `json` |
| 2 | `ParseForm` | 总是执行（内部调用 `r.ParseForm`） | `form` |
| 3 | `ParseQuery` | 总是执行 | `query` |
| 4 | `ParseHeaders` | 总是执行 | `header` |
| 5 | `ParsePath` | 总是执行 | `path` |

路径最后解析，所以同名路径参数会覆盖 body 中的值，避免客户端伪造 ID。任意一步返回错误，`Parse` 立即停止并返回该错误。

`ParseForm` 绑定的是 `r.Form`，它同时包含 URL 查询参数和表单 body，因此 `form` 标签也能读到查询参数。需要严格区分来源时，使用 `query` 标签。

### 2.2 字段规则

- `v` 必须是非 nil 的结构体指针，否则返回 `httpx: expected a non-nil pointer to struct`。
- 只处理可导出、可设置的字段；未导出字段被忽略。
- 标签为空或为 `-` 的字段被跳过。
- 请求中没有对应值时，字段保持原值，不会被清零。
- 标量字段取第一个值；切片字段收集全部值，并按元素类型转换。
- 支持 `string`、`bool`、所有 `int`/`uint` 宽度、`float32`、`float64`。
- 其他类型（包括指针和结构体）会返回 `unsupported type` 错误。

### 2.3 JSON 限制

- body 上限为 8 MiB，超出返回 `JSON body exceeds ... bytes`。
- 空 body 合法，视为没有输入。
- 只有空白字符的 body 会返回解码错误。
- 多个 JSON 值（例如 `{"a":1}{"b":2}`）会返回解码错误，防止忽略多余数据。
- `Content-Type` 不是 JSON 时完全跳过 JSON 解析，字段保持原值。

### 2.4 示例

```go
type UpdateUserReq struct {
	ID    int64    `json:"id" path:"id"`
	Name  string   `json:"name" form:"name"`
	Trace string   `header:"X-Trace-ID"`
	Tags  []string `query:"tag"`
}

func UpdateUser(svc *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req UpdateUserReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.Err(w, errorx.InvalidParam(err.Error()))
			return
		}
		// ...
	}
}
```

路径参数依赖 Go 1.22 起的 `ServeMux` 模式，例如 `GET /users/{id}`，并通过 `r.PathValue("id")` 取值。

## 3. 响应

### 3.1 信封

```go
type Body struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}
```

`Data` 为 nil 时不出现在 JSON 中，成功但无返回值的接口只输出 `code` 和 `msg`。

### 3.2 成功与失败

```go
httpx.Ok(w, user)              // 200, code 0, msg "ok"
httpx.Err(w, errorx.NotFound("user not found")) // 404, code 40400
```

`Err` 的映射规则：

| 输入 | 响应 |
| --- | --- |
| `*errorx.Error` | 使用它自己的 `Status`、`Code`、`Msg` |
| 5xx 的 `*errorx.Error` 且带 cause | 先记录 `slog.Error`，再返回业务响应 |
| 其他 error | 记录日志，返回 500 + `50000` + `internal server error` |
| `nil` | 返回 500 + `50000` + `internal server error` |

未知错误和 `nil` 都返回通用文案，客户端不会看到数据库地址、SQL 等内部信息。

### 3.3 JSON

`JSON(w, status, body)` 设置 `Content-Type: application/json; charset=utf-8`、写入状态码并编码 body。由于状态行已经发送，编码失败不会改变响应，也不会额外报错。

## 4. 注意事项

- 绑定标签必须显式写出；没有 `json` 标签的字段不参与 JSON 解码，没有 `query` 标签的字段不读取查询参数。
- `ParseForm` 会自动调用 `r.ParseForm`，multipart 表单需要业务自行处理。
- `Parse` 对同一字段可能写入多次（例如 JSON 后又匹配到路径），最终值取决于表格中的顺序。
- 响应统一走 `Ok` / `Err`，不要在 handler 里直接调用 `http.Error`，否则会破坏 JSON 信封。

## 5. 测试

`pkg/httpx` 覆盖了多来源绑定、媒体类型判断、路径覆盖 JSON、各来源的解析错误与类型转换错误、切片原子赋值、8 MiB 上限、空 body、未知错误不泄漏等行为。函数覆盖率为 100%，包含在 CI 的 `pkg/` 覆盖门禁中。
