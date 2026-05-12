package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"pre-ipo-market-platform/internal/domain"
	"pre-ipo-market-platform/internal/security"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) Migrate() error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			name TEXT NOT NULL,
			role TEXT NOT NULL,
			language TEXT NOT NULL DEFAULT 'zh',
			kyc_status TEXT NOT NULL,
			aml_status TEXT NOT NULL DEFAULT 'pending_review',
			accreditation_status TEXT NOT NULL,
			risk_rating TEXT NOT NULL DEFAULT 'medium'
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			expires_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS companies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			industry TEXT NOT NULL,
			valuation TEXT NOT NULL,
			funding_round TEXT NOT NULL,
			share_price REAL NOT NULL DEFAULT 0,
			description TEXT NOT NULL,
			tradable_status TEXT NOT NULL,
			transfer_restrictions TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS sell_orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			seller_id INTEGER NOT NULL REFERENCES users(id),
			company_id INTEGER NOT NULL REFERENCES companies(id),
			shares INTEGER NOT NULL,
			target_price REAL NOT NULL,
			status TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS buy_interests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			investor_id INTEGER NOT NULL REFERENCES users(id),
			company_id INTEGER NOT NULL REFERENCES companies(id),
			amount REAL NOT NULL,
			target_price REAL NOT NULL,
			status TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS transactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			buyer_id INTEGER NOT NULL REFERENCES users(id),
			seller_id INTEGER NOT NULL REFERENCES users(id),
			company_id INTEGER NOT NULL REFERENCES companies(id),
			shares INTEGER NOT NULL,
			price REAL NOT NULL,
			stage TEXT NOT NULL,
			document_status TEXT NOT NULL DEFAULT 'not_started',
			rofr_status TEXT NOT NULL DEFAULT 'not_started',
			company_approval_status TEXT NOT NULL DEFAULT 'not_started',
			escrow_status TEXT NOT NULL DEFAULT 'not_started'
		)`,
		`CREATE TABLE IF NOT EXISTS negotiations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			transaction_id INTEGER NOT NULL REFERENCES transactions(id),
			actor_id INTEGER NOT NULL REFERENCES users(id),
			offer_price REAL NOT NULL,
			shares INTEGER NOT NULL,
			note TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS deals (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			company_id INTEGER NOT NULL REFERENCES companies(id),
			name TEXT NOT NULL,
			deal_type TEXT NOT NULL DEFAULT 'spv',
			structure TEXT NOT NULL DEFAULT '',
			min_subscription REAL NOT NULL,
			target_size REAL NOT NULL,
			fee_description TEXT NOT NULL,
			status TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS subscriptions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			investor_id INTEGER NOT NULL REFERENCES users(id),
			deal_id INTEGER NOT NULL REFERENCES deals(id),
			amount REAL NOT NULL,
			status TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS holdings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id),
			company_name TEXT NOT NULL,
			source_type TEXT NOT NULL,
			cost REAL NOT NULL,
			status TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			actor_id INTEGER REFERENCES users(id),
			action TEXT NOT NULL,
			object_type TEXT NOT NULL,
			object_id INTEGER NOT NULL,
			note TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS spv_vehicles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			deal_id INTEGER NOT NULL REFERENCES deals(id),
			name TEXT NOT NULL,
			jurisdiction TEXT NOT NULL,
			manager TEXT NOT NULL,
			share_class TEXT NOT NULL,
			total_units INTEGER NOT NULL,
			issued_units INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS execution_documents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			transaction_id INTEGER NOT NULL REFERENCES transactions(id),
			document_type TEXT NOT NULL,
			status TEXT NOT NULL,
			signed_at TEXT NOT NULL,
			note TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS valuations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			company_id INTEGER NOT NULL REFERENCES companies(id),
			valuation TEXT NOT NULL,
			share_price REAL NOT NULL,
			as_of_date TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS exit_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			company_id INTEGER NOT NULL REFERENCES companies(id),
			event_type TEXT NOT NULL,
			description TEXT NOT NULL,
			status TEXT NOT NULL,
			expected_date TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS distributions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id),
			holding_id INTEGER REFERENCES holdings(id),
			amount REAL NOT NULL,
			status TEXT NOT NULL,
			tax_document TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS capital_calls (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id),
			deal_id INTEGER NOT NULL REFERENCES deals(id),
			amount REAL NOT NULL,
			due_date TEXT NOT NULL,
			status TEXT NOT NULL,
			notice TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS investor_reports (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id),
			report_type TEXT NOT NULL,
			title TEXT NOT NULL,
			period TEXT NOT NULL,
			status TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS notifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id),
			title TEXT NOT NULL,
			body TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS risk_alerts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			severity TEXT NOT NULL,
			status TEXT NOT NULL,
			subject TEXT NOT NULL,
			note TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS support_tickets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id),
			status TEXT NOT NULL,
			subject TEXT NOT NULL,
			note TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
	}
	for _, stmt := range statements {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	for _, migration := range []string{
		`ALTER TABLE users ADD COLUMN aml_status TEXT NOT NULL DEFAULT 'pending_review'`,
		`ALTER TABLE users ADD COLUMN risk_rating TEXT NOT NULL DEFAULT 'medium'`,
		`ALTER TABLE companies ADD COLUMN share_price REAL NOT NULL DEFAULT 0`,
		`ALTER TABLE companies ADD COLUMN transfer_restrictions TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE transactions ADD COLUMN document_status TEXT NOT NULL DEFAULT 'not_started'`,
		`ALTER TABLE transactions ADD COLUMN rofr_status TEXT NOT NULL DEFAULT 'not_started'`,
		`ALTER TABLE transactions ADD COLUMN company_approval_status TEXT NOT NULL DEFAULT 'not_started'`,
		`ALTER TABLE transactions ADD COLUMN escrow_status TEXT NOT NULL DEFAULT 'not_started'`,
		`ALTER TABLE deals ADD COLUMN deal_type TEXT NOT NULL DEFAULT 'spv'`,
		`ALTER TABLE deals ADD COLUMN structure TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err := s.db.Exec(migration); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return err
		}
	}
	return nil
}

func (s *Store) SeedDemoData() error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	hash, err := security.HashPassword("demo123")
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	users := []struct {
		email, name, role, lang, kyc, aml, acc, risk string
	}{
		{"admin@demo.local", "平台管理员", "admin", "zh", "approved", "approved", "approved", "low"},
		{"investor@demo.local", "合格投资人", "investor", "zh", "approved", "approved", "approved", "medium"},
		{"seller@demo.local", "早期股东", "seller", "zh", "approved", "approved", "approved", "medium"},
		{"institution@demo.local", "机构买方", "institution", "en", "approved", "approved", "approved", "low"},
		{"pending@demo.local", "待审核投资人", "investor", "en", "pending_review", "pending_review", "pending_review", "high"},
	}
	for _, u := range users {
		if _, err := tx.Exec(`INSERT INTO users (email, password_hash, name, role, language, kyc_status, aml_status, accreditation_status, risk_rating) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			u.email, hash, u.name, u.role, u.lang, u.kyc, u.aml, u.acc, u.risk); err != nil {
			return err
		}
	}

	companies := []struct {
		name, industry, valuation, round  string
		sharePrice                        float64
		description, status, restrictions string
	}{
		{"NeuralBridge AI", "人工智能基础设施", "$4.8B", "Series D", 42.50, "企业级 AI 工作流平台，近两年收入高速增长。", "tradable", "ROFR + board approval required"},
		{"HelioGrid Energy", "新能源", "$2.1B", "Series C", 18.75, "分布式储能与电网调度软件公司。", "tradable", "Company consent required; 30-day ROFR window"},
		{"QuantumPay", "金融科技", "$6.3B", "Series E", 64.20, "跨境支付和企业财资管理平台。", "limited", "Transfers limited to approved institutional buyers"},
	}
	for _, c := range companies {
		if _, err := tx.Exec(`INSERT INTO companies (name, industry, valuation, funding_round, share_price, description, tradable_status, transfer_restrictions) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			c.name, c.industry, c.valuation, c.round, c.sharePrice, c.description, c.status, c.restrictions); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(`INSERT INTO sell_orders (seller_id, company_id, shares, target_price, status) VALUES (3, 1, 1200, 42.50, 'open')`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO buy_interests (investor_id, company_id, amount, target_price, status) VALUES (2, 1, 50000, 41.00, 'interest_submitted')`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO transactions (buyer_id, seller_id, company_id, shares, price, stage, document_status, rofr_status, company_approval_status, escrow_status) VALUES (2, 3, 1, 800, 42.00, 'matched', 'drafted', 'not_started', 'not_started', 'not_started')`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO negotiations (transaction_id, actor_id, offer_price, shares, note, created_at) VALUES
		(1, 2, 41.50, 800, 'Buyer requests modest discount for ROFR timing risk.', ?),
		(1, 3, 42.00, 800, 'Seller accepts if SPA is signed this week.', ?)`, time.Now().Add(-2*time.Hour).Format(time.RFC3339), time.Now().Add(-1*time.Hour).Format(time.RFC3339)); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO deals (company_id, name, deal_type, structure, min_subscription, target_size, fee_description, status) VALUES
		(2, 'HelioGrid SPV I', 'spv', 'Single-company SPV with quarterly reporting', 25000, 5000000, '2% management fee, 10% carry after hurdle', 'open'),
		(1, 'NeuralBridge Growth Basket', 'fund_basket', 'Multi-company growth basket with pro-rata units', 50000, 8000000, '1.5% annual management fee', 'open'),
		(3, 'QuantumPay Direct Secondary', 'direct_secondary', 'Direct negotiated share transfer for approved buyers', 100000, 3000000, '1% transaction fee', 'open')`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO subscriptions (investor_id, deal_id, amount, status) VALUES (2, 1, 30000, 'admin_confirmed')`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO holdings (user_id, company_name, source_type, cost, status) VALUES
		(2, 'NeuralBridge AI', 'secondary', 33600, 'matched'),
		(2, 'HelioGrid Energy', 'spv', 30000, 'admin_confirmed')`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO spv_vehicles (deal_id, name, jurisdiction, manager, share_class, total_units, issued_units) VALUES
		(1, 'HelioGrid SPV I LLC', 'Delaware', 'PreIPO Demo GP LLC', 'Class A', 500000, 30000),
		(2, 'NeuralBridge Growth Basket LP', 'Cayman Islands', 'PreIPO Demo GP LLC', 'Limited Partner Units', 800000, 0)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO execution_documents (transaction_id, document_type, status, signed_at, note) VALUES
		(1, 'NDA', 'signed', ?, 'Counterparties cleared confidentiality'),
		(1, 'SPA', 'drafted', '', 'Pending ROFR package')`, time.Now().Format("2006-01-02")); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO valuations (company_id, valuation, share_price, as_of_date) VALUES
		(1, '$4.8B', 42.50, '2026-03-31'),
		(2, '$2.1B', 18.75, '2026-03-31'),
		(3, '$6.3B', 64.20, '2026-03-31')`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO exit_events (company_id, event_type, description, status, expected_date) VALUES
		(1, 'IPO readiness', 'Banker bake-off expected after next audit cycle', 'watchlist', '2027-H1'),
		(2, 'Strategic financing', 'Potential strategic round may refresh valuation', 'monitoring', '2026-Q4')`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO distributions (user_id, holding_id, amount, status, tax_document) VALUES
		(2, 2, 0, 'not_due', 'K-1 pending')`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO capital_calls (user_id, deal_id, amount, due_date, status, notice, created_at) VALUES
		(2, 1, 5000, '2026-07-15', 'pending', 'Initial capital call for HelioGrid SPV I.', ?)`, time.Now().Format(time.RFC3339)); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO investor_reports (user_id, report_type, title, period, status) VALUES
		(2, 'portfolio', 'Q1 2026 Portfolio Statement', '2026-Q1', 'available'),
		(2, 'tax', '2025 Tax Package Placeholder', '2025', 'pending')`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO notifications (user_id, title, body, status, created_at) VALUES
		(2, 'Welcome to Pre-IPO MVP', 'Your demo investor account is ready.', 'unread', ?),
		(3, 'Seller workflow ready', 'You can submit sell orders and track execution.', 'unread', ?)`, time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339)); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO risk_alerts (severity, status, subject, note, created_at) VALUES
		('medium', 'open', 'QuantumPay transfer restriction', 'Only approved institutional buyers can be matched.', ?),
		('low', 'monitoring', 'HelioGrid subscription concentration', 'Top investor exposure remains below internal threshold.', ?)`, time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339)); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO support_tickets (user_id, status, subject, note, created_at) VALUES
		(2, 'open', 'Subscription document question', 'Investor asked for SPV operating agreement summary.', ?)`, time.Now().Format(time.RFC3339)); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO audit_logs (actor_id, action, object_type, object_id, note, created_at) VALUES (1, 'seed', 'system', 1, 'demo data initialized', ?)`, time.Now().Format(time.RFC3339)); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) Authenticate(email, password string) (domain.User, error) {
	var user domain.User
	var hash string
	err := s.db.QueryRow(`SELECT id, email, password_hash, name, role, language, kyc_status, aml_status, accreditation_status, risk_rating FROM users WHERE email = ?`, email).
		Scan(&user.ID, &user.Email, &hash, &user.Name, &user.Role, &user.Language, &user.KYCStatus, &user.AMLStatus, &user.AccreditationStatus, &user.RiskRating)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user, errors.New("invalid email or password")
		}
		return user, err
	}
	if !security.CheckPassword(hash, password) {
		return user, errors.New("invalid email or password")
	}
	return user, nil
}

func (s *Store) CreateSession(userID int64, token string, expiresAt time.Time) error {
	_, err := s.db.Exec(`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`, token, userID, expiresAt.Format(time.RFC3339))
	return err
}

func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

func (s *Store) UserBySession(token string) (domain.User, error) {
	var expires string
	var user domain.User
	err := s.db.QueryRow(`SELECT u.id, u.email, u.name, u.role, u.language, u.kyc_status, u.aml_status, u.accreditation_status, u.risk_rating, s.expires_at
		FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.token = ?`, token).
		Scan(&user.ID, &user.Email, &user.Name, &user.Role, &user.Language, &user.KYCStatus, &user.AMLStatus, &user.AccreditationStatus, &user.RiskRating, &expires)
	if err != nil {
		return user, err
	}
	expiresAt, err := time.Parse(time.RFC3339, expires)
	if err != nil || time.Now().After(expiresAt) {
		_ = s.DeleteSession(token)
		return user, sql.ErrNoRows
	}
	return user, nil
}

func (s *Store) SetLanguage(userID int64, lang string) error {
	if lang != "zh" && lang != "en" {
		lang = "zh"
	}
	_, err := s.db.Exec(`UPDATE users SET language = ? WHERE id = ?`, lang, userID)
	return err
}

func (s *Store) UsersPendingReview() ([]domain.User, error) {
	rows, err := s.db.Query(`SELECT id, email, name, role, language, kyc_status, aml_status, accreditation_status, risk_rating FROM users WHERE kyc_status = 'pending_review' OR aml_status = 'pending_review' OR accreditation_status = 'pending_review' ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []domain.User
	for rows.Next() {
		var user domain.User
		if err := rows.Scan(&user.ID, &user.Email, &user.Name, &user.Role, &user.Language, &user.KYCStatus, &user.AMLStatus, &user.AccreditationStatus, &user.RiskRating); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *Store) Users() ([]domain.User, error) {
	rows, err := s.db.Query(`SELECT id, email, name, role, language, kyc_status, aml_status, accreditation_status, risk_rating FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []domain.User
	for rows.Next() {
		var user domain.User
		if err := rows.Scan(&user.ID, &user.Email, &user.Name, &user.Role, &user.Language, &user.KYCStatus, &user.AMLStatus, &user.AccreditationStatus, &user.RiskRating); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *Store) ApproveUser(ctx context.Context, actorID, userID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE users SET kyc_status = 'approved', aml_status = 'approved', accreditation_status = 'approved', risk_rating = 'medium' WHERE id = ?`, userID); err != nil {
		return err
	}
	if err := insertAudit(ctx, tx, actorID, "approve_user", "user", userID, "KYC and accreditation approved"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RejectUser(ctx context.Context, actorID, userID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE users SET kyc_status = 'rejected', aml_status = 'rejected', accreditation_status = 'rejected', risk_rating = 'high' WHERE id = ?`, userID); err != nil {
		return err
	}
	if err := insertAudit(ctx, tx, actorID, "reject_user", "user", userID, "KYC, AML and accreditation rejected"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Companies() ([]domain.Company, error) {
	rows, err := s.db.Query(`SELECT id, name, industry, valuation, funding_round, share_price, description, tradable_status, transfer_restrictions FROM companies ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var companies []domain.Company
	for rows.Next() {
		var company domain.Company
		if err := rows.Scan(&company.ID, &company.Name, &company.Industry, &company.Valuation, &company.FundingRound, &company.SharePrice, &company.Description, &company.TradableStatus, &company.TransferRestrictions); err != nil {
			return nil, err
		}
		companies = append(companies, company)
	}
	return companies, rows.Err()
}

func (s *Store) Company(id int64) (domain.Company, error) {
	var company domain.Company
	err := s.db.QueryRow(`SELECT id, name, industry, valuation, funding_round, share_price, description, tradable_status, transfer_restrictions FROM companies WHERE id = ?`, id).
		Scan(&company.ID, &company.Name, &company.Industry, &company.Valuation, &company.FundingRound, &company.SharePrice, &company.Description, &company.TradableStatus, &company.TransferRestrictions)
	return company, err
}

func (s *Store) CreateCompany(ctx context.Context, actorID int64, company domain.Company) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO companies (name, industry, valuation, funding_round, share_price, description, tradable_status, transfer_restrictions) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		company.Name, company.Industry, company.Valuation, company.FundingRound, company.SharePrice, company.Description, company.TradableStatus, company.TransferRestrictions)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if err := insertAudit(ctx, tx, actorID, "create_company", "company", id, company.Name); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SellOrders(user domain.User) ([]domain.SellOrder, error) {
	query := `SELECT so.id, so.seller_id, u.name, so.company_id, c.name, so.shares, so.target_price, so.status
		FROM sell_orders so JOIN users u ON u.id = so.seller_id JOIN companies c ON c.id = so.company_id`
	var rows *sql.Rows
	var err error
	if user.Role == domain.RoleSeller {
		rows, err = s.db.Query(query+` WHERE so.seller_id = ? ORDER BY so.id DESC`, user.ID)
	} else {
		rows, err = s.db.Query(query + ` ORDER BY so.id DESC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var orders []domain.SellOrder
	for rows.Next() {
		var order domain.SellOrder
		if err := rows.Scan(&order.ID, &order.SellerID, &order.SellerName, &order.CompanyID, &order.CompanyName, &order.Shares, &order.TargetPrice, &order.Status); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (s *Store) BuyInterests(user domain.User) ([]domain.BuyInterest, error) {
	query := `SELECT bi.id, bi.investor_id, u.name, bi.company_id, c.name, bi.amount, bi.target_price, bi.status
		FROM buy_interests bi JOIN users u ON u.id = bi.investor_id JOIN companies c ON c.id = bi.company_id`
	var rows *sql.Rows
	var err error
	if user.Role == domain.RoleInvestor {
		rows, err = s.db.Query(query+` WHERE bi.investor_id = ? ORDER BY bi.id DESC`, user.ID)
	} else {
		rows, err = s.db.Query(query + ` ORDER BY bi.id DESC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var interests []domain.BuyInterest
	for rows.Next() {
		var interest domain.BuyInterest
		if err := rows.Scan(&interest.ID, &interest.InvestorID, &interest.InvestorName, &interest.CompanyID, &interest.CompanyName, &interest.Amount, &interest.TargetPrice, &interest.Status); err != nil {
			return nil, err
		}
		interests = append(interests, interest)
	}
	return interests, rows.Err()
}

func (s *Store) CreateSellOrder(ctx context.Context, sellerID, companyID, shares int64, price float64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO sell_orders (seller_id, company_id, shares, target_price, status) VALUES (?, ?, ?, ?, 'open')`, sellerID, companyID, shares, price)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if err := insertAudit(ctx, tx, sellerID, "create_sell_order", "sell_order", id, "seller submitted shares for sale"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CreateBuyInterest(ctx context.Context, investorID, companyID int64, amount, price float64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO buy_interests (investor_id, company_id, amount, target_price, status) VALUES (?, ?, ?, ?, ?)`, investorID, companyID, amount, price, string(domain.StageInterestSubmitted))
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if err := insertAudit(ctx, tx, investorID, "create_buy_interest", "buy_interest", id, "investor submitted buy interest"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CreateMatchedTransaction(ctx context.Context, actorID, sellOrderID, buyInterestID, shares int64, price float64) error {
	if shares <= 0 || price <= 0 {
		return fmt.Errorf("shares and price must be positive")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var sellerID, sellCompanyID, availableShares int64
	var sellStatus string
	if err := tx.QueryRowContext(ctx, `SELECT seller_id, company_id, shares, status FROM sell_orders WHERE id = ?`, sellOrderID).
		Scan(&sellerID, &sellCompanyID, &availableShares, &sellStatus); err != nil {
		return err
	}

	var buyerID, buyCompanyID int64
	var budget float64
	var buyStatus string
	if err := tx.QueryRowContext(ctx, `SELECT investor_id, company_id, amount, status FROM buy_interests WHERE id = ?`, buyInterestID).
		Scan(&buyerID, &buyCompanyID, &budget, &buyStatus); err != nil {
		return err
	}

	if sellStatus != "open" {
		return fmt.Errorf("sell order is not open")
	}
	if buyStatus != string(domain.StageInterestSubmitted) {
		return fmt.Errorf("buy interest is not open")
	}
	if sellCompanyID != buyCompanyID {
		return fmt.Errorf("sell order and buy interest must reference the same company")
	}
	if shares > availableShares {
		return fmt.Errorf("matched shares exceed available sell order shares")
	}
	if float64(shares)*price > budget {
		return fmt.Errorf("matched notional exceeds buyer interest amount")
	}

	res, err := tx.ExecContext(ctx, `INSERT INTO transactions (buyer_id, seller_id, company_id, shares, price, stage) VALUES (?, ?, ?, ?, ?, ?)`,
		buyerID, sellerID, sellCompanyID, shares, price, string(domain.StageMatched))
	if err != nil {
		return err
	}
	transactionID, _ := res.LastInsertId()
	if _, err := tx.ExecContext(ctx, `UPDATE sell_orders SET status = 'matched' WHERE id = ?`, sellOrderID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE buy_interests SET status = 'matched' WHERE id = ?`, buyInterestID); err != nil {
		return err
	}
	if err := insertAudit(ctx, tx, actorID, "create_match", "transaction", transactionID, fmt.Sprintf("sell_order #%d + buy_interest #%d", sellOrderID, buyInterestID)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Transactions(user domain.User) ([]domain.Transaction, error) {
	query := `SELECT t.id, t.buyer_id, bu.name, t.seller_id, su.name, t.company_id, c.name, t.shares, t.price, t.stage, t.document_status, t.rofr_status, t.company_approval_status, t.escrow_status
		FROM transactions t
		JOIN users bu ON bu.id = t.buyer_id
		JOIN users su ON su.id = t.seller_id
		JOIN companies c ON c.id = t.company_id`
	var rows *sql.Rows
	var err error
	switch user.Role {
	case domain.RoleInvestor:
		rows, err = s.db.Query(query+` WHERE t.buyer_id = ? ORDER BY t.id DESC`, user.ID)
	case domain.RoleSeller:
		rows, err = s.db.Query(query+` WHERE t.seller_id = ? ORDER BY t.id DESC`, user.ID)
	default:
		rows, err = s.db.Query(query + ` ORDER BY t.id DESC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var transactions []domain.Transaction
	for rows.Next() {
		var transaction domain.Transaction
		if err := rows.Scan(&transaction.ID, &transaction.BuyerID, &transaction.BuyerName, &transaction.SellerID, &transaction.SellerName, &transaction.CompanyID, &transaction.CompanyName, &transaction.Shares, &transaction.Price, &transaction.Stage, &transaction.DocumentStatus, &transaction.ROFRStatus, &transaction.CompanyApprovalStatus, &transaction.EscrowStatus); err != nil {
			return nil, err
		}
		transactions = append(transactions, transaction)
	}
	return transactions, rows.Err()
}

func (s *Store) Negotiations(user domain.User) ([]domain.Negotiation, error) {
	query := `SELECT n.id, n.transaction_id, n.actor_id, u.name, u.role, n.offer_price, n.shares, n.note, n.created_at
		FROM negotiations n
		JOIN users u ON u.id = n.actor_id
		JOIN transactions t ON t.id = n.transaction_id`
	var rows *sql.Rows
	var err error
	switch user.Role {
	case domain.RoleInvestor, domain.RoleInstitution:
		rows, err = s.db.Query(query+` WHERE t.buyer_id = ? ORDER BY n.id DESC`, user.ID)
	case domain.RoleSeller:
		rows, err = s.db.Query(query+` WHERE t.seller_id = ? ORDER BY n.id DESC`, user.ID)
	default:
		rows, err = s.db.Query(query + ` ORDER BY n.id DESC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var negotiations []domain.Negotiation
	for rows.Next() {
		var negotiation domain.Negotiation
		if err := rows.Scan(&negotiation.ID, &negotiation.TransactionID, &negotiation.ActorID, &negotiation.ActorName, &negotiation.ActorRole, &negotiation.OfferPrice, &negotiation.Shares, &negotiation.Note, &negotiation.CreatedAt); err != nil {
			return nil, err
		}
		negotiations = append(negotiations, negotiation)
	}
	return negotiations, rows.Err()
}

func (s *Store) CreateNegotiation(ctx context.Context, actor domain.User, transactionID int64, offerPrice float64, shares int64, note string) error {
	if transactionID <= 0 || offerPrice <= 0 || shares <= 0 {
		return fmt.Errorf("transaction, price and shares are required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var buyerID, sellerID int64
	var stage domain.TransactionStage
	if err := tx.QueryRowContext(ctx, `SELECT buyer_id, seller_id, stage FROM transactions WHERE id = ?`, transactionID).Scan(&buyerID, &sellerID, &stage); err != nil {
		return err
	}
	if stage == domain.StageSettled || stage == domain.StageCancelled {
		return fmt.Errorf("transaction is already terminal: %s", stage)
	}
	if actor.Role != domain.RoleAdmin && actor.ID != buyerID && actor.ID != sellerID {
		return fmt.Errorf("actor cannot negotiate this transaction")
	}

	res, err := tx.ExecContext(ctx, `INSERT INTO negotiations (transaction_id, actor_id, offer_price, shares, note, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		transactionID, actor.ID, offerPrice, shares, note, time.Now().Format(time.RFC3339))
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if err := insertAudit(ctx, tx, actor.ID, "create_negotiation", "negotiation", id, fmt.Sprintf("transaction #%d offer %.2f x %d", transactionID, offerPrice, shares)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AdvanceTransaction(ctx context.Context, actorID, transactionID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var stage domain.TransactionStage
	var buyerID int64
	var sellerID int64
	var companyName string
	var cost float64
	err = tx.QueryRowContext(ctx, `SELECT t.stage, t.buyer_id, t.seller_id, c.name, t.shares * t.price FROM transactions t JOIN companies c ON c.id = t.company_id WHERE t.id = ?`, transactionID).
		Scan(&stage, &buyerID, &sellerID, &companyName, &cost)
	if err != nil {
		return err
	}
	next, err := domain.NextTransactionStage(stage)
	if err != nil {
		return err
	}
	documentStatus, rofrStatus, approvalStatus, escrowStatus := executionStatusesForStage(next)
	if _, err := tx.ExecContext(ctx, `UPDATE transactions SET stage = ?, document_status = ?, rofr_status = ?, company_approval_status = ?, escrow_status = ? WHERE id = ?`,
		string(next), documentStatus, rofrStatus, approvalStatus, escrowStatus, transactionID); err != nil {
		return err
	}
	if next == domain.StageSettled {
		if _, err := tx.ExecContext(ctx, `INSERT INTO holdings (user_id, company_name, source_type, cost, status) VALUES (?, ?, 'secondary', ?, ?)`, buyerID, companyName, cost, string(next)); err != nil {
			return err
		}
	}
	if err := insertAudit(ctx, tx, actorID, "advance_transaction", "transaction", transactionID, fmt.Sprintf("%s -> %s", stage, next)); err != nil {
		return err
	}
	notificationBody := fmt.Sprintf("%s transaction moved from %s to %s", companyName, stage, next)
	if err := insertNotification(ctx, tx, buyerID, "Transaction status updated", notificationBody); err != nil {
		return err
	}
	if err := insertNotification(ctx, tx, sellerID, "Transaction status updated", notificationBody); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CancelTransaction(ctx context.Context, actorID, transactionID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var stage domain.TransactionStage
	var buyerID, sellerID int64
	var companyName string
	if err := tx.QueryRowContext(ctx, `SELECT t.stage, t.buyer_id, t.seller_id, c.name FROM transactions t JOIN companies c ON c.id = t.company_id WHERE t.id = ?`, transactionID).Scan(&stage, &buyerID, &sellerID, &companyName); err != nil {
		return err
	}
	if stage == domain.StageSettled || stage == domain.StageCancelled {
		return fmt.Errorf("transaction is already terminal: %s", stage)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE transactions SET stage = ?, document_status = 'cancelled', rofr_status = 'cancelled', company_approval_status = 'cancelled', escrow_status = 'cancelled' WHERE id = ?`,
		string(domain.StageCancelled), transactionID); err != nil {
		return err
	}
	if err := insertAudit(ctx, tx, actorID, "cancel_transaction", "transaction", transactionID, fmt.Sprintf("%s -> %s", stage, domain.StageCancelled)); err != nil {
		return err
	}
	body := fmt.Sprintf("%s transaction was cancelled from %s", companyName, stage)
	if err := insertNotification(ctx, tx, buyerID, "Transaction cancelled", body); err != nil {
		return err
	}
	if err := insertNotification(ctx, tx, sellerID, "Transaction cancelled", body); err != nil {
		return err
	}
	return tx.Commit()
}

func executionStatusesForStage(stage domain.TransactionStage) (string, string, string, string) {
	switch stage {
	case domain.StageMatched:
		return "drafted", "not_started", "not_started", "not_started"
	case domain.StageCompanyReview:
		return "sent", "not_started", "pending", "not_started"
	case domain.StageROFRPending:
		return "signed", "pending", "pending", "not_started"
	case domain.StagePaymentPending:
		return "signed", "waived", "approved", "pending_funding"
	case domain.StageSettled:
		return "archived", "waived", "approved", "released"
	default:
		return "not_started", "not_started", "not_started", "not_started"
	}
}

func (s *Store) Deals() ([]domain.Deal, error) {
	rows, err := s.db.Query(`SELECT d.id, d.company_id, c.name, d.name, d.deal_type, d.structure, d.min_subscription, d.target_size, d.fee_description, d.status, COALESCE(SUM(s.amount), 0), d.target_size - COALESCE(SUM(s.amount), 0)
		FROM deals d JOIN companies c ON c.id = d.company_id
		LEFT JOIN subscriptions s ON s.deal_id = d.id AND s.status != 'cancelled'
		GROUP BY d.id, d.company_id, c.name, d.name, d.deal_type, d.structure, d.min_subscription, d.target_size, d.fee_description, d.status
		ORDER BY d.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var deals []domain.Deal
	for rows.Next() {
		var deal domain.Deal
		if err := rows.Scan(&deal.ID, &deal.CompanyID, &deal.CompanyName, &deal.Name, &deal.DealType, &deal.Structure, &deal.MinSubscription, &deal.TargetSize, &deal.FeeDescription, &deal.Status, &deal.SubscribedAmount, &deal.RemainingAmount); err != nil {
			return nil, err
		}
		deals = append(deals, deal)
	}
	return deals, rows.Err()
}

func (s *Store) Deal(id int64) (domain.Deal, error) {
	var deal domain.Deal
	err := s.db.QueryRow(`SELECT d.id, d.company_id, c.name, d.name, d.deal_type, d.structure, d.min_subscription, d.target_size, d.fee_description, d.status, COALESCE(SUM(s.amount), 0), d.target_size - COALESCE(SUM(s.amount), 0)
		FROM deals d JOIN companies c ON c.id = d.company_id
		LEFT JOIN subscriptions s ON s.deal_id = d.id AND s.status != 'cancelled'
		WHERE d.id = ?
		GROUP BY d.id, d.company_id, c.name, d.name, d.deal_type, d.structure, d.min_subscription, d.target_size, d.fee_description, d.status`, id).
		Scan(&deal.ID, &deal.CompanyID, &deal.CompanyName, &deal.Name, &deal.DealType, &deal.Structure, &deal.MinSubscription, &deal.TargetSize, &deal.FeeDescription, &deal.Status, &deal.SubscribedAmount, &deal.RemainingAmount)
	return deal, err
}

func (s *Store) CreateDeal(ctx context.Context, actorID int64, deal domain.Deal) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO deals (company_id, name, deal_type, structure, min_subscription, target_size, fee_description, status) VALUES (?, ?, ?, ?, ?, ?, ?, 'open')`,
		deal.CompanyID, deal.Name, deal.DealType, deal.Structure, deal.MinSubscription, deal.TargetSize, deal.FeeDescription)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if deal.DealType == "spv" || deal.DealType == "fund_basket" {
		if _, err := tx.ExecContext(ctx, `INSERT INTO spv_vehicles (deal_id, name, jurisdiction, manager, share_class, total_units, issued_units) VALUES (?, ?, 'Delaware', 'PreIPO Demo GP LLC', 'Class A', ?, 0)`,
			id, deal.Name+" Vehicle", int64(deal.TargetSize/100)); err != nil {
			return err
		}
	}
	if err := insertAudit(ctx, tx, actorID, "create_deal", "deal", id, deal.Name); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Subscriptions(user domain.User) ([]domain.Subscription, error) {
	query := `SELECT s.id, s.investor_id, u.name, s.deal_id, d.name, s.amount, s.status
		FROM subscriptions s JOIN users u ON u.id = s.investor_id JOIN deals d ON d.id = s.deal_id`
	var rows *sql.Rows
	var err error
	if user.Role == domain.RoleInvestor {
		rows, err = s.db.Query(query+` WHERE s.investor_id = ? ORDER BY s.id DESC`, user.ID)
	} else {
		rows, err = s.db.Query(query + ` ORDER BY s.id DESC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var subscriptions []domain.Subscription
	for rows.Next() {
		var subscription domain.Subscription
		if err := rows.Scan(&subscription.ID, &subscription.InvestorID, &subscription.InvestorName, &subscription.DealID, &subscription.DealName, &subscription.Amount, &subscription.Status); err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, subscription)
	}
	return subscriptions, rows.Err()
}

func (s *Store) CreateSubscription(ctx context.Context, investorID, dealID int64, amount float64) error {
	deal, err := s.Deal(dealID)
	if err != nil {
		return err
	}
	if err := domain.ValidateSubscription(amount, deal.MinSubscription); err != nil {
		return err
	}
	if deal.Status != "open" {
		return fmt.Errorf("deal is not open")
	}
	if amount > deal.RemainingAmount {
		return fmt.Errorf("subscription amount exceeds remaining deal capacity")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO subscriptions (investor_id, deal_id, amount, status) VALUES (?, ?, ?, ?)`, investorID, dealID, amount, string(domain.SubscriptionSubmitted))
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if err := insertAudit(ctx, tx, investorID, "create_subscription", "subscription", id, "investor submitted SPV subscription"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AdvanceSubscription(ctx context.Context, actorID, subscriptionID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status domain.SubscriptionStatus
	var investorID int64
	var dealID int64
	var dealName string
	var amount float64
	var targetSize float64
	err = tx.QueryRowContext(ctx, `SELECT s.status, s.investor_id, s.deal_id, d.name, s.amount, d.target_size FROM subscriptions s JOIN deals d ON d.id = s.deal_id WHERE s.id = ?`, subscriptionID).
		Scan(&status, &investorID, &dealID, &dealName, &amount, &targetSize)
	if err != nil {
		return err
	}
	next, err := domain.NextSubscriptionStatus(status)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE subscriptions SET status = ? WHERE id = ?`, string(next), subscriptionID); err != nil {
		return err
	}
	if next == domain.SubscriptionActive {
		if _, err := tx.ExecContext(ctx, `INSERT INTO holdings (user_id, company_name, source_type, cost, status) VALUES (?, ?, 'spv', ?, ?)`, investorID, dealName, amount, string(next)); err != nil {
			return err
		}
		units := int64(amount / 100)
		if units < 1 {
			units = 1
		}
		if _, err := tx.ExecContext(ctx, `UPDATE spv_vehicles SET issued_units = issued_units + ? WHERE deal_id = ?`, units, dealID); err != nil {
			return err
		}
		var activeAmount float64
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount), 0) FROM subscriptions WHERE deal_id = ? AND status = ?`, dealID, string(domain.SubscriptionActive)).Scan(&activeAmount); err != nil {
			return err
		}
		if activeAmount >= targetSize {
			if _, err := tx.ExecContext(ctx, `UPDATE deals SET status = 'closed' WHERE id = ?`, dealID); err != nil {
				return err
			}
		}
	}
	if err := insertAudit(ctx, tx, actorID, "advance_subscription", "subscription", subscriptionID, fmt.Sprintf("%s -> %s", status, next)); err != nil {
		return err
	}
	if err := insertNotification(ctx, tx, investorID, "Subscription status updated", fmt.Sprintf("%s moved from %s to %s", dealName, status, next)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CancelSubscription(ctx context.Context, actorID, subscriptionID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status domain.SubscriptionStatus
	var investorID int64
	var dealName string
	if err := tx.QueryRowContext(ctx, `SELECT s.status, s.investor_id, d.name FROM subscriptions s JOIN deals d ON d.id = s.deal_id WHERE s.id = ?`, subscriptionID).Scan(&status, &investorID, &dealName); err != nil {
		return err
	}
	if status == domain.SubscriptionActive || status == domain.SubscriptionCancelled {
		return fmt.Errorf("subscription is already terminal: %s", status)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE subscriptions SET status = ? WHERE id = ?`, string(domain.SubscriptionCancelled), subscriptionID); err != nil {
		return err
	}
	if err := insertAudit(ctx, tx, actorID, "cancel_subscription", "subscription", subscriptionID, fmt.Sprintf("%s -> %s", status, domain.SubscriptionCancelled)); err != nil {
		return err
	}
	if err := insertNotification(ctx, tx, investorID, "Subscription cancelled", fmt.Sprintf("%s subscription was cancelled from %s", dealName, status)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Holdings(userID int64) ([]domain.Holding, error) {
	rows, err := s.db.Query(`SELECT id, user_id, company_name, source_type, cost, status FROM holdings WHERE user_id = ? ORDER BY id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var holdings []domain.Holding
	for rows.Next() {
		var holding domain.Holding
		if err := rows.Scan(&holding.ID, &holding.UserID, &holding.CompanyName, &holding.SourceType, &holding.Cost, &holding.Status); err != nil {
			return nil, err
		}
		holdings = append(holdings, holding)
	}
	return holdings, rows.Err()
}

func (s *Store) SPVVehicles() ([]domain.SPVVehicle, error) {
	rows, err := s.db.Query(`SELECT v.id, v.deal_id, d.name, v.name, v.jurisdiction, v.manager, v.share_class, v.total_units, v.issued_units
		FROM spv_vehicles v JOIN deals d ON d.id = v.deal_id ORDER BY v.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var vehicles []domain.SPVVehicle
	for rows.Next() {
		var vehicle domain.SPVVehicle
		if err := rows.Scan(&vehicle.ID, &vehicle.DealID, &vehicle.DealName, &vehicle.Name, &vehicle.Jurisdiction, &vehicle.Manager, &vehicle.ShareClass, &vehicle.TotalUnits, &vehicle.IssuedUnits); err != nil {
			return nil, err
		}
		vehicles = append(vehicles, vehicle)
	}
	return vehicles, rows.Err()
}

func (s *Store) ExecutionDocuments(user domain.User) ([]domain.ExecutionDocument, error) {
	query := `SELECT d.id, d.transaction_id, d.document_type, d.status, d.signed_at, d.note
		FROM execution_documents d JOIN transactions t ON t.id = d.transaction_id`
	var rows *sql.Rows
	var err error
	switch user.Role {
	case domain.RoleInvestor, domain.RoleInstitution:
		rows, err = s.db.Query(query+` WHERE t.buyer_id = ? ORDER BY d.id DESC`, user.ID)
	case domain.RoleSeller:
		rows, err = s.db.Query(query+` WHERE t.seller_id = ? ORDER BY d.id DESC`, user.ID)
	default:
		rows, err = s.db.Query(query + ` ORDER BY d.id DESC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var documents []domain.ExecutionDocument
	for rows.Next() {
		var document domain.ExecutionDocument
		if err := rows.Scan(&document.ID, &document.TransactionID, &document.DocumentType, &document.Status, &document.SignedAt, &document.Note); err != nil {
			return nil, err
		}
		documents = append(documents, document)
	}
	return documents, rows.Err()
}

func (s *Store) CreateExecutionDocument(ctx context.Context, actorID, transactionID int64, documentType, note string) error {
	if transactionID <= 0 || documentType == "" {
		return fmt.Errorf("transaction and document type are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var stage domain.TransactionStage
	if err := tx.QueryRowContext(ctx, `SELECT stage FROM transactions WHERE id = ?`, transactionID).Scan(&stage); err != nil {
		return err
	}
	if stage == domain.StageCancelled {
		return fmt.Errorf("cannot add document to cancelled transaction")
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO execution_documents (transaction_id, document_type, status, signed_at, note) VALUES (?, ?, ?, '', ?)`,
		transactionID, documentType, string(domain.DocumentDrafted), note)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if _, err := tx.ExecContext(ctx, `UPDATE transactions SET document_status = ? WHERE id = ?`, string(domain.DocumentDrafted), transactionID); err != nil {
		return err
	}
	if err := insertAudit(ctx, tx, actorID, "create_execution_document", "execution_document", id, fmt.Sprintf("transaction #%d %s", transactionID, documentType)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AdvanceExecutionDocument(ctx context.Context, actorID, documentID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status domain.DocumentStatus
	var transactionID int64
	if err := tx.QueryRowContext(ctx, `SELECT status, transaction_id FROM execution_documents WHERE id = ?`, documentID).Scan(&status, &transactionID); err != nil {
		return err
	}
	next, err := domain.NextDocumentStatus(status)
	if err != nil {
		return err
	}
	signedAt := ""
	if next == domain.DocumentSigned {
		signedAt = time.Now().Format("2006-01-02")
	}
	if signedAt == "" {
		if _, err := tx.ExecContext(ctx, `UPDATE execution_documents SET status = ? WHERE id = ?`, string(next), documentID); err != nil {
			return err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `UPDATE execution_documents SET status = ?, signed_at = ? WHERE id = ?`, string(next), signedAt, documentID); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE transactions SET document_status = ? WHERE id = ?`, string(next), transactionID); err != nil {
		return err
	}
	if err := insertAudit(ctx, tx, actorID, "advance_execution_document", "execution_document", documentID, fmt.Sprintf("%s -> %s", status, next)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Valuations() ([]domain.ValuationRecord, error) {
	rows, err := s.db.Query(`SELECT v.id, v.company_id, c.name, v.valuation, v.share_price, v.as_of_date FROM valuations v JOIN companies c ON c.id = v.company_id ORDER BY v.as_of_date DESC, v.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var valuations []domain.ValuationRecord
	for rows.Next() {
		var valuation domain.ValuationRecord
		if err := rows.Scan(&valuation.ID, &valuation.CompanyID, &valuation.CompanyName, &valuation.Valuation, &valuation.SharePrice, &valuation.AsOfDate); err != nil {
			return nil, err
		}
		valuations = append(valuations, valuation)
	}
	return valuations, rows.Err()
}

func (s *Store) CreateValuation(ctx context.Context, actorID int64, companyID int64, valuation string, sharePrice float64, asOfDate string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO valuations (company_id, valuation, share_price, as_of_date) VALUES (?, ?, ?, ?)`, companyID, valuation, sharePrice, asOfDate)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if _, err := tx.ExecContext(ctx, `UPDATE companies SET valuation = ?, share_price = ? WHERE id = ?`, valuation, sharePrice, companyID); err != nil {
		return err
	}
	if err := insertAudit(ctx, tx, actorID, "create_valuation", "valuation", id, valuation); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ExitEvents() ([]domain.ExitEvent, error) {
	rows, err := s.db.Query(`SELECT e.id, e.company_id, c.name, e.event_type, e.description, e.status, e.expected_date FROM exit_events e JOIN companies c ON c.id = e.company_id ORDER BY e.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []domain.ExitEvent
	for rows.Next() {
		var event domain.ExitEvent
		if err := rows.Scan(&event.ID, &event.CompanyID, &event.CompanyName, &event.EventType, &event.Description, &event.Status, &event.ExpectedDate); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) CreateExitEvent(ctx context.Context, actorID int64, event domain.ExitEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO exit_events (company_id, event_type, description, status, expected_date) VALUES (?, ?, ?, ?, ?)`,
		event.CompanyID, event.EventType, event.Description, event.Status, event.ExpectedDate)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if err := insertAudit(ctx, tx, actorID, "create_exit_event", "exit_event", id, event.EventType); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Distributions(userID int64) ([]domain.Distribution, error) {
	rows, err := s.db.Query(`SELECT id, user_id, COALESCE(holding_id, 0), amount, status, tax_document FROM distributions WHERE user_id = ? ORDER BY id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var distributions []domain.Distribution
	for rows.Next() {
		var distribution domain.Distribution
		if err := rows.Scan(&distribution.ID, &distribution.UserID, &distribution.HoldingID, &distribution.Amount, &distribution.Status, &distribution.TaxDocument); err != nil {
			return nil, err
		}
		distributions = append(distributions, distribution)
	}
	return distributions, rows.Err()
}

func (s *Store) CapitalCalls(user domain.User) ([]domain.CapitalCall, error) {
	query := `SELECT cc.id, cc.user_id, u.name, cc.deal_id, d.name, cc.amount, cc.due_date, cc.status, cc.notice, cc.created_at
		FROM capital_calls cc JOIN users u ON u.id = cc.user_id JOIN deals d ON d.id = cc.deal_id`
	var rows *sql.Rows
	var err error
	if user.Role == domain.RoleAdmin {
		rows, err = s.db.Query(query + ` ORDER BY cc.id DESC`)
	} else {
		rows, err = s.db.Query(query+` WHERE cc.user_id = ? ORDER BY cc.id DESC`, user.ID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var calls []domain.CapitalCall
	for rows.Next() {
		var call domain.CapitalCall
		if err := rows.Scan(&call.ID, &call.UserID, &call.UserName, &call.DealID, &call.DealName, &call.Amount, &call.DueDate, &call.Status, &call.Notice, &call.CreatedAt); err != nil {
			return nil, err
		}
		calls = append(calls, call)
	}
	return calls, rows.Err()
}

func (s *Store) CreateCapitalCall(ctx context.Context, actorID int64, call domain.CapitalCall) error {
	if call.UserID <= 0 || call.DealID <= 0 || call.Amount <= 0 || call.DueDate == "" {
		return fmt.Errorf("user, deal, amount and due date are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO capital_calls (user_id, deal_id, amount, due_date, status, notice, created_at) VALUES (?, ?, ?, ?, 'pending', ?, ?)`,
		call.UserID, call.DealID, call.Amount, call.DueDate, call.Notice, time.Now().Format(time.RFC3339))
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if err := insertAudit(ctx, tx, actorID, "create_capital_call", "capital_call", id, fmt.Sprintf("amount %.2f due %s", call.Amount, call.DueDate)); err != nil {
		return err
	}
	if err := insertNotification(ctx, tx, call.UserID, "Capital call issued", fmt.Sprintf("A capital call of %.2f is due on %s", call.Amount, call.DueDate)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ConfirmCapitalCall(ctx context.Context, userID, callID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status string
	var amount float64
	if err := tx.QueryRowContext(ctx, `SELECT status, amount FROM capital_calls WHERE id = ? AND user_id = ?`, callID, userID).Scan(&status, &amount); err != nil {
		return err
	}
	if status != "pending" {
		return fmt.Errorf("capital call is not pending")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE capital_calls SET status = 'funded' WHERE id = ? AND user_id = ?`, callID, userID); err != nil {
		return err
	}
	if err := insertAudit(ctx, tx, userID, "confirm_capital_call", "capital_call", callID, fmt.Sprintf("funded %.2f", amount)); err != nil {
		return err
	}
	if err := insertNotification(ctx, tx, userID, "Capital call funded", fmt.Sprintf("Your capital call of %.2f was marked funded", amount)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CreateDistribution(ctx context.Context, actorID int64, distribution domain.Distribution) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var holding any
	if distribution.HoldingID > 0 {
		holding = distribution.HoldingID
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO distributions (user_id, holding_id, amount, status, tax_document) VALUES (?, ?, ?, ?, ?)`,
		distribution.UserID, holding, distribution.Amount, distribution.Status, distribution.TaxDocument)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if err := insertAudit(ctx, tx, actorID, "create_distribution", "distribution", id, distribution.Status); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Reports(userID int64) ([]domain.InvestorReport, error) {
	rows, err := s.db.Query(`SELECT id, user_id, report_type, title, period, status FROM investor_reports WHERE user_id = ? ORDER BY id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reports []domain.InvestorReport
	for rows.Next() {
		var report domain.InvestorReport
		if err := rows.Scan(&report.ID, &report.UserID, &report.ReportType, &report.Title, &report.Period, &report.Status); err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, rows.Err()
}

func (s *Store) CreateReport(ctx context.Context, actorID int64, report domain.InvestorReport) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO investor_reports (user_id, report_type, title, period, status) VALUES (?, ?, ?, ?, ?)`,
		report.UserID, report.ReportType, report.Title, report.Period, report.Status)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if err := insertAudit(ctx, tx, actorID, "create_report", "investor_report", id, report.Title); err != nil {
		return err
	}
	if err := insertNotification(ctx, tx, report.UserID, "Investor report available", fmt.Sprintf("%s for %s is %s", report.Title, report.Period, report.Status)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Notifications(userID int64, limit int) ([]domain.Notification, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(`SELECT id, user_id, title, body, status, created_at FROM notifications WHERE user_id = ? ORDER BY id DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var notifications []domain.Notification
	for rows.Next() {
		var notification domain.Notification
		if err := rows.Scan(&notification.ID, &notification.UserID, &notification.Title, &notification.Body, &notification.Status, &notification.CreatedAt); err != nil {
			return nil, err
		}
		notifications = append(notifications, notification)
	}
	return notifications, rows.Err()
}

func (s *Store) MarkNotificationRead(ctx context.Context, userID, notificationID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE notifications SET status = 'read' WHERE id = ? AND user_id = ?`, notificationID, userID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	if err := insertAudit(ctx, tx, userID, "mark_notification_read", "notification", notificationID, "status -> read"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) MarkAllNotificationsRead(ctx context.Context, userID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE notifications SET status = 'read' WHERE user_id = ? AND status = 'unread'`, userID); err != nil {
		return err
	}
	if err := insertAudit(ctx, tx, userID, "mark_all_notifications_read", "notification", userID, "all unread notifications -> read"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RiskAlerts() ([]domain.RiskAlert, error) {
	rows, err := s.db.Query(`SELECT id, severity, status, subject, note, created_at FROM risk_alerts ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var alerts []domain.RiskAlert
	for rows.Next() {
		var alert domain.RiskAlert
		if err := rows.Scan(&alert.ID, &alert.Severity, &alert.Status, &alert.Subject, &alert.Note, &alert.CreatedAt); err != nil {
			return nil, err
		}
		alerts = append(alerts, alert)
	}
	return alerts, rows.Err()
}

func (s *Store) CreateRiskAlert(ctx context.Context, actorID int64, alert domain.RiskAlert) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO risk_alerts (severity, status, subject, note, created_at) VALUES (?, ?, ?, ?, ?)`,
		alert.Severity, alert.Status, alert.Subject, alert.Note, time.Now().Format(time.RFC3339))
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if err := insertAudit(ctx, tx, actorID, "create_risk_alert", "risk_alert", id, alert.Subject); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ResolveRiskAlert(ctx context.Context, actorID, alertID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE risk_alerts SET status = 'resolved' WHERE id = ?`, alertID); err != nil {
		return err
	}
	if err := insertAudit(ctx, tx, actorID, "resolve_risk_alert", "risk_alert", alertID, "status -> resolved"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SupportTickets(userID int64, includeAll bool) ([]domain.SupportTicket, error) {
	query := `SELECT t.id, t.user_id, u.name, t.status, t.subject, t.note, t.created_at FROM support_tickets t JOIN users u ON u.id = t.user_id`
	var rows *sql.Rows
	var err error
	if includeAll {
		rows, err = s.db.Query(query + ` ORDER BY t.id DESC`)
	} else {
		rows, err = s.db.Query(query+` WHERE t.user_id = ? ORDER BY t.id DESC`, userID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tickets []domain.SupportTicket
	for rows.Next() {
		var ticket domain.SupportTicket
		if err := rows.Scan(&ticket.ID, &ticket.UserID, &ticket.UserName, &ticket.Status, &ticket.Subject, &ticket.Note, &ticket.CreatedAt); err != nil {
			return nil, err
		}
		tickets = append(tickets, ticket)
	}
	return tickets, rows.Err()
}

func (s *Store) CreateSupportTicket(ctx context.Context, userID int64, subject, note string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO support_tickets (user_id, status, subject, note, created_at) VALUES (?, 'open', ?, ?, ?)`, userID, subject, note, time.Now().Format(time.RFC3339))
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if err := insertAudit(ctx, tx, userID, "create_support_ticket", "support_ticket", id, subject); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CloseSupportTicket(ctx context.Context, actorID, ticketID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var ticketUserID int64
	var subject string
	if err := tx.QueryRowContext(ctx, `SELECT user_id, subject FROM support_tickets WHERE id = ?`, ticketID).Scan(&ticketUserID, &subject); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE support_tickets SET status = 'closed' WHERE id = ?`, ticketID); err != nil {
		return err
	}
	if err := insertAudit(ctx, tx, actorID, "close_support_ticket", "support_ticket", ticketID, "status -> closed"); err != nil {
		return err
	}
	if err := insertNotification(ctx, tx, ticketUserID, "Support ticket closed", fmt.Sprintf("%s has been closed", subject)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AuditLogs(limit int) ([]domain.AuditLog, error) {
	rows, err := s.db.Query(`SELECT a.id, COALESCE(u.name, 'system'), a.action, a.object_type, a.object_id, a.note, a.created_at
		FROM audit_logs a LEFT JOIN users u ON u.id = a.actor_id
		ORDER BY a.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logs []domain.AuditLog
	for rows.Next() {
		var log domain.AuditLog
		if err := rows.Scan(&log.ID, &log.ActorName, &log.Action, &log.ObjectType, &log.ObjectID, &log.Note, &log.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

func insertAudit(ctx context.Context, tx *sql.Tx, actorID int64, action, objectType string, objectID int64, note string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO audit_logs (actor_id, action, object_type, object_id, note, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		actorID, action, objectType, objectID, note, time.Now().Format(time.RFC3339))
	return err
}

func insertNotification(ctx context.Context, tx *sql.Tx, userID int64, title, body string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO notifications (user_id, title, body, status, created_at) VALUES (?, ?, ?, 'unread', ?)`,
		userID, title, body, time.Now().Format(time.RFC3339))
	return err
}
