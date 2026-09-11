# 🐷 小小存钱罐
![preview](preview.png)

给孩子的存钱工具:自定义月利率,按天复利滚动,日历可视化每天总金额,帮孩子养成存钱习惯。

## 技术栈
- 后端:Go 1.22(标准库 `net/http` + `lib/pq`)
- 数据库:PostgreSQL 16
- 部署:Docker / docker-compose

## 一键启动

确保已安装 Docker,然后在本目录执行:

```bash
docker compose up -d --build
```

浏览器打开 http://localhost:8080 即可使用。

停止:`docker compose down`(数据保留)。彻底清空数据:`docker compose down -v`。

## 功能
- 多个孩子各自独立账户、利率和余额
- 存入 / 取出,自动记录明细
- 按天复利:月利率 10% = 每 30 天 ×1.1,折算等价日利率每天滚动,中途存取都公平
- **日历视图**:点开任意一天看当天总金额,绿色越深金额越高,直观感受"钱在长大"

## 利息算法
日利率因子 `f = (1 + 月利率/100) ^ (1/30)`。
任意时刻余额 = 把每笔交易按发生时间排序,逐笔之间用 `f^经过天数` 增长,再加/减交易额。
已验证:存入 100,30 天后 = 110.00,60 天后 = 121.00。

## API
| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/accounts` | 账户列表(含当前余额) |
| POST | `/api/accounts` | 新建账户 `{name, monthly_rate}` |
| DELETE | `/api/accounts/{id}` | 删除账户 |
| PUT | `/api/accounts/{id}/rate` | 修改利率 `{monthly_rate}` |
| GET | `/api/accounts/{id}/tx` | 交易明细 |
| POST | `/api/accounts/{id}/tx` | 新增交易 `{type:deposit/withdraw, amount}` |
| GET | `/api/accounts/{id}/daily?from=&to=` | 每日余额(日历用) |

## 本地开发(不用 Docker)
需本机有 Go 和 Postgres:
```bash
cd backend
export DATABASE_URL="postgres://kids:kids@localhost:5432/kidsbank?sslmode=disable"
go run .          # 启动服务
go test ./...     # 运行复利测试
```
