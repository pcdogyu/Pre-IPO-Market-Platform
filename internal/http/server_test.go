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
	cookie := loginCookie(t, app, "investor@demo.local")
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
	cookie := loginCookie(t, app, "seller@demo.local")
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

func TestInvestorCannotAccessAdmin(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "investor@demo.local")
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin status got %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestInvestorCanSubscribeToDeal(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "investor@demo.local")
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

func TestAdminCanCreateMatch(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "admin@demo.local")
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
	cookie := loginCookie(t, app, "admin@demo.local")

	companyForm := url.Values{
		"name":                  {"Atlas Robotics"},
		"industry":              {"Automation"},
		"valuation":             {"$1.4B"},
		"funding_round":         {"Series C"},
		"share_price":           {"22.5"},
		"tradable_status":       {"tradable"},
		"transfer_restrictions": {"ROFR"},
		"description":           {"Robotics company"},
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
		"name":             {"New SPV"},
		"deal_type":        {"spv"},
		"structure":        {"Single company SPV"},
		"min_subscription": {"25000"},
		"target_size":      {"1000000"},
		"fee_description":  {"2% management fee"},
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

func TestAdminCanManagePostInvestmentAndOps(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "admin@demo.local")

	cases := []struct {
		path string
		form url.Values
	}{
		{"/admin/valuations/create", url.Values{"company_id": {"1"}, "valuation": {"$5.0B"}, "share_price": {"45"}, "as_of_date": {"2026-06-30"}}},
		{"/admin/exits/create", url.Values{"company_id": {"1"}, "event_type": {"Tender offer"}, "status": {"confirmed"}, "expected_date": {"2026-Q4"}, "description": {"Company sponsored liquidity"}}},
		{"/admin/distributions/create", url.Values{"user_id": {"2"}, "amount": {"1200"}, "status": {"pending"}, "tax_document": {"K-1 draft"}}},
		{"/admin/reports/create", url.Values{"user_id": {"2"}, "report_type": {"portfolio"}, "title": {"Q2 Report"}, "period": {"2026-Q2"}, "status": {"available"}}},
		{"/admin/risks/create", url.Values{"severity": {"high"}, "subject": {"Concentration limit"}, "note": {"Review exposure"}}},
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

func TestAdminCanRejectAndCancel(t *testing.T) {
	app := testApp(t)
	cookie := loginCookie(t, app, "admin@demo.local")

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
