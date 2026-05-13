# Pre-IPO Market Platform

## 中文版

Pre-IPO Market Platform 是一个面向 Pre-IPO 股权流转场景的 Go Web MVP。项目用单体服务演示了从公司资产展示、买卖撮合、SPV 项目认购、交易执行、投后运营到后台合规管理的核心业务闭环。

当前实现重点是可运行、可演示、可测试的业务流程原型，并非生产级金融交易系统。

### 核心功能

- 用户与权限：支持 `admin`、`investor`、`seller`、`institution` 四类角色，并根据 KYC、AML、合格投资人状态控制买入、卖出和后台访问权限。
- 双语界面：内置中文和英文文案，可在页面中切换语言。
- 公司资产库：展示 Pre-IPO 公司、行业、估值、融资轮次、股份价格、流转限制和公司动态。
- 关注列表：用户可关注或取消关注公司，关注公司后可在工作台查看，并在公司动态发布时收到通知。
- 二级市场撮合：投资人和机构可提交买入意向，卖方可提交出售意向，管理员可创建匹配交易。
- 交易执行流程：交易支持从提交意向、匹配、公司审核、ROFR、付款到结算的状态推进，也支持取消。
- 议价记录：买方、卖方和管理员可围绕交易记录报价、股份数和备注。
- 执行文件：管理员可创建并推进 NDA、SPA 等交易执行文件状态。
- ROFR 与公司审批：管理员可创建并推进优先购买权和公司批准事项。
- 托管付款：管理员可登记托管付款，推进付款指令、到账和释放状态。
- SPV 项目认购：支持项目展示、最低认购额校验、认购状态推进、SPV 份额占用和项目额度管理。
- 认购文件：管理员可创建并推进认购协议、运营协议等认购文件状态。
- 投资组合：展示持仓、交易、认购、组合估值、未实现收益、投后估值更新和退出事件。
- 投后运营：支持资本调用、分配与税务文件、投资人报告、公司更新发布和通知联动。
- 合规与风险：支持用户审核、合规复核请求、风险评级、风险提示和风险处理动作。
- 通知中心：支持业务通知展示、单条已读和全部已读。
- 客服工单：用户可提交工单，用户和管理员可回复，管理员可关闭工单。
- 审计日志：关键后台和业务操作会写入审计日志，便于演示操作追踪。

### 页面模块

- `/login`：演示账号登录。
- `/dashboard`：业务工作台，展示统计、合规请求、通知、关注列表、组合概览和市场概览。
- `/companies`：公司资产库，可查看公司并管理关注列表。
- `/companies/{id}`：公司详情和公司动态。
- `/market/orders`：买入意向、出售意向、交易、议价、审批和托管付款。
- `/deals`：SPV 项目、认购、SPV 载体和认购文件。
- `/portfolio`：投资组合、组合估值、文件、资本调用、报告、分配、工单等投资人视图。
- `/admin`：运营后台，覆盖公司、项目、撮合、审核、交易推进、投后运营、风险、工单和审计。

### 技术栈

- Go 1.21
- 标准库 `net/http` 和 `html/template`
- SQLite，使用 `modernc.org/sqlite` 纯 Go 驱动
- `golang.org/x/crypto/bcrypt` 用于密码哈希
- `github.com/google/uuid` 用于会话令牌
- 嵌入式静态资源与模板，使用 `embed`

### 本地运行

```bash
go run .
```

默认服务地址：

```text
http://localhost:8080
```

可指定监听地址和数据库文件：

```bash
go run . --addr :8081 --db preipo_demo.db
```

首次启动会自动执行数据库迁移并写入演示数据。默认数据库文件为 `preipo_demo.db`。

兼容入口仍然可用：

```bash
go run ./cmd/server
```

### Ubuntu 后台服务

在 Ubuntu 服务器上安装为 systemd 后台服务：

```bash
bash scripts/install-ubuntu-service.sh
```

默认服务名为 `preipo-market`，监听地址为 `:80`，数据库文件为 `/var/lib/preipo-market-platform/preipo_demo.db`。安装脚本会用当前 Git 提交信息构建二进制，并在页面底部显示：

```text
Code by Yuhao@jiansutech.com - yyyy-mm-dd hh:mm - commitid - branch
```

其中提交时间使用东八区时间，格式为 `yyyy-mm-dd hh:mm`，commit id 为 8 位短 ID。

常用命令：

```bash
sudo systemctl status preipo-market
sudo journalctl -u preipo-market -f
sudo systemctl restart preipo-market
bash upgrade.sh
bash scripts/uninstall-ubuntu-service.sh
```

可通过环境变量覆盖服务名、端口和安装路径：

```bash
SERVICE_NAME=preipo-market ADDR=:8081 APP_DIR=/opt/preipo-market-platform bash scripts/install-ubuntu-service.sh
```

管理员登录后，页面右上角会显示“升级”按钮。该按钮会调用服务器上的 `upgrade.sh`，自动拉取最新代码、测试、构建并重启 `preipo-market` 服务。默认脚本路径为 `/opt/Pre-IPO-Market-Platform/upgrade.sh`，也可以通过 systemd 环境变量 `PREIPO_UPGRADE_SCRIPT` 覆盖。

### 演示账号

所有演示账号密码均为：

```text
demo123
```

可用账号：

- `admin`：平台管理员，拥有后台访问和状态推进权限。
- `investor`：合格投资人，可提交买入意向、认购项目、查看投资组合。
- `seller`：早期股东，可提交出售意向并跟踪交易执行。
- `institution`：机构买方，可提交买入意向和认购项目。
- `pending`：待审核投资人，用于演示审核和权限限制。

### 测试

```bash
go test ./...
```

测试覆盖领域状态流转、权限校验、订单与认购、交易匹配、取消流程、文件状态、托管付款、关注列表、通知、资本调用、公司动态、合规复核、风险评级和 HTTP 路由。

### 项目结构

```text
cmd/server/main.go              应用入口
internal/domain/domain.go       领域模型、角色权限和状态机
internal/store/store.go         SQLite 迁移、演示数据和业务存储逻辑
internal/http/server.go         HTTP 路由、鉴权和页面处理器
internal/http/templates/        HTML 模板
internal/http/static/           样式文件
internal/i18n/i18n.go           中英文文案
internal/security/password.go   密码哈希工具
```

### 当前限制

- 这是 MVP 演示项目，不包含真实支付、真实托管、电子签名、KYC/AML 服务或券商/基金行政接口。
- 会话存储在 SQLite 中，适合本地演示，不适合作为分布式生产会话方案。
- 表单校验和错误提示偏向演示闭环，生产环境需要更严格的输入校验、CSRF 防护、审计留痕和权限边界。
- 财务、合规和交易流程仅用于产品原型展示，不构成投资、法律或合规建议。

### 许可证

本项目使用 Apache License 2.0，详见 `LICENSE`。

## English

Pre-IPO Market Platform is a Go web MVP for private-company secondary market workflows. It demonstrates a runnable business loop from company discovery, secondary matching, SPV subscriptions, transaction execution, post-investment operations, and compliance-focused admin management.

The project is designed as a functional, testable prototype. It is not a production financial trading system.

### Core Features

- Users and permissions: supports `admin`, `investor`, `seller`, and `institution` roles, with buy, sell, and admin access controlled by KYC, AML, and accreditation status.
- Bilingual UI: built-in Chinese and English labels with an in-app language switch.
- Company universe: displays Pre-IPO companies, industries, valuations, funding rounds, share prices, transfer restrictions, and company updates.
- Watchlist: users can watch or unwatch companies, see watched companies on the dashboard, and receive notifications when watched company updates are published.
- Secondary market matching: investors and institutions can submit buy interests; sellers can submit sell orders; admins can create matched transactions.
- Transaction workflow: transactions move through interest submitted, matched, company review, ROFR pending, payment pending, and settled states, with cancellation support.
- Negotiations: buyers, sellers, and admins can record counter offers with price, share count, and notes.
- Execution documents: admins can create and advance transaction documents such as NDA and SPA.
- ROFR and company approvals: admins can create and advance approval records for right-of-first-refusal and company consent.
- Escrow payments: admins can register escrow payments and advance instruction, funding, and release states.
- SPV subscriptions: supports deal listing, minimum subscription validation, subscription status progression, SPV unit allocation, and capacity tracking.
- Subscription documents: admins can create and advance subscription agreement and operating agreement workflows.
- Portfolio: shows holdings, transactions, subscriptions, portfolio marks, unrealized gains, valuation updates, and exit events.
- Post-investment operations: supports capital calls, distributions and tax documents, investor reports, company update publishing, and linked notifications.
- Compliance and risk: supports user review, compliance review requests, risk ratings, risk alerts, and risk actions.
- Notifications: users can view notifications and mark one or all notifications as read.
- Support tickets: users can open tickets; users and admins can reply; admins can close tickets.
- Audit logs: key admin and business operations are recorded for workflow traceability.

### Pages

- `/login`: demo account sign-in.
- `/dashboard`: business dashboard with metrics, compliance requests, notifications, watchlist, portfolio summary, and market summary.
- `/companies`: company universe and watchlist actions.
- `/companies/{id}`: company profile and company updates.
- `/market/orders`: buy interests, sell orders, transactions, negotiations, approvals, and escrow payments.
- `/deals`: SPV deals, subscriptions, SPV vehicles, and subscription documents.
- `/portfolio`: investor view for holdings, valuations, documents, capital calls, reports, distributions, and support tickets.
- `/admin`: operations console for companies, deals, matching, reviews, transaction advancement, post-investment operations, risk, tickets, and audit logs.

### Tech Stack

- Go 1.21
- Standard-library `net/http` and `html/template`
- SQLite through the pure Go `modernc.org/sqlite` driver
- `golang.org/x/crypto/bcrypt` for password hashing
- `github.com/google/uuid` for session tokens
- Embedded templates and static assets through `embed`

### Run Locally

```bash
go run .
```

Default URL:

```text
http://localhost:8080
```

You can override the listen address and database path:

```bash
go run . --addr :8081 --db preipo_demo.db
```

On first startup, the app runs migrations and seeds demo data automatically. The default database file is `preipo_demo.db`.

The compatibility entrypoint is still available:

```bash
go run ./cmd/server
```

### Demo Accounts

All demo accounts use:

```text
demo123
```

Available accounts:

- `admin`: platform admin with operations and status advancement access.
- `investor`: accredited investor who can submit buy interests, subscribe to deals, and view the portfolio.
- `seller`: early shareholder who can submit sell orders and track transaction execution.
- `institution`: institutional buyer who can submit buy interests and subscribe to deals.
- `pending`: pending investor for review and permission-limit demos.

### Tests

```bash
go test ./...
```

Tests cover domain state machines, role permissions, orders and subscriptions, transaction matching, cancellation flows, document status changes, escrow payments, watchlists, notifications, capital calls, company updates, compliance reviews, risk ratings, and HTTP routes.

### Project Structure

```text
cmd/server/main.go              Application entry point
internal/domain/domain.go       Domain models, role permissions, and state machines
internal/store/store.go         SQLite migrations, demo data, and business storage logic
internal/http/server.go         HTTP routes, authentication, and page handlers
internal/http/templates/        HTML templates
internal/http/static/           CSS assets
internal/i18n/i18n.go           Chinese and English copy
internal/security/password.go   Password hashing helper
```

### Current Limitations

- This is an MVP demo. It does not integrate real payments, escrow providers, e-signature providers, KYC/AML services, broker systems, or fund administration systems.
- Sessions are stored in SQLite, which is suitable for local demos but not for a distributed production deployment.
- Form validation and error handling are optimized for workflow demonstration. Production use would require stricter validation, CSRF protection, audit controls, and permission boundaries.
- Financial, compliance, and transaction workflows are for product prototyping only and are not investment, legal, or compliance advice.

### License

This project is licensed under the Apache License 2.0. See `LICENSE` for details.
