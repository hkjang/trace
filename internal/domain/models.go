package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	Status      string    `json:"status"`
	Locale      string    `json:"locale"`
	Timezone    string    `json:"timezone"`
	Roles       []string  `json:"roles"`
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (u User) IsAdmin() bool {
	for _, role := range u.Roles {
		if role == "administrator" {
			return true
		}
	}
	return false
}

func (u User) Can(permission string) bool {
	for _, item := range u.Permissions {
		if item == permission || item == "*" {
			return true
		}
	}
	return false
}

type OIDCSettings struct {
	Enabled       bool   `json:"enabled"`
	IssuerURL     string `json:"issuerUrl"`
	ClientID      string `json:"clientId"`
	ClientSecret  string `json:"clientSecret,omitempty"`
	Scopes        string `json:"scopes"`
	UsernameClaim string `json:"usernameClaim"`
	EmailClaim    string `json:"emailClaim"`
	DisplayClaim  string `json:"displayClaim"`
	AutoProvision bool   `json:"autoProvision"`
	BaseURL       string `json:"baseUrl"`
}

type AISettings struct {
	Enabled           bool   `json:"enabled"`
	ProviderName      string `json:"providerName"`
	BaseURL           string `json:"baseUrl"`
	Protocol          string `json:"protocol"`
	APIKey            string `json:"apiKey,omitempty"`
	Model             string `json:"model"`
	EmbeddingModel    string `json:"embeddingModel"`
	MaxOutputTokens   int    `json:"maxOutputTokens"`
	ContextWindow     int    `json:"contextWindow"`
	RequestTimeoutSec int    `json:"requestTimeoutSec"`
	SystemPrompt      string `json:"systemPrompt"`
}

type WorkflowSettings struct {
	ApprovalRequired   bool `json:"approvalRequired"`
	RequireTeamManager bool `json:"requireTeamManager"`
}

type BrandingSettings struct {
	ServiceName string `json:"serviceName"`
	Tagline     string `json:"tagline"`
}

type Decision struct {
	ID                     uuid.UUID               `json:"id"`
	OwnerID                uuid.UUID               `json:"ownerId"`
	OwnerName              string                  `json:"ownerName,omitempty"`
	TeamID                 *uuid.UUID              `json:"teamId,omitempty"`
	Title                  string                  `json:"title"`
	Category               string                  `json:"category"`
	Decision               string                  `json:"decision"`
	Reason                 string                  `json:"reason"`
	Assumptions            string                  `json:"assumptions"`
	InvalidationConditions string                  `json:"invalidationConditions"`
	Confidence             int                     `json:"confidence"`
	Status                 string                  `json:"status"`
	WorkflowState          string                  `json:"workflowState"`
	DecidedAt              time.Time               `json:"decidedAt"`
	ReviewAt               *time.Time              `json:"reviewAt,omitempty"`
	CreatedAt              time.Time               `json:"createdAt"`
	UpdatedAt              time.Time               `json:"updatedAt"`
	Version                int                     `json:"version"`
	Alternatives           []Alternative           `json:"alternatives,omitempty"`
	Evidence               []Evidence              `json:"evidence,omitempty"`
	Expectations           []Expectation           `json:"expectations,omitempty"`
	Outcomes               []Outcome               `json:"outcomes,omitempty"`
	Reflections            []Reflection            `json:"reflections,omitempty"`
	Insights               []AIInsight             `json:"insights,omitempty"`
	Events                 []DecisionEvent         `json:"events,omitempty"`
	AssumptionItems        []Assumption            `json:"assumptionItems,omitempty"`
	Invalidations          []InvalidationCondition `json:"invalidations,omitempty"`
	ConfidenceHistory      []ConfidenceRecord      `json:"confidenceHistory,omitempty"`
	LatestScore            *DecisionScore          `json:"latestScore,omitempty"`
	Health                 string                  `json:"health,omitempty"`
}

type DecisionInput struct {
	Title                  string             `json:"title"`
	Category               string             `json:"category"`
	Decision               string             `json:"decision"`
	Reason                 string             `json:"reason"`
	Assumptions            string             `json:"assumptions"`
	InvalidationConditions string             `json:"invalidationConditions"`
	Confidence             int                `json:"confidence"`
	Status                 string             `json:"status"`
	DecidedAt              time.Time          `json:"decidedAt"`
	ReviewAt               *time.Time         `json:"reviewAt"`
	TeamID                 *uuid.UUID         `json:"teamId"`
	Alternatives           []AlternativeInput `json:"alternatives"`
	Evidence               []EvidenceInput    `json:"evidence"`
	Expectation            *ExpectationInput  `json:"expectation"`
}

type DecisionPatch struct {
	Title                  *string    `json:"title"`
	Category               *string    `json:"category"`
	Decision               *string    `json:"decision"`
	Reason                 *string    `json:"reason"`
	Assumptions            *string    `json:"assumptions"`
	InvalidationConditions *string    `json:"invalidationConditions"`
	Confidence             *int       `json:"confidence"`
	Status                 *string    `json:"status"`
	ReviewAt               *time.Time `json:"reviewAt"`
	Version                int        `json:"version"`
	ChangeReason           string     `json:"changeReason"`
}

type Alternative struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
}

type AlternativeInput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Evidence struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	Type        string     `json:"type"`
	Source      string     `json:"source"`
	Content     string     `json:"content"`
	Snapshot    string     `json:"snapshot,omitempty"`
	Summary     string     `json:"summary"`
	Reliability *int       `json:"reliability,omitempty"`
	Stance      string     `json:"stance"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
	KnownAt     time.Time  `json:"knownAt"`
	CapturedAt  time.Time  `json:"capturedAt"`
}

type EvidenceInput struct {
	Title       string     `json:"title"`
	Type        string     `json:"type"`
	Source      string     `json:"source"`
	Content     string     `json:"content"`
	Snapshot    string     `json:"snapshot"`
	Reliability *int       `json:"reliability"`
	Stance      string     `json:"stance"`
	PublishedAt *time.Time `json:"publishedAt"`
	KnownAt     time.Time  `json:"knownAt"`
}

type Expectation struct {
	ID              uuid.UUID  `json:"id"`
	Expectation     string     `json:"expectation"`
	SuccessCriteria string     `json:"successCriteria"`
	ExpectedAt      *time.Time `json:"expectedAt,omitempty"`
	Probability     *int       `json:"probability,omitempty"`
}

type ExpectationInput struct {
	Expectation     string     `json:"expectation"`
	SuccessCriteria string     `json:"successCriteria"`
	ExpectedAt      *time.Time `json:"expectedAt"`
	Probability     *int       `json:"probability"`
}

type Outcome struct {
	ID              uuid.UUID `json:"id"`
	Result          string    `json:"result"`
	OutcomeScore    int       `json:"outcomeScore"`
	DecisionQuality *int      `json:"decisionQuality,omitempty"`
	OutcomeAt       time.Time `json:"outcomeAt"`
	CreatedAt       time.Time `json:"createdAt"`
}

type Reflection struct {
	ID                  uuid.UUID `json:"id"`
	Reflection          string    `json:"reflection"`
	Learning            string    `json:"learning"`
	ReasoningStillSound *bool     `json:"reasoningStillSound,omitempty"`
	CreatedAt           time.Time `json:"createdAt"`
}

type AIInsight struct {
	ID                uuid.UUID       `json:"id"`
	InsightType       string          `json:"insightType"`
	Content           json.RawMessage `json:"content"`
	Model             string          `json:"model"`
	PromptVersion     string          `json:"promptVersion"`
	ReplayAt          *time.Time      `json:"replayAt,omitempty"`
	InputSnapshotHash string          `json:"inputSnapshotHash"`
	GeneratedAt       time.Time       `json:"generatedAt"`
}

type DecisionEvent struct {
	ID          uuid.UUID       `json:"id"`
	EventType   string          `json:"eventType"`
	Payload     json.RawMessage `json:"payload"`
	EffectiveAt time.Time       `json:"effectiveAt"`
	KnownAt     time.Time       `json:"knownAt"`
	CreatedAt   time.Time       `json:"createdAt"`
}

type DecisionVersion struct {
	ID                     uuid.UUID  `json:"id"`
	DecisionID             uuid.UUID  `json:"decisionId"`
	Version                int        `json:"version"`
	Title                  string     `json:"title"`
	Category               string     `json:"category"`
	Decision               string     `json:"decision"`
	Reason                 string     `json:"reason"`
	Assumptions            string     `json:"assumptions"`
	InvalidationConditions string     `json:"invalidationConditions"`
	Confidence             int        `json:"confidence"`
	Status                 string     `json:"status"`
	WorkflowState          string     `json:"workflowState"`
	DecidedAt              time.Time  `json:"decidedAt"`
	ReviewAt               *time.Time `json:"reviewAt,omitempty"`
	ChangeReason           string     `json:"changeReason"`
	ChangedBy              *uuid.UUID `json:"changedBy,omitempty"`
	ValidFrom              time.Time  `json:"validFrom"`
	ValidTo                *time.Time `json:"validTo,omitempty"`
	CreatedAt              time.Time  `json:"createdAt"`
}

type ConfidenceRecord struct {
	ID         uuid.UUID `json:"id"`
	DecisionID uuid.UUID `json:"decisionId"`
	Confidence int       `json:"confidence"`
	Reason     string    `json:"reason"`
	RecordedAt time.Time `json:"recordedAt"`
	CreatedAt  time.Time `json:"createdAt"`
}

type AssumptionEvent struct {
	ID             uuid.UUID  `json:"id"`
	PreviousStatus *string    `json:"previousStatus,omitempty"`
	Status         string     `json:"status"`
	Reason         string     `json:"reason"`
	EvidenceID     *uuid.UUID `json:"evidenceId,omitempty"`
	KnownAt        time.Time  `json:"knownAt"`
	CreatedAt      time.Time  `json:"createdAt"`
}

type Assumption struct {
	ID         uuid.UUID         `json:"id"`
	DecisionID uuid.UUID         `json:"decisionId"`
	Assumption string            `json:"assumption"`
	Status     string            `json:"status"`
	KnownAt    time.Time         `json:"knownAt"`
	CreatedAt  time.Time         `json:"createdAt"`
	UpdatedAt  time.Time         `json:"updatedAt"`
	Events     []AssumptionEvent `json:"events,omitempty"`
}

type InvalidationCondition struct {
	ID            uuid.UUID  `json:"id"`
	DecisionID    uuid.UUID  `json:"decisionId"`
	Condition     string     `json:"condition"`
	Status        string     `json:"status"`
	EvidenceID    *uuid.UUID `json:"evidenceId,omitempty"`
	DetectionNote string     `json:"detectionNote"`
	KnownAt       time.Time  `json:"knownAt"`
	TriggeredAt   *time.Time `json:"triggeredAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

type DecisionLink struct {
	ID               uuid.UUID `json:"id"`
	SourceDecisionID uuid.UUID `json:"sourceDecisionId"`
	TargetDecisionID uuid.UUID `json:"targetDecisionId"`
	RelationType     string    `json:"relationType"`
	Description      string    `json:"description"`
	EffectiveAt      time.Time `json:"effectiveAt"`
	CreatedAt        time.Time `json:"createdAt"`
}

type GraphNode struct {
	ID         uuid.UUID `json:"id"`
	Title      string    `json:"title"`
	Category   string    `json:"category"`
	Status     string    `json:"status"`
	Confidence int       `json:"confidence"`
	Outcome    *int      `json:"outcome,omitempty"`
	DecidedAt  time.Time `json:"decidedAt"`
	Depth      int       `json:"depth"`
	Health     string    `json:"health"`
}

type DecisionGraph struct {
	FocusID uuid.UUID      `json:"focusId"`
	At      *time.Time     `json:"at,omitempty"`
	Nodes   []GraphNode    `json:"nodes"`
	Edges   []DecisionLink `json:"edges"`
}

type ReplaySnapshot struct {
	At                 time.Time `json:"at"`
	Version            int       `json:"version"`
	Confidence         int       `json:"confidence"`
	EvidenceCount      int       `json:"evidenceCount"`
	AlternativeCount   int       `json:"alternativeCount"`
	AssumptionCount    int       `json:"assumptionCount"`
	AtRiskAssumptions  int       `json:"atRiskAssumptions"`
	OutcomeCount       int       `json:"outcomeCount"`
	LatestOutcomeScore *int      `json:"latestOutcomeScore,omitempty"`
	Decision           Decision  `json:"decision"`
}

type ReplayChange struct {
	Kind        string `json:"kind"`
	Label       string `json:"label"`
	Before      string `json:"before"`
	After       string `json:"after"`
	Description string `json:"description"`
}

type ReplayComparison struct {
	From    ReplaySnapshot `json:"from"`
	To      ReplaySnapshot `json:"to"`
	Changes []ReplayChange `json:"changes"`
}

type SimilarDecision struct {
	Decision       Decision `json:"decision"`
	Similarity     float64  `json:"similarity"`
	ContextScore   float64  `json:"contextScore"`
	MatchedExcerpt string   `json:"matchedExcerpt"`
	Reasons        []string `json:"reasons"`
}

type DecisionScore struct {
	EvidenceQuality          *int       `json:"evidenceQuality,omitempty"`
	LogicQuality             *int       `json:"logicQuality,omitempty"`
	AlternativeConsideration *int       `json:"alternativeConsideration,omitempty"`
	RiskAwareness            *int       `json:"riskAwareness,omitempty"`
	AssumptionQuality        *int       `json:"assumptionQuality,omitempty"`
	Calibration              *int       `json:"calibration,omitempty"`
	CounterEvidence          *int       `json:"counterEvidence,omitempty"`
	Overall                  *int       `json:"overall,omitempty"`
	ReplayAt                 *time.Time `json:"replayAt,omitempty"`
	EstimatedAt              time.Time  `json:"estimatedAt"`
}

type ReviewItem struct {
	DecisionID   uuid.UUID  `json:"decisionId"`
	Title        string     `json:"title"`
	Category     string     `json:"category"`
	Confidence   int        `json:"confidence"`
	Priority     int        `json:"priority"`
	Health       string     `json:"health"`
	Reasons      []string   `json:"reasons"`
	ReviewAt     *time.Time `json:"reviewAt,omitempty"`
	LastReviewed *time.Time `json:"lastReviewed,omitempty"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type BiasProfileItem struct {
	BiasType   string         `json:"biasType"`
	Count      int            `json:"count"`
	Percentage float64        `json:"percentage"`
	ByCategory map[string]int `json:"byCategory"`
}

type PatternInsight struct {
	Code        string `json:"code"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Strength    string `json:"strength"`
}

type DecisionProfile struct {
	EvidenceDriven      string `json:"evidenceDriven"`
	RiskTolerance       string `json:"riskTolerance"`
	AlternativeThinking string `json:"alternativeThinking"`
	DecisionSpeed       string `json:"decisionSpeed"`
	ConfidenceStyle     string `json:"confidenceStyle"`
	ReflectionHabit     string `json:"reflectionHabit"`
	Summary             string `json:"summary"`
}

type Dashboard struct {
	ActiveCount  int          `json:"activeCount"`
	WaitingCount int          `json:"waitingCount"`
	ReviewDue    int          `json:"reviewDue"`
	ClosedCount  int          `json:"closedCount"`
	Recent       []Decision   `json:"recent"`
	ReviewInbox  []ReviewItem `json:"reviewInbox"`
}

type ApprovalRequest struct {
	ID            uuid.UUID  `json:"id"`
	DecisionID    uuid.UUID  `json:"decisionId"`
	DecisionTitle string     `json:"decisionTitle"`
	RequesterID   uuid.UUID  `json:"requesterId"`
	RequesterName string     `json:"requesterName"`
	ReviewerID    *uuid.UUID `json:"reviewerId,omitempty"`
	State         string     `json:"state"`
	RequestNote   string     `json:"requestNote"`
	ResponseNote  string     `json:"responseNote"`
	RequestedAt   time.Time  `json:"requestedAt"`
	ResolvedAt    *time.Time `json:"resolvedAt,omitempty"`
}

type PersonalKey struct {
	ID            uuid.UUID  `json:"id"`
	Name          string     `json:"name"`
	Kind          string     `json:"kind"`
	Permissions   []string   `json:"permissions"`
	Status        string     `json:"status"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
	LastRotatedAt time.Time  `json:"lastRotatedAt"`
	CreatedAt     time.Time  `json:"createdAt"`
}

type APIToken struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	CreatedAt  time.Time  `json:"createdAt"`
}

type Team struct {
	ID            uuid.UUID   `json:"id"`
	Name          string      `json:"name"`
	Description   string      `json:"description"`
	ManagerUserID *uuid.UUID  `json:"managerUserId,omitempty"`
	ManagerName   string      `json:"managerName,omitempty"`
	MemberIDs     []uuid.UUID `json:"memberIds"`
	MemberCount   int         `json:"memberCount"`
	CreatedAt     time.Time   `json:"createdAt"`
}

type CalibrationBucket struct {
	Confidence  int     `json:"confidence"`
	Count       int     `json:"count"`
	SuccessRate float64 `json:"successRate"`
}

type Analytics struct {
	TotalDecisions    int                 `json:"totalDecisions"`
	AverageConfidence float64             `json:"averageConfidence"`
	EvidenceDepth     float64             `json:"evidenceDepth"`
	ReflectionRate    float64             `json:"reflectionRate"`
	Skill             int                 `json:"skill"`
	BadLuck           int                 `json:"badLuck"`
	GoodLuck          int                 `json:"goodLuck"`
	Mistake           int                 `json:"mistake"`
	Calibration       []CalibrationBucket `json:"calibration"`
	Biases            []BiasProfileItem   `json:"biases"`
	Patterns          []PatternInsight    `json:"patterns"`
	Profile           DecisionProfile     `json:"profile"`
}
