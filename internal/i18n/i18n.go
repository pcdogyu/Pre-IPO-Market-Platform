package i18n

var messages = map[string]map[string]string{
	"zh": {
		"app.title":        "Pre-IPO 流转平台",
		"nav.dashboard":    "工作台",
		"nav.companies":    "公司资产库",
		"nav.market":       "撮合市场",
		"nav.deals":        "SPV 认购",
		"nav.portfolio":    "投资组合",
		"nav.admin":        "运营后台",
		"nav.logout":       "退出",
		"login.title":      "登录演示账号",
		"login.email":      "登录名",
		"login.password":   "密码",
		"login.submit":     "登录",
		"dashboard.title":  "业务工作台",
		"companies.title":  "Pre-IPO 公司资产库",
		"market.title":     "买卖撮合市场",
		"deals.title":      "SPV 项目认购",
		"portfolio.title":  "投资组合",
		"admin.title":      "运营后台",
		"form.submit":      "提交",
		"form.advance":     "推进状态",
		"form.approve":     "批准",
		"label.company":    "公司",
		"label.status":     "状态",
		"label.amount":     "金额",
		"label.price":      "目标价格",
		"label.shares":     "股份数",
		"label.role":       "角色",
		"label.valuation":  "估值",
		"label.round":      "融资轮次",
		"label.industry":   "行业",
		"label.minimum":    "最低认购额",
		"label.target":     "目标规模",
		"label.fees":       "费用",
		"label.audit":      "审计日志",
		"action.buy":       "提交买入意向",
		"action.sell":      "提交出售意向",
		"action.subscribe": "认购",
		"empty":            "暂无数据",
		"language.switch":  "English",
	},
	"en": {
		"app.title":        "Pre-IPO Market Platform",
		"nav.dashboard":    "Dashboard",
		"nav.companies":    "Companies",
		"nav.market":       "Market",
		"nav.deals":        "SPV Deals",
		"nav.portfolio":    "Portfolio",
		"nav.admin":        "Admin",
		"nav.logout":       "Logout",
		"login.title":      "Sign in with a demo account",
		"login.email":      "Login",
		"login.password":   "Password",
		"login.submit":     "Sign in",
		"dashboard.title":  "Business Dashboard",
		"companies.title":  "Pre-IPO Company Universe",
		"market.title":     "Secondary Matching Market",
		"deals.title":      "SPV Subscriptions",
		"portfolio.title":  "Portfolio",
		"admin.title":      "Operations Admin",
		"form.submit":      "Submit",
		"form.advance":     "Advance",
		"form.approve":     "Approve",
		"label.company":    "Company",
		"label.status":     "Status",
		"label.amount":     "Amount",
		"label.price":      "Target price",
		"label.shares":     "Shares",
		"label.role":       "Role",
		"label.valuation":  "Valuation",
		"label.round":      "Round",
		"label.industry":   "Industry",
		"label.minimum":    "Minimum",
		"label.target":     "Target size",
		"label.fees":       "Fees",
		"label.audit":      "Audit log",
		"action.buy":       "Submit buy interest",
		"action.sell":      "Submit sell order",
		"action.subscribe": "Subscribe",
		"empty":            "No records",
		"language.switch":  "中文",
	},
}

func T(lang, key string) string {
	if lang != "en" {
		lang = "zh"
	}
	if value, ok := messages[lang][key]; ok {
		return value
	}
	if value, ok := messages["zh"][key]; ok {
		return value
	}
	return key
}
