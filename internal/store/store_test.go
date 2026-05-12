package store

import (
	"context"
	"path/filepath"
	"testing"

	"pre-ipo-market-platform/internal/domain"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
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
	return s
}

func TestStoreCreatesOrdersAndSubscriptions(t *testing.T) {
	s := testStore(t)
	user, err := s.Authenticate("investor@demo.local", "demo123")
	if err != nil {
		t.Fatalf("authenticate investor: %v", err)
	}
	if err := s.CreateBuyInterest(context.Background(), user.ID, 1, 75000, 43.5); err != nil {
		t.Fatalf("create buy interest: %v", err)
	}
	interests, err := s.BuyInterests(user)
	if err != nil {
		t.Fatalf("list buy interests: %v", err)
	}
	if len(interests) < 2 {
		t.Fatalf("expected seeded and created buy interests, got %d", len(interests))
	}

	if err := s.CreateSubscription(context.Background(), user.ID, 1, 1000); err == nil {
		t.Fatal("subscription below minimum should fail")
	}
	if err := s.CreateSubscription(context.Background(), user.ID, 1, 30000); err != nil {
		t.Fatalf("create subscription: %v", err)
	}
	subscriptions, err := s.Subscriptions(user)
	if err != nil {
		t.Fatalf("list subscriptions: %v", err)
	}
	if len(subscriptions) < 2 {
		t.Fatalf("expected seeded and created subscriptions, got %d", len(subscriptions))
	}
}

func TestAdvanceTransactionCreatesSettledHolding(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin@demo.local", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	for i := 0; i < 4; i++ {
		if err := s.AdvanceTransaction(context.Background(), admin.ID, 1); err != nil {
			t.Fatalf("advance transaction step %d: %v", i, err)
		}
	}
	investor := domain.User{ID: 2}
	holdings, err := s.Holdings(investor.ID)
	if err != nil {
		t.Fatalf("holdings: %v", err)
	}
	var settled bool
	for _, holding := range holdings {
		if holding.CompanyName == "NeuralBridge AI" && holding.Status == string(domain.StageSettled) {
			settled = true
		}
	}
	if !settled {
		t.Fatal("expected settled holding after transaction completion")
	}
}

func TestCreateMatchedTransaction(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin@demo.local", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}

	if err := s.CreateMatchedTransaction(context.Background(), admin.ID, 1, 1, 500, 42); err != nil {
		t.Fatalf("create match: %v", err)
	}
	transactions, err := s.Transactions(admin)
	if err != nil {
		t.Fatalf("transactions: %v", err)
	}
	var found bool
	for _, tx := range transactions {
		if tx.Shares == 500 && tx.Price == 42 && tx.Stage == domain.StageMatched {
			found = true
		}
	}
	if !found {
		t.Fatal("expected matched transaction")
	}
	if err := s.CreateMatchedTransaction(context.Background(), admin.ID, 1, 1, 500, 42); err == nil {
		t.Fatal("matched order and interest should not be matched again")
	}
}

func TestCreateCompanyDealAndSupportTicket(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin@demo.local", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	if err := s.CreateCompany(context.Background(), admin.ID, domain.Company{
		Name:                 "Atlas Robotics",
		Industry:             "Automation",
		Valuation:            "$1.4B",
		FundingRound:         "Series C",
		SharePrice:           22.5,
		Description:          "Robotics company",
		TradableStatus:       "tradable",
		TransferRestrictions: "ROFR",
	}); err != nil {
		t.Fatalf("create company: %v", err)
	}
	companies, err := s.Companies()
	if err != nil {
		t.Fatalf("companies: %v", err)
	}
	newCompany := companies[len(companies)-1]
	if err := s.CreateDeal(context.Background(), admin.ID, domain.Deal{
		CompanyID:       newCompany.ID,
		Name:            "Atlas SPV I",
		DealType:        "spv",
		Structure:       "Single-company SPV",
		MinSubscription: 25000,
		TargetSize:      1000000,
		FeeDescription:  "2% management fee",
	}); err != nil {
		t.Fatalf("create deal: %v", err)
	}
	spvs, err := s.SPVVehicles()
	if err != nil {
		t.Fatalf("spv vehicles: %v", err)
	}
	if len(spvs) < 3 {
		t.Fatalf("expected new SPV vehicle, got %d", len(spvs))
	}
	investor, err := s.Authenticate("investor@demo.local", "demo123")
	if err != nil {
		t.Fatalf("authenticate investor: %v", err)
	}
	if err := s.CreateSupportTicket(context.Background(), investor.ID, "Need report", "Please upload Q2 report"); err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	tickets, err := s.SupportTickets(investor.ID, false)
	if err != nil {
		t.Fatalf("tickets: %v", err)
	}
	if len(tickets) < 2 {
		t.Fatalf("expected seeded and created tickets, got %d", len(tickets))
	}
}

func TestPostInvestmentAndOpsWorkflows(t *testing.T) {
	s := testStore(t)
	admin, err := s.Authenticate("admin@demo.local", "demo123")
	if err != nil {
		t.Fatalf("authenticate admin: %v", err)
	}
	if err := s.CreateValuation(context.Background(), admin.ID, 1, "$5.0B", 45, "2026-06-30"); err != nil {
		t.Fatalf("create valuation: %v", err)
	}
	valuations, err := s.Valuations()
	if err != nil {
		t.Fatalf("valuations: %v", err)
	}
	if valuations[0].Valuation != "$5.0B" {
		t.Fatalf("latest valuation got %s", valuations[0].Valuation)
	}
	if err := s.CreateExitEvent(context.Background(), admin.ID, domain.ExitEvent{CompanyID: 1, EventType: "Tender offer", Description: "Company sponsored window", Status: "confirmed", ExpectedDate: "2026-Q4"}); err != nil {
		t.Fatalf("create exit event: %v", err)
	}
	if err := s.CreateDistribution(context.Background(), admin.ID, domain.Distribution{UserID: 2, Amount: 1200, Status: "pending", TaxDocument: "K-1 draft"}); err != nil {
		t.Fatalf("create distribution: %v", err)
	}
	if err := s.CreateReport(context.Background(), admin.ID, domain.InvestorReport{UserID: 2, ReportType: "portfolio", Title: "Q2 Report", Period: "2026-Q2", Status: "available"}); err != nil {
		t.Fatalf("create report: %v", err)
	}
	if err := s.CreateRiskAlert(context.Background(), admin.ID, domain.RiskAlert{Severity: "high", Status: "open", Subject: "Concentration limit", Note: "Review exposure"}); err != nil {
		t.Fatalf("create risk alert: %v", err)
	}
	alerts, err := s.RiskAlerts()
	if err != nil {
		t.Fatalf("risk alerts: %v", err)
	}
	if err := s.ResolveRiskAlert(context.Background(), admin.ID, alerts[0].ID); err != nil {
		t.Fatalf("resolve alert: %v", err)
	}
	if err := s.CloseSupportTicket(context.Background(), admin.ID, 1); err != nil {
		t.Fatalf("close ticket: %v", err)
	}
}
