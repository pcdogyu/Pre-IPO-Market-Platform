package domain

import (
	"errors"
	"fmt"
)

type Role string

const (
	RoleAdmin       Role = "admin"
	RoleInvestor    Role = "investor"
	RoleSeller      Role = "seller"
	RoleInstitution Role = "institution"
)

type ReviewStatus string

const (
	ReviewPending  ReviewStatus = "pending_review"
	ReviewApproved ReviewStatus = "approved"
	ReviewRejected ReviewStatus = "rejected"
)

type TransactionStage string

const (
	StageInterestSubmitted TransactionStage = "interest_submitted"
	StageMatched           TransactionStage = "matched"
	StageCompanyReview     TransactionStage = "company_review"
	StageROFRPending       TransactionStage = "rofr_pending"
	StagePaymentPending    TransactionStage = "payment_pending"
	StageSettled           TransactionStage = "settled"
	StageCancelled         TransactionStage = "cancelled"
)

type SubscriptionStatus string

const (
	SubscriptionSubmitted      SubscriptionStatus = "submitted"
	SubscriptionAdminConfirmed SubscriptionStatus = "admin_confirmed"
	SubscriptionFunded         SubscriptionStatus = "funded"
	SubscriptionActive         SubscriptionStatus = "active"
	SubscriptionCancelled      SubscriptionStatus = "cancelled"
)

type DocumentStatus string

const (
	DocumentDrafted  DocumentStatus = "drafted"
	DocumentSent     DocumentStatus = "sent"
	DocumentSigned   DocumentStatus = "signed"
	DocumentArchived DocumentStatus = "archived"
	DocumentVoid     DocumentStatus = "void"
)

type EscrowPaymentStatus string

const (
	EscrowInstructionSent EscrowPaymentStatus = "instruction_sent"
	EscrowFunded          EscrowPaymentStatus = "funded"
	EscrowReleased        EscrowPaymentStatus = "released"
)

type User struct {
	ID                  int64
	Email               string
	Name                string
	Role                Role
	Language            string
	KYCStatus           ReviewStatus
	AMLStatus           ReviewStatus
	AccreditationStatus ReviewStatus
	RiskRating          string
}

type ComplianceReview struct {
	ID          int64
	UserID      int64
	UserName    string
	UserEmail   string
	ReviewType  string
	Status      ReviewStatus
	Note        string
	SubmittedAt string
	ReviewedAt  string
}

type Company struct {
	ID                   int64
	Name                 string
	Industry             string
	Valuation            string
	FundingRound         string
	SharePrice           float64
	Description          string
	TradableStatus       string
	TransferRestrictions string
	IPOProgress          string
	InvestorStructure    string
	ComparableCompanies  string
	HeatScore            int
	DataConfidence       int
}

type CompanyFundingRound struct {
	ID            int64
	CompanyID     int64
	CompanyName   string
	RoundName     string
	Amount        string
	Valuation     string
	LeadInvestors string
	AnnouncedAt   string
}

type CompanyRisk struct {
	ID          int64
	CompanyID   int64
	CompanyName string
	RiskType    string
	Severity    string
	Summary     string
	Mitigation  string
}

type CompanyUpdate struct {
	ID          int64
	CompanyID   int64
	CompanyName string
	UpdateType  string
	Title       string
	Body        string
	PublishedAt string
}

type CompanyFinancialReport struct {
	ID          int64
	CompanyID   int64
	CompanyName string
	ReportType  string
	Title       string
	Period      string
	FiscalDate  string
	Revenue     float64
	NetIncome   float64
	CashBalance float64
	Status      string
	PublishedAt string
}

type WatchlistItem struct {
	ID             int64
	UserID         int64
	CompanyID      int64
	CompanyName    string
	Industry       string
	Valuation      string
	TradableStatus string
	AddedAt        string
}

type SellOrder struct {
	ID          int64
	SellerID    int64
	SellerName  string
	CompanyID   int64
	CompanyName string
	Shares      int64
	TargetPrice float64
	Status      string
}

type BuyInterest struct {
	ID           int64
	InvestorID   int64
	InvestorName string
	CompanyID    int64
	CompanyName  string
	Amount       float64
	TargetPrice  float64
	Status       string
}

type Transaction struct {
	ID                    int64
	BuyerID               int64
	BuyerName             string
	SellerID              int64
	SellerName            string
	CompanyID             int64
	CompanyName           string
	Shares                int64
	Price                 float64
	Stage                 TransactionStage
	DocumentStatus        string
	ROFRStatus            string
	CompanyApprovalStatus string
	EscrowStatus          string
}

type Negotiation struct {
	ID            int64
	TransactionID int64
	ActorID       int64
	ActorName     string
	ActorRole     Role
	OfferPrice    float64
	Shares        int64
	Note          string
	CreatedAt     string
}

type Deal struct {
	ID               int64
	CompanyID        int64
	CompanyName      string
	Name             string
	DealType         string
	Structure        string
	MinSubscription  float64
	TargetSize       float64
	FeeDescription   string
	Status           string
	SubscribedAmount float64
	RemainingAmount  float64
	Eligibility      string
	KeyRisks         string
	PartnerName      string
	DocumentStatus   string
}

type InvestmentIntent struct {
	ID                int64
	UserID            int64
	UserName          string
	CompanyID         int64
	CompanyName       string
	Focus             string
	Amount            float64
	MinTicket         float64
	Lockup            string
	ProductPreference string
	AcceptStructures  string
	KYCWilling        bool
	Status            string
	CreatedAt         string
}

type IntentSummary struct {
	Label       string
	IntentCount int
	TotalAmount float64
	AvgTicket   float64
	KYCWilling  int
}

type LiquidityRequest struct {
	ID             int64
	UserID         int64
	UserName       string
	CompanyID      int64
	CompanyName    string
	Side           string
	Amount         float64
	SharePriceLow  float64
	SharePriceHigh float64
	Window         string
	Status         string
	Note           string
	CreatedAt      string
}

type Subscription struct {
	ID           int64
	InvestorID   int64
	InvestorName string
	DealID       int64
	DealName     string
	Amount       float64
	Status       SubscriptionStatus
}

type SubscriptionDocument struct {
	ID             int64
	SubscriptionID int64
	DealName       string
	InvestorName   string
	DocumentType   string
	Status         DocumentStatus
	SignedAt       string
	Note           string
}

type Holding struct {
	ID          int64
	UserID      int64
	CompanyName string
	SourceType  string
	Cost        float64
	Status      string
}

type PortfolioValuation struct {
	Label          string
	SourceType     string
	Cost           float64
	CurrentValue   float64
	UnrealizedGain float64
	Status         string
}

type PortfolioSummary struct {
	Cost           float64
	CurrentValue   float64
	UnrealizedGain float64
}

type SPVVehicle struct {
	ID           int64
	DealID       int64
	DealName     string
	Name         string
	Jurisdiction string
	Manager      string
	ShareClass   string
	TotalUnits   int64
	IssuedUnits  int64
}

type ExecutionDocument struct {
	ID            int64
	TransactionID int64
	DocumentType  string
	Status        string
	SignedAt      string
	Note          string
}

type ExecutionApproval struct {
	ID            int64
	TransactionID int64
	CompanyName   string
	ApprovalType  string
	Status        string
	DueDate       string
	DecidedAt     string
	Note          string
}

type EscrowPayment struct {
	ID            int64
	TransactionID int64
	CompanyName   string
	BuyerName     string
	SellerName    string
	Amount        float64
	Status        EscrowPaymentStatus
	Reference     string
	Note          string
	CreatedAt     string
	ReleasedAt    string
}

type ValuationRecord struct {
	ID          int64
	CompanyID   int64
	CompanyName string
	Valuation   string
	SharePrice  float64
	AsOfDate    string
}

type ExitEvent struct {
	ID           int64
	CompanyID    int64
	CompanyName  string
	EventType    string
	Description  string
	Status       string
	ExpectedDate string
}

type Distribution struct {
	ID          int64
	UserID      int64
	UserName    string
	HoldingID   int64
	Amount      float64
	Status      string
	TaxDocument string
}

type CapitalCall struct {
	ID        int64
	UserID    int64
	UserName  string
	DealID    int64
	DealName  string
	Amount    float64
	DueDate   string
	Status    string
	Notice    string
	CreatedAt string
}

type InvestorReport struct {
	ID         int64
	UserID     int64
	UserName   string
	ReportType string
	Title      string
	Period     string
	Status     string
}

type Notification struct {
	ID        int64
	UserID    int64
	Title     string
	Body      string
	Status    string
	CreatedAt string
}

type RiskAlert struct {
	ID             int64
	Severity       string
	Status         string
	Subject        string
	Note           string
	AssignedTo     int64
	AssignedToName string
	CreatedAt      string
}

type RiskAction struct {
	ID        int64
	AlertID   int64
	ActorID   int64
	ActorName string
	Action    string
	Note      string
	CreatedAt string
}

type SupportTicket struct {
	ID        int64
	UserID    int64
	UserName  string
	Status    string
	Subject   string
	Note      string
	CreatedAt string
}

type SupportTicketMessage struct {
	ID        int64
	TicketID  int64
	ActorID   int64
	ActorName string
	ActorRole Role
	Message   string
	CreatedAt string
}

type AuditLog struct {
	ID         int64
	ActorName  string
	Action     string
	ObjectType string
	ObjectID   int64
	Note       string
	CreatedAt  string
}

func CanAccessAdmin(role Role) bool {
	return role == RoleAdmin
}

func CanSubmitBuyInterest(user User) bool {
	return (user.Role == RoleInvestor || user.Role == RoleInstitution) && user.KYCStatus == ReviewApproved && user.AMLStatus == ReviewApproved && user.AccreditationStatus == ReviewApproved
}

func CanSubmitSellOrder(user User) bool {
	return user.Role == RoleSeller && user.KYCStatus == ReviewApproved
}

func NextTransactionStage(stage TransactionStage) (TransactionStage, error) {
	switch stage {
	case StageInterestSubmitted:
		return StageMatched, nil
	case StageMatched:
		return StageCompanyReview, nil
	case StageCompanyReview:
		return StageROFRPending, nil
	case StageROFRPending:
		return StagePaymentPending, nil
	case StagePaymentPending:
		return StageSettled, nil
	case StageSettled, StageCancelled:
		return stage, fmt.Errorf("transaction is already terminal: %s", stage)
	default:
		return stage, fmt.Errorf("unknown transaction stage: %s", stage)
	}
}

func NextSubscriptionStatus(status SubscriptionStatus) (SubscriptionStatus, error) {
	switch status {
	case SubscriptionSubmitted:
		return SubscriptionAdminConfirmed, nil
	case SubscriptionAdminConfirmed:
		return SubscriptionFunded, nil
	case SubscriptionFunded:
		return SubscriptionActive, nil
	case SubscriptionActive, SubscriptionCancelled:
		return status, fmt.Errorf("subscription is already terminal: %s", status)
	default:
		return status, fmt.Errorf("unknown subscription status: %s", status)
	}
}

func NextDocumentStatus(status DocumentStatus) (DocumentStatus, error) {
	switch status {
	case DocumentDrafted:
		return DocumentSent, nil
	case DocumentSent:
		return DocumentSigned, nil
	case DocumentSigned:
		return DocumentArchived, nil
	case DocumentArchived, DocumentVoid:
		return status, fmt.Errorf("document is already terminal: %s", status)
	default:
		return status, fmt.Errorf("unknown document status: %s", status)
	}
}

func NextEscrowPaymentStatus(status EscrowPaymentStatus) (EscrowPaymentStatus, error) {
	switch status {
	case EscrowInstructionSent:
		return EscrowFunded, nil
	case EscrowFunded:
		return EscrowReleased, nil
	case EscrowReleased:
		return status, fmt.Errorf("escrow payment is already terminal: %s", status)
	default:
		return status, fmt.Errorf("unknown escrow payment status: %s", status)
	}
}

func ValidateSubscription(amount, minimum float64) error {
	if amount <= 0 {
		return errors.New("subscription amount must be positive")
	}
	if amount < minimum {
		return fmt.Errorf("subscription amount %.2f is below minimum %.2f", amount, minimum)
	}
	return nil
}
