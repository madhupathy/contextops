package model

import (
	"encoding/json"
	"time"
)

// Tenant represents a tenant in the system
type Tenant struct {
	ID        string          `json:"id" db:"id"`
	Name      string          `json:"name" db:"name"`
	Config    json.RawMessage `json:"config" db:"config"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
}

// Identity represents a user or service account
type Identity struct {
	ID         string          `json:"id" db:"id"`
	TenantID   string          `json:"tenant_id" db:"tenant_id"`
	ExternalID string          `json:"external_id" db:"external_id"`
	Name       string          `json:"name" db:"name"`
	Email      *string         `json:"email,omitempty" db:"email"`
	Roles      []string        `json:"roles" db:"roles"`
	Groups     []string        `json:"groups" db:"groups"`
	Metadata   json.RawMessage `json:"metadata" db:"metadata"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at" db:"updated_at"`
}

// Agent represents a registered agent configuration
type Agent struct {
	ID          string          `json:"id" db:"id"`
	TenantID    string          `json:"tenant_id" db:"tenant_id"`
	Name        string          `json:"name" db:"name"`
	Version     string          `json:"version" db:"version"`
	Description *string         `json:"description,omitempty" db:"description"`
	Config      json.RawMessage `json:"config" db:"config"`
	Tags        []string        `json:"tags" db:"tags"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

// Run represents a single agent invocation
type Run struct {
	ID               string          `json:"id" db:"id"`
	TenantID         string          `json:"tenant_id" db:"tenant_id"`
	AgentID          *string         `json:"agent_id,omitempty" db:"agent_id"`
	IdentityID       *string         `json:"identity_id,omitempty" db:"identity_id"`
	Query            string          `json:"query" db:"query"`
	QueryMetadata    json.RawMessage `json:"query_metadata" db:"query_metadata"`
	UserRoles        []string        `json:"user_roles" db:"user_roles"`
	UserGroups       []string        `json:"user_groups" db:"user_groups"`
	Model            *string         `json:"model,omitempty" db:"model"`
	ModelConfig      json.RawMessage `json:"model_config" db:"model_config"`
	FinalAnswer      *string         `json:"final_answer,omitempty" db:"final_answer"`
	Citations        json.RawMessage `json:"citations" db:"citations"`
	SideEffects      json.RawMessage `json:"side_effects" db:"side_effects"`
	PromptTokens     int             `json:"prompt_tokens" db:"prompt_tokens"`
	CompletionTokens int             `json:"completion_tokens" db:"completion_tokens"`
	TotalTokens      int             `json:"total_tokens" db:"total_tokens"`
	LatencyMs        int             `json:"latency_ms" db:"latency_ms"`
	EstimatedCost    float64         `json:"estimated_cost" db:"estimated_cost"`
	Status           string          `json:"status" db:"status"`
	ErrorMessage     *string         `json:"error_message,omitempty" db:"error_message"`
	ExpectedAnswer   *string         `json:"expected_answer,omitempty" db:"expected_answer"`
	ExpectedTools    json.RawMessage `json:"expected_tools,omitempty" db:"expected_tools"`
	ExpectedSources  json.RawMessage `json:"expected_sources,omitempty" db:"expected_sources"`
	ContextManifest  json.RawMessage `json:"context_manifest" db:"context_manifest"`
	StartedAt        *time.Time      `json:"started_at,omitempty" db:"started_at"`
	CompletedAt      *time.Time      `json:"completed_at,omitempty" db:"completed_at"`
	CreatedAt        time.Time       `json:"created_at" db:"created_at"`
}

// RetrievalCandidate represents a document considered during retrieval
type RetrievalCandidate struct {
	ID              string          `json:"id" db:"id"`
	RunID           string          `json:"run_id" db:"run_id"`
	DocID           string          `json:"doc_id" db:"doc_id"`
	ChunkID         *string         `json:"chunk_id,omitempty" db:"chunk_id"`
	Title           *string         `json:"title,omitempty" db:"title"`
	Source          *string         `json:"source,omitempty" db:"source"`
	ContentPreview  *string         `json:"content_preview,omitempty" db:"content_preview"`
	Score           float32         `json:"score" db:"score"`
	Rank            *int            `json:"rank,omitempty" db:"rank"`
	RetrievalMethod *string         `json:"retrieval_method,omitempty" db:"retrieval_method"`
	ACLPassed       bool            `json:"acl_passed" db:"acl_passed"`
	ACLReason       *string         `json:"acl_reason,omitempty" db:"acl_reason"`
	ACLRules        json.RawMessage `json:"acl_rules" db:"acl_rules"`
	Selected        bool            `json:"selected" db:"selected"`
	RejectionReason *string         `json:"rejection_reason,omitempty" db:"rejection_reason"`
	DocMetadata     json.RawMessage `json:"doc_metadata" db:"doc_metadata"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
}

// MemoryCandidate represents a memory item considered during a run
type MemoryCandidate struct {
	ID              string          `json:"id" db:"id"`
	RunID           string          `json:"run_id" db:"run_id"`
	MemoryID        string          `json:"memory_id" db:"memory_id"`
	MemoryType      string          `json:"memory_type" db:"memory_type"`
	Content         string          `json:"content" db:"content"`
	RelevanceScore  float32         `json:"relevance_score" db:"relevance_score"`
	RecencyScore    float32         `json:"recency_score" db:"recency_score"`
	MemoryCreatedAt *time.Time      `json:"memory_created_at,omitempty" db:"memory_created_at"`
	IsStale         bool            `json:"is_stale" db:"is_stale"`
	StaleReason     *string         `json:"stale_reason,omitempty" db:"stale_reason"`
	Selected        bool            `json:"selected" db:"selected"`
	RejectionReason *string         `json:"rejection_reason,omitempty" db:"rejection_reason"`
	MemoryMetadata  json.RawMessage `json:"memory_metadata" db:"memory_metadata"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
}

// ToolCall represents a tool invocation during a run
type ToolCall struct {
	ID               string          `json:"id" db:"id"`
	RunID            string          `json:"run_id" db:"run_id"`
	ToolName         string          `json:"tool_name" db:"tool_name"`
	ToolArgs         json.RawMessage `json:"tool_args" db:"tool_args"`
	ToolResult       json.RawMessage `json:"tool_result,omitempty" db:"tool_result"`
	StepNumber       int             `json:"step_number" db:"step_number"`
	Status           string          `json:"status" db:"status"`
	ErrorMessage     *string         `json:"error_message,omitempty" db:"error_message"`
	LatencyMs        int             `json:"latency_ms" db:"latency_ms"`
	WasCorrect       *bool           `json:"was_correct,omitempty" db:"was_correct"`
	ExpectedTool     *string         `json:"expected_tool,omitempty" db:"expected_tool"`
	ExpectedArgs     json.RawMessage `json:"expected_args,omitempty" db:"expected_args"`
	RequiresApproval bool            `json:"requires_approval" db:"requires_approval"`
	ApprovalStatus   *string         `json:"approval_status,omitempty" db:"approval_status"`
	SideEffectType   *string         `json:"side_effect_type,omitempty" db:"side_effect_type"`
	CreatedAt        time.Time       `json:"created_at" db:"created_at"`
}

// ReasoningStep represents one step in agent reasoning trajectory
type ReasoningStep struct {
	ID         string          `json:"id" db:"id"`
	RunID      string          `json:"run_id" db:"run_id"`
	StepNumber int             `json:"step_number" db:"step_number"`
	StepType   string          `json:"step_type" db:"step_type"`
	Content    string          `json:"content" db:"content"`
	Metadata   json.RawMessage `json:"metadata" db:"metadata"`
	LatencyMs  int             `json:"latency_ms" db:"latency_ms"`
	TokensUsed int             `json:"tokens_used" db:"tokens_used"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
}

// Evaluation represents an evaluation result for a run
type Evaluation struct {
	ID               string          `json:"id" db:"id"`
	RunID            string          `json:"run_id" db:"run_id"`
	TenantID         string          `json:"tenant_id" db:"tenant_id"`
	EvaluatorName    string          `json:"evaluator_name" db:"evaluator_name"`
	EvaluatorVersion string          `json:"evaluator_version" db:"evaluator_version"`
	Category         string          `json:"category" db:"category"`
	Score            float32         `json:"score" db:"score"`
	Passed           bool            `json:"passed" db:"passed"`
	Details          json.RawMessage `json:"details" db:"details"`
	Reasoning        *string         `json:"reasoning,omitempty" db:"reasoning"`
	ModelUsed        *string         `json:"model_used,omitempty" db:"model_used"`
	EvalTokens       int             `json:"eval_tokens" db:"eval_tokens"`
	EvalLatencyMs    int             `json:"eval_latency_ms" db:"eval_latency_ms"`
	CreatedAt        time.Time       `json:"created_at" db:"created_at"`
}

// BenchmarkSuite represents a collection of benchmark test cases
type BenchmarkSuite struct {
	ID          string          `json:"id" db:"id"`
	TenantID    string          `json:"tenant_id" db:"tenant_id"`
	Name        string          `json:"name" db:"name"`
	Description *string         `json:"description,omitempty" db:"description"`
	Category    string          `json:"category" db:"category"`
	Config      json.RawMessage `json:"config" db:"config"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

// BenchmarkCase represents a single test case
type BenchmarkCase struct {
	ID                 string          `json:"id" db:"id"`
	SuiteID            string          `json:"suite_id" db:"suite_id"`
	Name               string          `json:"name" db:"name"`
	Query              string          `json:"query" db:"query"`
	ExpectedAnswer     *string         `json:"expected_answer,omitempty" db:"expected_answer"`
	ExpectedSources    json.RawMessage `json:"expected_sources,omitempty" db:"expected_sources"`
	ExpectedTools      json.RawMessage `json:"expected_tools,omitempty" db:"expected_tools"`
	ExpectedTrajectory json.RawMessage `json:"expected_trajectory,omitempty" db:"expected_trajectory"`
	SimulateUserRoles  []string        `json:"simulate_user_roles" db:"simulate_user_roles"`
	SimulateUserGroups []string        `json:"simulate_user_groups" db:"simulate_user_groups"`
	ForbiddenSources   json.RawMessage `json:"forbidden_sources" db:"forbidden_sources"`
	ForbiddenContent   []string        `json:"forbidden_content" db:"forbidden_content"`
	SeedMemories       json.RawMessage `json:"seed_memories" db:"seed_memories"`
	StaleMemories      json.RawMessage `json:"stale_memories" db:"stale_memories"`
	PassCriteria       json.RawMessage `json:"pass_criteria" db:"pass_criteria"`
	Metadata           json.RawMessage `json:"metadata" db:"metadata"`
	CreatedAt          time.Time       `json:"created_at" db:"created_at"`
}

// BenchmarkResult represents aggregate results for a suite run
type BenchmarkResult struct {
	ID                 string          `json:"id" db:"id"`
	SuiteID            string          `json:"suite_id" db:"suite_id"`
	TenantID           string          `json:"tenant_id" db:"tenant_id"`
	AgentID            *string         `json:"agent_id,omitempty" db:"agent_id"`
	TotalCases         int             `json:"total_cases" db:"total_cases"`
	PassedCases        int             `json:"passed_cases" db:"passed_cases"`
	FailedCases        int             `json:"failed_cases" db:"failed_cases"`
	Scores             json.RawMessage `json:"scores" db:"scores"`
	TotalTokens        int             `json:"total_tokens" db:"total_tokens"`
	TotalLatencyMs     int             `json:"total_latency_ms" db:"total_latency_ms"`
	TotalCost          float64         `json:"total_cost" db:"total_cost"`
	BaselineResultID   *string         `json:"baseline_result_id,omitempty" db:"baseline_result_id"`
	RegressionDetected bool            `json:"regression_detected" db:"regression_detected"`
	RegressionDetails  json.RawMessage `json:"regression_details" db:"regression_details"`
	Status             string          `json:"status" db:"status"`
	StartedAt          time.Time       `json:"started_at" db:"started_at"`
	CompletedAt        *time.Time      `json:"completed_at,omitempty" db:"completed_at"`
	CreatedAt          time.Time       `json:"created_at" db:"created_at"`
}

// AuditEntry represents an audit log entry
type AuditEntry struct {
	ID           string          `json:"id" db:"id"`
	TenantID     string          `json:"tenant_id" db:"tenant_id"`
	ActorID      *string         `json:"actor_id,omitempty" db:"actor_id"`
	Action       string          `json:"action" db:"action"`
	ResourceType string          `json:"resource_type" db:"resource_type"`
	ResourceID   *string         `json:"resource_id,omitempty" db:"resource_id"`
	Details      json.RawMessage `json:"details" db:"details"`
	IPAddress    *string         `json:"ip_address,omitempty" db:"ip_address"`
	CreatedAt    time.Time       `json:"created_at" db:"created_at"`
}

// Dataset represents an evaluation dataset
type Dataset struct {
	ID          string          `json:"id" db:"id"`
	TenantID    string          `json:"tenant_id" db:"tenant_id"`
	Name        string          `json:"name" db:"name"`
	Description *string         `json:"description,omitempty" db:"description"`
	Category    *string         `json:"category,omitempty" db:"category"`
	CaseCount   int             `json:"case_count" db:"case_count"`
	Config      json.RawMessage `json:"config" db:"config"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

// DatasetCase represents a single test case in a dataset
type DatasetCase struct {
	ID              string          `json:"id" db:"id"`
	DatasetID       string          `json:"dataset_id" db:"dataset_id"`
	Name            string          `json:"name" db:"name"`
	Query           string          `json:"query" db:"query"`
	ExpectedAnswer  *string         `json:"expected_answer,omitempty" db:"expected_answer"`
	ExpectedSources json.RawMessage `json:"expected_sources,omitempty" db:"expected_sources"`
	ExpectedTools   json.RawMessage `json:"expected_tools,omitempty" db:"expected_tools"`
	PassCriteria    json.RawMessage `json:"pass_criteria" db:"pass_criteria"`
	Metadata        json.RawMessage `json:"metadata" db:"metadata"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
}

// RunTimeline is the full debug view of a run
type RunTimeline struct {
	Run                 Run                  `json:"run"`
	RetrievalCandidates []RetrievalCandidate `json:"retrieval_candidates"`
	MemoryCandidates    []MemoryCandidate    `json:"memory_candidates"`
	ToolCalls           []ToolCall           `json:"tool_calls"`
	ReasoningSteps      []ReasoningStep      `json:"reasoning_steps"`
	Evaluations         []Evaluation         `json:"evaluations"`
}

// RunComparison compares two runs side by side
type RunComparison struct {
	RunA        RunTimeline     `json:"run_a"`
	RunB        RunTimeline     `json:"run_b"`
	Diffs       json.RawMessage `json:"diffs"`
	Regressions json.RawMessage `json:"regressions"`
}
