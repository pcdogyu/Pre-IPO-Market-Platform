package http

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"pre-ipo-market-platform/internal/store"
)

func testApp(t *testing.T) http.Handler {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "http.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := s.SeedDemoData(); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return NewServer(s).Routes()
}

func loginCookie(t *testing.T, app http.Handler, email string) *http.Cookie {
	t.Helper()
	form := url.Values{"email": {email}, "password": {"demo123"}}
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("login status got %d, want %d", rec.Code, http.StatusSeeOther)
	}
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "preipo_session" {
			return cookie
		}
	}
	t.Fatal("missing session cookie")
	return nil
}

func TestInvestorCanSubmitBuyInterest(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "investor")
	form := url.Values{"company_id": {"1"}, "amount": {"60000"}, "target_price": {"42.5"}}
	req := httptest.NewRequest(http.MethodPost, "/orders/buy-interest", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("submit status got %d, want %d", rec.Code, http.StatusSeeOther)
	}
}

func TestSellerCanSubmitSellOrder(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "seller")
	form := url.Values{"company_id": {"1"}, "shares": {"500"}, "target_price": {"44"}}
	req := httptest.NewRequest(http.MethodPost, "/orders/sell", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("submit status got %d, want %d", rec.Code, http.StatusSeeOther)
	}
}

func TestUsersCanCancelOpenOrders(t *testing.T) {
	app := testApp(t)
	investorCookie := loginCookie(t, app, "investor")
	form := url.Values{"interest_id": {"1"}}
	req := httptest.NewRequest(http.MethodPost, "/orders/buy-interest/cancel", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(investorCookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("cancel buy interest status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	sellerCookie := loginCookie(t, app, "seller")
	form = url.Values{"order_id": {"1"}}
	req = httptest.NewRequest(http.MethodPost, "/orders/sell/cancel", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sellerCookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("cancel sell order status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	req = httptest.NewRequest(http.MethodGet, "/market/orders", nil)
	req.AddCookie(sellerCookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("market status got %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "已取消") {
		t.Fatal("market should render cancelled order status")
	}
}

func TestInvestorCannotAccessAdmin(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "investor")
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin status got %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestUserCanSubmitComplianceReviewAndAdminResolve(t *testing.T) {
	app := testApp(t)
	userCookie := loginCookie(t, app, "pending")
	form := url.Values{"review_type": {"all"}, "note": {"已更新合规文件包"}}
	req := httptest.NewRequest(http.MethodPost, "/compliance/reviews/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(userCookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create compliance review status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	adminCookie := loginCookie(t, app, "admin")
	req = httptest.NewRequest(http.MethodPost, "/admin/compliance-reviews/1/approve", nil)
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("approve compliance review status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	userCookie = loginCookie(t, app, "pending")
	req = httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.AddCookie(userCookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard status got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "合规复核申请") {
		t.Fatal("dashboard should render compliance reviews")
	}
	if !strings.Contains(body, "已通过") {
		t.Fatal("dashboard should show approved compliance review")
	}
}

func TestInvestorCanSubscribeToDeal(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "investor")
	form := url.Values{"amount": {"30000"}}
	req := httptest.NewRequest(http.MethodPost, "/deals/1/subscribe", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("subscribe status got %d, want %d", rec.Code, http.StatusSeeOther)
	}
}

func TestDealsPageRendersPopularUSSPVProjects(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "investor")
	req := httptest.NewRequest(http.MethodGet, "/deals", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("deals status got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{"SPV 项目介绍", "SpaceX 星链与航天专项 SPV", "OpenAI 基础模型专项 SPV", "美国市场高关注未上市公司", "专项载体 · 开放", "基金组合 · 开放", "直接二级转让 · 开放", "EN:", "下一页"} {
		if !strings.Contains(body, want) {
			t.Fatalf("deals page should render %q", want)
		}
	}
	if cardCount := strings.Count(body, `<article class="card">`); cardCount != 9 {
		t.Fatalf("deals page card count got %d, want 9", cardCount)
	}
}

func TestAdminCanCreateMatch(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "admin")
	form := url.Values{"sell_order_id": {"1"}, "buy_interest_id": {"1"}, "shares": {"500"}, "price": {"42"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/matches/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("match status got %d, want %d", rec.Code, http.StatusSeeOther)
	}
}

func TestAdminCanCreateCompanyAndDeal(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "admin")

	companyForm := url.Values{
		"name":                  {"阿特拉斯机器人"},
		"industry":              {"自动化"},
		"valuation":             {"$1.4B"},
		"funding_round":         {"C 轮"},
		"share_price":           {"22.5"},
		"tradable_status":       {"tradable"},
		"transfer_restrictions": {"ROFR"},
		"description":           {"机器人公司"},
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/companies/create", strings.NewReader(companyForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("company status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	dealForm := url.Values{
		"company_id":       {"1"},
		"name":             {"新增专项载体"},
		"deal_type":        {"spv"},
		"structure":        {"单一公司专项载体"},
		"min_subscription": {"25000"},
		"target_size":      {"1000000"},
		"fee_description":  {"2% 管理费"},
	}
	req = httptest.NewRequest(http.MethodPost, "/admin/deals/create", strings.NewReader(dealForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("deal status got %d, want %d", rec.Code, http.StatusSeeOther)
	}
}

func TestAdminCanUpdateDealStatus(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "admin")
	form := url.Values{"deal_id": {"1"}, "status": {"closed"}, "note": {"项目容量复核"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/deals/status", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("deal status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin status got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "项目流水线") {
		t.Fatal("admin should render deal pipeline")
	}
	if !strings.Contains(body, "项目容量复核") {
		t.Fatal("admin audit log should render deal status note")
	}
}

func TestAdminCanUpdateUserRiskRating(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "admin")
	form := url.Values{"user_id": {"2"}, "risk_rating": {"high"}, "note": {"年度适当性复核"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/users/risk-rating", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("risk rating status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin status got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "用户风险评级") {
		t.Fatal("admin should render user risk ratings section")
	}
	if !strings.Contains(body, "年度适当性复核") {
		t.Fatal("admin audit log should render risk rating review note")
	}
}

func TestUserCanManageWatchlist(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "investor")
	form := url.Values{"company_id": {"3"}}
	req := httptest.NewRequest(http.MethodPost, "/watchlist/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("add watchlist status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	req = httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard status got %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "量子支付") {
		t.Fatal("dashboard should render watched company")
	}

	req = httptest.NewRequest(http.MethodPost, "/watchlist/remove", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("remove watchlist status got %d, want %d", rec.Code, http.StatusSeeOther)
	}
}

func TestCompanyPageRendersMarketValueAndSharePriceCharts(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "investor")
	req := httptest.NewRequest(http.MethodGet, "/companies/94", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("company status got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{"总市值历史", "每股价格历史", "chart-grid", "chart-y-label", "chart-x-label"} {
		if !strings.Contains(body, want) {
			t.Fatalf("company page should render %q", want)
		}
	}
	if tickCount := strings.Count(body, `class="chart-x-label"`); tickCount < 12 {
		t.Fatalf("chart x-axis labels got %d, want at least 12", tickCount)
	}
}

func TestCompanyPageLinksToScopedMarketAndDeals(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "investor")
	req := httptest.NewRequest(http.MethodGet, "/companies/1", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("company status got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{`href="/market/orders?company_id=1"`, `href="/deals?company_id=1"`, `class="action-button active"`, "★", "取消关注"} {
		if !strings.Contains(body, want) {
			t.Fatalf("company page should render scoped action %q", want)
		}
	}
}

func TestMarketAndDealsCanFilterByCompany(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "investor")
	req := httptest.NewRequest(http.MethodGet, "/market/orders?company_id=1", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("market status got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{"神经桥智能 · 买卖撮合市场", `option value="1" selected`} {
		if !strings.Contains(body, want) {
			t.Fatalf("market page should render selected company context %q", want)
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/deals?company_id=1", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("deals status got %d, want %d", rec.Code, http.StatusOK)
	}
	body = rec.Body.String()
	if !strings.Contains(body, "神经桥智能 · 项目载体认购") || !strings.Contains(body, "神经桥成长组合") {
		t.Fatal("deals page should render selected company deal context")
	}
	if strings.Contains(body, "赫利欧电网专项载体一期") {
		t.Fatal("deals page should not render other company deals when filtered")
	}
}

func TestCompanyPageRendersFinancialReports(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "investor")
	req := httptest.NewRequest(http.MethodGet, "/companies/1", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("company status got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{"公司财报历史", "年报", "季报", "净利润", "现金"} {
		if !strings.Contains(body, want) {
			t.Fatalf("company page should render %q", want)
		}
	}
}

func TestAssetInformationIntentAndLiquidityHTTP(t *testing.T) {
	app := testApp(t)
	investorCookie := loginCookie(t, app, "investor")

	req := httptest.NewRequest(http.MethodGet, "/companies?q=%E7%A5%9E%E7%BB%8F&sort=heat", nil)
	req.AddCookie(investorCookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("companies status got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{"热度分", "数据可信度", "登记意向", "神经桥智能"} {
		if !strings.Contains(body, want) {
			t.Fatalf("companies page should render %q", want)
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/companies/1", nil)
	req.AddCookie(investorCookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("company status got %d, want %d", rec.Code, http.StatusOK)
	}
	body = rec.Body.String()
	for _, want := range []string{"融资历史", "关键风险", "潜在 IPO 进展", "主要投资人", `action="/intents/create"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("company detail should render %q", want)
		}
	}

	intentForm := url.Values{
		"company_id":         {"1"},
		"focus":              {"人工智能基础设施"},
		"amount":             {"120000"},
		"min_ticket":         {"50000"},
		"lockup":             {"12个月"},
		"product_preference": {"SPV 份额"},
		"accept_structures":  {"接受 SPV 和收益权"},
		"kyc_willing":        {"yes"},
	}
	req = httptest.NewRequest(http.MethodPost, "/intents/create", strings.NewReader(intentForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(investorCookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("intent submit status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	liquidityForm := url.Values{
		"company_id":       {"1"},
		"side":             {"buyer_indication"},
		"amount":           {"90000"},
		"share_price_low":  {"36"},
		"share_price_high": {"42"},
		"window":           {"2026-Q3 季度窗口"},
		"note":             {"测试流动性意向"},
	}
	req = httptest.NewRequest(http.MethodPost, "/market/liquidity", strings.NewReader(liquidityForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(investorCookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("liquidity submit status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	adminCookie := loginCookie(t, app, "admin")
	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin status got %d, want %d", rec.Code, http.StatusOK)
	}
	body = rec.Body.String()
	for _, want := range []string{"用户意向池", "有限流动性窗口", "后续评估模块"} {
		if !strings.Contains(body, want) {
			t.Fatalf("admin page should render %q", want)
		}
	}
}

func TestPortfolioRendersValuationSummary(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "investor")
	req := httptest.NewRequest(http.MethodGet, "/portfolio", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("portfolio status got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "组合估值") {
		t.Fatal("portfolio should render valuation summary")
	}
	if !strings.Contains(body, "未实现收益/亏损") {
		t.Fatal("portfolio should render unrealized gain label")
	}
}

func TestAdminCanManagePostInvestmentAndOps(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "admin")

	cases := []struct {
		path string
		form url.Values
	}{
		{"/admin/valuations/create", url.Values{"company_id": {"1"}, "valuation": {"$5.0B"}, "share_price": {"45"}, "as_of_date": {"2026-06-30"}}},
		{"/admin/exits/create", url.Values{"company_id": {"1"}, "event_type": {"要约收购"}, "status": {"confirmed"}, "expected_date": {"2026-Q4"}, "description": {"公司发起的流动性窗口"}}},
		{"/admin/distributions/create", url.Values{"user_id": {"2"}, "amount": {"1200"}, "status": {"pending"}, "tax_document": {"K-1 草稿"}}},
		{"/admin/reports/create", url.Values{"user_id": {"2"}, "report_type": {"portfolio"}, "title": {"二季度组合报告"}, "period": {"2026-Q2"}, "status": {"available"}}},
		{"/admin/risks/create", url.Values{"severity": {"high"}, "subject": {"集中度限制"}, "note": {"复核敞口"}}},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("%s status got %d, want %d", tc.path, rec.Code, http.StatusSeeOther)
		}
	}

	for _, path := range []string{"/admin/risks/1/resolve", "/admin/tickets/1/close"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("%s status got %d, want %d", path, rec.Code, http.StatusSeeOther)
		}
	}
}

func TestAdminCanAdvanceDistribution(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "admin")
	form := url.Values{"user_id": {"2"}, "amount": {"1500"}, "status": {"pending"}, "tax_document": {"K-1 已准备"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/distributions/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create distribution status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/distributions/2/advance", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("advance distribution status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin status got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "分配队列") {
		t.Fatal("admin should render distribution queue")
	}
	if !strings.Contains(body, "已支付") {
		t.Fatal("admin should render paid distribution status")
	}
}

func TestAdminCanAdvanceReport(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "admin")
	form := url.Values{"user_id": {"2"}, "report_type": {"tax"}, "title": {"2026年三季度税务草稿"}, "period": {"2026-Q3"}, "status": {"pending"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/reports/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create report status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/reports/3/advance", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("advance report status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin status got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "报告队列") {
		t.Fatal("admin should render report queue")
	}
	if !strings.Contains(body, "2026年三季度税务草稿") || !strings.Contains(body, "可查看") {
		t.Fatal("admin should render advanced report")
	}
}

func TestAdminCanAddRiskAction(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "admin")
	form := url.Values{"alert_id": {"1"}, "assigned_to": {"1"}, "action": {"assigned"}, "note": {"分配负责人"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/risks/actions/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("risk action status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin status got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "风险处理记录") {
		t.Fatal("admin should render risk actions section")
	}
	if !strings.Contains(body, "分配负责人") {
		t.Fatal("admin should render submitted risk action note")
	}
}

func TestUserAndAdminCanReplySupportTicket(t *testing.T) {
	app := testApp(t)
	investorCookie := loginCookie(t, app, "investor")
	form := url.Values{"ticket_id": {"1"}, "message": {"投资人补充问题"}}
	req := httptest.NewRequest(http.MethodPost, "/support/tickets/reply", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(investorCookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("investor reply status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	adminCookie := loginCookie(t, app, "admin")
	form = url.Values{"ticket_id": {"1"}, "message": {"管理员回复内容"}, "redirect": {"/admin"}}
	req = httptest.NewRequest(http.MethodPost, "/support/tickets/reply", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("admin reply status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	req = httptest.NewRequest(http.MethodGet, "/portfolio", nil)
	req.AddCookie(investorCookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("portfolio status got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "管理员回复内容") {
		t.Fatal("portfolio should render admin ticket reply")
	}
	if !strings.Contains(body, "客服工单回复") {
		t.Fatal("portfolio should render support ticket reply notification")
	}
}

func TestAdminCanRejectAndCancel(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "admin")

	for _, path := range []string{
		"/admin/reviews/5/reject",
		"/admin/transactions/1/cancel",
		"/admin/subscriptions/1/cancel",
	} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("%s status got %d, want %d", path, rec.Code, http.StatusSeeOther)
		}
	}
}

func TestUserCanCreateNegotiation(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "investor")
	form := url.Values{"transaction_id": {"1"}, "offer_price": {"41.75"}, "shares": {"800"}, "note": {"买方还价"}, "redirect": {"/market/orders"}}
	req := httptest.NewRequest(http.MethodPost, "/negotiations/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("negotiation status got %d, want %d", rec.Code, http.StatusSeeOther)
	}
}

func TestAdminCanManageExecutionDocuments(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "admin")
	form := url.Values{"transaction_id": {"1"}, "document_type": {"Transfer Instruction"}, "note": {"转让文件包"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/documents/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create document status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/documents/1/advance", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("advance document status got %d, want %d", rec.Code, http.StatusSeeOther)
	}
}

func TestAdminCanManageExecutionApprovals(t *testing.T) {
	app := testApp(t)
	adminCookie := loginCookie(t, app, "admin")
	form := url.Values{"transaction_id": {"1"}, "approval_type": {"company_approval"}, "due_date": {"2026-07-15"}, "note": {"董事会同意申请"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/approvals/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create approval status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/approvals/1/advance", nil)
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("advance approval status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	investorCookie := loginCookie(t, app, "investor")
	req = httptest.NewRequest(http.MethodGet, "/portfolio", nil)
	req.AddCookie(investorCookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("portfolio status got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "优先购买权/公司审批") {
		t.Fatal("portfolio should render execution approvals")
	}
	if !strings.Contains(body, "执行审批已更新") {
		t.Fatal("portfolio should render execution approval notification text")
	}
}

func TestAdminCanManageSubscriptionDocuments(t *testing.T) {
	app := testApp(t)
	adminCookie := loginCookie(t, app, "admin")
	form := url.Values{"subscription_id": {"1"}, "document_type": {"Risk Disclosure"}, "note": {"风险揭示文件包"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/subscription-documents/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create subscription document status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/subscription-documents/1/advance", nil)
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("advance subscription document status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	investorCookie := loginCookie(t, app, "investor")
	req = httptest.NewRequest(http.MethodGet, "/portfolio", nil)
	req.AddCookie(investorCookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("portfolio status got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "风险揭示书") {
		t.Fatal("portfolio should render subscription document")
	}
	if !strings.Contains(body, "认购文件") {
		t.Fatal("portfolio should render subscription document notifications")
	}
}

func TestAdminCanManageEscrowPayments(t *testing.T) {
	app := testApp(t)
	adminCookie := loginCookie(t, app, "admin")
	form := url.Values{
		"transaction_id": {"1"},
		"amount":         {"33600"},
		"status":         {"instruction_sent"},
		"reference":      {"ESCROW-HTTP-001"},
		"note":           {"HTTP 托管测试"},
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/escrow-payments/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create escrow payment status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	req = httptest.NewRequest(http.MethodPost, "/admin/escrow-payments/1/advance", nil)
	req.AddCookie(adminCookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("advance escrow payment status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	investorCookie := loginCookie(t, app, "investor")
	req = httptest.NewRequest(http.MethodGet, "/portfolio", nil)
	req.AddCookie(investorCookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("portfolio status got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "ESCROW-HTTP-001") {
		t.Fatal("portfolio should render escrow payment reference")
	}
	if !strings.Contains(body, "托管付款已更新") {
		t.Fatal("portfolio should render escrow payment notification")
	}
}

func TestUserCanMarkNotificationRead(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "investor")

	req := httptest.NewRequest(http.MethodPost, "/notifications/1/read", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("notification read status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	req = httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard status got %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "通知") {
		t.Fatal("dashboard should render notifications")
	}

	req = httptest.NewRequest(http.MethodPost, "/notifications/read-all", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("notification read all status got %d, want %d", rec.Code, http.StatusSeeOther)
	}
}

func TestCapitalCallRoutes(t *testing.T) {
	app := testApp(t)
	adminCookie := loginCookie(t, app, "admin")
	form := url.Values{"user_id": {"2"}, "deal_id": {"1"}, "amount": {"7500"}, "due_date": {"2026-08-01"}, "notice": {"后续资本调用"}}
	req := httptest.NewRequest(http.MethodPost, "/admin/capital-calls/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create capital call status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	investorCookie := loginCookie(t, app, "investor")
	req = httptest.NewRequest(http.MethodPost, "/capital-calls/1/confirm", nil)
	req.AddCookie(investorCookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("confirm capital call status got %d, want %d", rec.Code, http.StatusSeeOther)
	}
}

func TestAdminCanPublishCompanyUpdate(t *testing.T) {
	app := testApp(t)
	adminCookie := loginCookie(t, app, "admin")
	form := url.Values{
		"company_id":  {"1"},
		"update_type": {"financing"},
		"title":       {"神经桥持有人更新"},
		"body":        {"面向现有持有人的融资进展更新。"},
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/company-updates/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("publish update status got %d, want %d", rec.Code, http.StatusSeeOther)
	}

	investorCookie := loginCookie(t, app, "investor")
	req = httptest.NewRequest(http.MethodGet, "/portfolio", nil)
	req.AddCookie(investorCookie)
	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("portfolio status got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "神经桥持有人更新") {
		t.Fatal("portfolio should render published company update")
	}
	if !strings.Contains(body, "公司更新已发布") {
		t.Fatal("portfolio should render company update notification")
	}
}

func TestAdminPageIncludesUpgradeWindow(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "admin")
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin status got %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{"upgradeOverlay", "系统升级", "30 秒后返回首页", "/admin/upgrade/logs"} {
		if !strings.Contains(body, want) {
			t.Fatalf("admin page should render upgrade window content %q", want)
		}
	}
}

func TestUpgradeLogsRejectsInvalidUnit(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "admin")
	req := httptest.NewRequest(http.MethodGet, "/admin/upgrade/logs?unit=../../bad", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("upgrade logs status got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
