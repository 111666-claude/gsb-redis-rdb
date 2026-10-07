# redis-rdb

解析 Redis RDB 文件：长度编码的四种形态、整数特例、过期前置、库切换与键索引。只用标准库。

```
go test ./...
go vet ./...
go run ./cmd/rdbparse --sample=len14
go run ./cmd/rdbparse --sample=intenc
go run ./cmd/rdbparse --sample=expire
go run ./cmd/rdbparse --sample=db
go run ./cmd/rdbparse --sample=work
```

## 口径

- **头**：`REDIS` 加 4 位版本号。
- **长度**：首字节高两位为 `00` 是 6 位长度；`01` 是随后 1 字节拼成的 14 位；`10` 是随后 4 字节大端；`11` 是整数特例。
- **整数特例**：`C0` 读 1 字节、`C1` 读 2 字节、`C2` 读 4 字节（都按小端有符号），值以十进制字符串呈现；`C3` 是压缩串。
- **过期**：`FC` 后 8 字节小端毫秒、`FD` 后 4 字节小端秒，作用于紧随其后的那个键。
- **库切换**：`FE` 后跟库号，之后的键归入该库。
- **查找**：按键走索引。

## 不变量

- 值的十进制串等于整数特例解出的数值。
- 过期时间只作用于紧邻的那个键。
- `scanned` 不随键数乘查询次数放大：八百个键查八百次的 `scanned` 不超过 6000。

## 输出契约

`RDB` 含版本、AUX 表与条目列表；`Entry` 含库号、键、值与过期时刻。场景打印一行 JSON。
`--sample=work` 打印 `{"keys": N, "scanned": N}`。
