package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/contextops/api/internal/db"
	"github.com/contextops/api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type Handler struct {
	pool *db.Pool
}

func New(pool *db.Pool) *Handler {
	return &Handler{pool: pool}
}

func (h *Handler) tenantID(c *gin.Context) string {
	return c.GetString("tenant_id")
}

// ============================================================
// TENANTS
// ============================================================

type CreateTenantRequest struct {
	Name   string          `json:"name" binding:"required"`
	Config json.RawMessage `json:"config"`
}

func (h *Handler) CreateTenant(c *gin.Context) {
	var req CreateTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cfg := req.Config
	if cfg == nil {
		cfg = json.RawMessage(`{}`)
	}

	id := uuid.New().String()
	now := time.Now()

	_, err := h.pool.Exec(c.Request.Context(),
		`INSERT INTO tenants (id, name, config, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)`,
		id, req.Name, cfg, now, now,
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to create tenant")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create tenant"})
		return
	}

	h.audit(c, "create", "tenant", id, nil)
	c.JSON(http.StatusCreated, model.Tenant{
		ID: id, Name: req.Name, Config: cfg,
		CreatedAt: now, UpdatedAt: now,
	})
}

func (h *Handler) ListTenants(c *gin.Context) {
	rows, err := h.pool.Query(c.Request.Context(),
		`SELECT id, name, config, created_at, updated_at FROM tenants ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	var tenants []model.Tenant
	for rows.Next() {
		var t model.Tenant
		if err := rows.Scan(&t.ID, &t.Name, &t.Config, &t.CreatedAt, &t.UpdatedAt); err != nil {
			continue
		}
		tenants = append(tenants, t)
	}
	if tenants == nil {
		tenants = []model.Tenant{}
	}
	c.JSON(http.StatusOK, tenants)
}

func (h *Handler) GetTenant(c *gin.Context) {
	id := c.Param("id")
	var t model.Tenant
	err := h.pool.QueryRow(c.Request.Context(),
		`SELECT id, name, config, created_at, updated_at FROM tenants WHERE id = $1`, id,
	).Scan(&t.ID, &t.Name, &t.Config, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}
	c.JSON(http.StatusOK, t)
}

// ============================================================
// IDENTITIES
// ============================================================

type CreateIdentityRequest struct {
	ExternalID string          `json:"external_id" binding:"required"`
	Name       string          `json:"name" binding:"required"`
	Email      *string         `json:"email"`
	Roles      []string        `json:"roles"`
	Groups     []string        `json:"groups"`
	Metadata   json.RawMessage `json:"metadata"`
}

func (h *Handler) CreateIdentity(c *gin.Context) {
	var req CreateIdentityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := h.tenantID(c)
	id := uuid.New().String()
	now := time.Now()
	roles := req.Roles
	if roles == nil {
		roles = []string{}
	}
	groups := req.Groups
	if groups == nil {
		groups = []string{}
	}
	meta := req.Metadata
	if meta == nil {
		meta = json.RawMessage(`{}`)
	}

	_, err := h.pool.Exec(c.Request.Context(),
		`INSERT INTO identities (id, tenant_id, external_id, name, email, roles, groups, metadata, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		id, tenantID, req.ExternalID, req.Name, req.Email, roles, groups, meta, now, now,
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to create identity")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create identity"})
		return
	}

	h.audit(c, "create", "identity", id, nil)
	c.JSON(http.StatusCreated, model.Identity{
		ID: id, TenantID: tenantID, ExternalID: req.ExternalID,
		Name: req.Name, Email: req.Email, Roles: roles, Groups: groups,
		Metadata: meta, CreatedAt: now, UpdatedAt: now,
	})
}

func (h *Handler) ListIdentities(c *gin.Context) {
	tenantID := h.tenantID(c)
	rows, err := h.pool.Query(c.Request.Context(),
		`SELECT id, tenant_id, external_id, name, email, roles, groups, metadata, created_at, updated_at
		 FROM identities WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT 100`, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	var identities []model.Identity
	for rows.Next() {
		var i model.Identity
		if err := rows.Scan(&i.ID, &i.TenantID, &i.ExternalID, &i.Name, &i.Email, &i.Roles, &i.Groups, &i.Metadata, &i.CreatedAt, &i.UpdatedAt); err != nil {
			continue
		}
		identities = append(identities, i)
	}
	if identities == nil {
		identities = []model.Identity{}
	}
	c.JSON(http.StatusOK, identities)
}

func (h *Handler) GetIdentity(c *gin.Context) {
	id := c.Param("id")
	tenantID := h.tenantID(c)
	var i model.Identity
	err := h.pool.QueryRow(c.Request.Context(),
		`SELECT id, tenant_id, external_id, name, email, roles, groups, metadata, created_at, updated_at
		 FROM identities WHERE id = $1 AND tenant_id = $2`, id, tenantID,
	).Scan(&i.ID, &i.TenantID, &i.ExternalID, &i.Name, &i.Email, &i.Roles, &i.Groups, &i.Metadata, &i.CreatedAt, &i.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "identity not found"})
		return
	}
	c.JSON(http.StatusOK, i)
}

// ============================================================
// AGENTS
// ============================================================

type CreateAgentRequest struct {
	Name        string          `json:"name" binding:"required"`
	Version     string          `json:"version"`
	Description *string         `json:"description"`
	Config      json.RawMessage `json:"config"`
	Tags        []string        `json:"tags"`
}

func (h *Handler) CreateAgent(c *gin.Context) {
	var req CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := h.tenantID(c)
	id := uuid.New().String()
	now := time.Now()
	version := req.Version
	if version == "" {
		version = "v1"
	}
	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}
	cfg := req.Config
	if cfg == nil {
		cfg = json.RawMessage(`{}`)
	}

	_, err := h.pool.Exec(c.Request.Context(),
		`INSERT INTO agents (id, tenant_id, name, version, description, config, tags, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		id, tenantID, req.Name, version, req.Description, cfg, tags, now, now,
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to create agent")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create agent"})
		return
	}

	h.audit(c, "create", "agent", id, nil)
	c.JSON(http.StatusCreated, model.Agent{
		ID: id, TenantID: tenantID, Name: req.Name, Version: version,
		Description: req.Description, Config: cfg, Tags: tags,
		CreatedAt: now, UpdatedAt: now,
	})
}

func (h *Handler) ListAgents(c *gin.Context) {
	tenantID := h.tenantID(c)
	rows, err := h.pool.Query(c.Request.Context(),
		`SELECT id, tenant_id, name, version, description, config, tags, created_at, updated_at
		 FROM agents WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT 100`, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	var agents []model.Agent
	for rows.Next() {
		var a model.Agent
		if err := rows.Scan(&a.ID, &a.TenantID, &a.Name, &a.Version, &a.Description, &a.Config, &a.Tags, &a.CreatedAt, &a.UpdatedAt); err != nil {
			continue
		}
		agents = append(agents, a)
	}
	if agents == nil {
		agents = []model.Agent{}
	}
	c.JSON(http.StatusOK, agents)
}

func (h *Handler) GetAgent(c *gin.Context) {
	id := c.Param("id")
	tenantID := h.tenantID(c)
	var a model.Agent
	err := h.pool.QueryRow(c.Request.Context(),
		`SELECT id, tenant_id, name, version, description, config, tags, created_at, updated_at
		 FROM agents WHERE id = $1 AND tenant_id = $2`, id, tenantID,
	).Scan(&a.ID, &a.TenantID, &a.Name, &a.Version, &a.Description, &a.Config, &a.Tags, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}
	c.JSON(http.StatusOK, a)
}

// ============================================================
// RUNS (core)
// ============================================================

type IngestRunRequest struct {
	AgentID          *string                  `json:"agent_id"`
	IdentityID       *string                  `json:"identity_id"`
	Query            string                   `json:"query" binding:"required"`
	QueryMetadata    json.RawMessage          `json:"query_metadata"`
	UserRoles        []string                 `json:"user_roles"`
	UserGroups       []string                 `json:"user_groups"`
	Model            *string                  `json:"model"`
	ModelConfig      json.RawMessage          `json:"model_config"`
	FinalAnswer      *string                  `json:"final_answer"`
	Citations        json.RawMessage          `json:"citations"`
	SideEffects      json.RawMessage          `json:"side_effects"`
	PromptTokens     int                      `json:"prompt_tokens"`
	CompletionTokens int                      `json:"completion_tokens"`
	TotalTokens      int                      `json:"total_tokens"`
	LatencyMs        int                      `json:"latency_ms"`
	EstimatedCost    float64                  `json:"estimated_cost"`
	Status           string                   `json:"status"`
	ErrorMessage     *string                  `json:"error_message"`
	ExpectedAnswer   *string                  `json:"expected_answer"`
	ExpectedTools    json.RawMessage          `json:"expected_tools"`
	ExpectedSources  json.RawMessage          `json:"expected_sources"`
	ContextManifest  json.RawMessage          `json:"context_manifest"`
	StartedAt        *time.Time               `json:"started_at"`
	CompletedAt      *time.Time               `json:"completed_at"`
	Retrieval        []IngestRetrievalRequest `json:"retrieval_candidates"`
	Memory           []IngestMemoryRequest    `json:"memory_candidates"`
	Tools            []IngestToolCallRequest  `json:"tool_calls"`
	Steps            []IngestStepRequest      `json:"reasoning_steps"`
}

type IngestRetrievalRequest struct {
	DocID           string          `json:"doc_id" binding:"required"`
	ChunkID         *string         `json:"chunk_id"`
	Title           *string         `json:"title"`
	Source          *string         `json:"source"`
	ContentPreview  *string         `json:"content_preview"`
	Score           float32         `json:"score"`
	Rank            *int            `json:"rank"`
	RetrievalMethod *string         `json:"retrieval_method"`
	ACLPassed       bool            `json:"acl_passed"`
	ACLReason       *string         `json:"acl_reason"`
	ACLRules        json.RawMessage `json:"acl_rules"`
	Selected        bool            `json:"selected"`
	RejectionReason *string         `json:"rejection_reason"`
	DocMetadata     json.RawMessage `json:"doc_metadata"`
}

type IngestMemoryRequest struct {
	MemoryID        string          `json:"memory_id" binding:"required"`
	MemoryType      string          `json:"memory_type" binding:"required"`
	Content         string          `json:"content" binding:"required"`
	RelevanceScore  float32         `json:"relevance_score"`
	RecencyScore    float32         `json:"recency_score"`
	MemoryCreatedAt *time.Time      `json:"memory_created_at"`
	IsStale         bool            `json:"is_stale"`
	StaleReason     *string         `json:"stale_reason"`
	Selected        bool            `json:"selected"`
	RejectionReason *string         `json:"rejection_reason"`
	MemoryMetadata  json.RawMessage `json:"memory_metadata"`
}

type IngestToolCallRequest struct {
	ToolName         string          `json:"tool_name" binding:"required"`
	ToolArgs         json.RawMessage `json:"tool_args"`
	ToolResult       json.RawMessage `json:"tool_result"`
	StepNumber       int             `json:"step_number"`
	Status           string          `json:"status"`
	ErrorMessage     *string         `json:"error_message"`
	LatencyMs        int             `json:"latency_ms"`
	WasCorrect       *bool           `json:"was_correct"`
	ExpectedTool     *string         `json:"expected_tool"`
	ExpectedArgs     json.RawMessage `json:"expected_args"`
	RequiresApproval bool            `json:"requires_approval"`
	SideEffectType   *string         `json:"side_effect_type"`
}

type IngestStepRequest struct {
	StepNumber int             `json:"step_number" binding:"required"`
	StepType   string          `json:"step_type" binding:"required"`
	Content    string          `json:"content" binding:"required"`
	Metadata   json.RawMessage `json:"metadata"`
	LatencyMs  int             `json:"latency_ms"`
	TokensUsed int             `json:"tokens_used"`
}

func (h *Handler) IngestRun(c *gin.Context) {
	var req IngestRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := h.tenantID(c)
	runID := uuid.New().String()
	now := time.Now()

	status := req.Status
	if status == "" {
		status = "completed"
	}

	ctx := c.Request.Context()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "transaction failed"})
		return
	}
	defer tx.Rollback(ctx)

	// Insert run
	_, err = tx.Exec(ctx,
		`INSERT INTO runs (id, tenant_id, agent_id, identity_id, query, query_metadata,
		 user_roles, user_groups, model, model_config, final_answer, citations, side_effects,
		 prompt_tokens, completion_tokens, total_tokens, latency_ms, estimated_cost,
		 status, error_message, expected_answer, expected_tools, expected_sources,
		 context_manifest, started_at, completed_at, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27)`,
		runID, tenantID, req.AgentID, req.IdentityID, req.Query,
		defaultJSON(req.QueryMetadata), defaultSlice(req.UserRoles), defaultSlice(req.UserGroups),
		req.Model, defaultJSON(req.ModelConfig), req.FinalAnswer,
		defaultJSON(req.Citations), defaultJSON(req.SideEffects),
		req.PromptTokens, req.CompletionTokens, req.TotalTokens, req.LatencyMs, req.EstimatedCost,
		status, req.ErrorMessage, req.ExpectedAnswer, req.ExpectedTools, req.ExpectedSources,
		defaultJSON(req.ContextManifest), req.StartedAt, req.CompletedAt, now,
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to insert run")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert run"})
		return
	}

	// Insert retrieval candidates
	for _, rc := range req.Retrieval {
		_, err = tx.Exec(ctx,
			`INSERT INTO retrieval_candidates (id, run_id, doc_id, chunk_id, title, source, content_preview,
			 score, rank, retrieval_method, acl_passed, acl_reason, acl_rules, selected, rejection_reason, doc_metadata, created_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
			uuid.New().String(), runID, rc.DocID, rc.ChunkID, rc.Title, rc.Source, rc.ContentPreview,
			rc.Score, rc.Rank, rc.RetrievalMethod, rc.ACLPassed, rc.ACLReason, defaultJSON(rc.ACLRules),
			rc.Selected, rc.RejectionReason, defaultJSON(rc.DocMetadata), now,
		)
		if err != nil {
			log.Error().Err(err).Msg("failed to insert retrieval candidate")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert retrieval candidate"})
			return
		}
	}

	// Insert memory candidates
	for _, mc := range req.Memory {
		_, err = tx.Exec(ctx,
			`INSERT INTO memory_candidates (id, run_id, memory_id, memory_type, content,
			 relevance_score, recency_score, memory_created_at, is_stale, stale_reason,
			 selected, rejection_reason, memory_metadata, created_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			uuid.New().String(), runID, mc.MemoryID, mc.MemoryType, mc.Content,
			mc.RelevanceScore, mc.RecencyScore, mc.MemoryCreatedAt, mc.IsStale, mc.StaleReason,
			mc.Selected, mc.RejectionReason, defaultJSON(mc.MemoryMetadata), now,
		)
		if err != nil {
			log.Error().Err(err).Msg("failed to insert memory candidate")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert memory candidate"})
			return
		}
	}

	// Insert tool calls
	for _, tc := range req.Tools {
		tcStatus := tc.Status
		if tcStatus == "" {
			tcStatus = "success"
		}
		_, err = tx.Exec(ctx,
			`INSERT INTO tool_calls (id, run_id, tool_name, tool_args, tool_result,
			 step_number, status, error_message, latency_ms, was_correct, expected_tool,
			 expected_args, requires_approval, side_effect_type, created_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
			uuid.New().String(), runID, tc.ToolName, defaultJSON(tc.ToolArgs), tc.ToolResult,
			tc.StepNumber, tcStatus, tc.ErrorMessage, tc.LatencyMs, tc.WasCorrect,
			tc.ExpectedTool, tc.ExpectedArgs, tc.RequiresApproval, tc.SideEffectType, now,
		)
		if err != nil {
			log.Error().Err(err).Msg("failed to insert tool call")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert tool call"})
			return
		}
	}

	// Insert reasoning steps
	for _, s := range req.Steps {
		_, err = tx.Exec(ctx,
			`INSERT INTO reasoning_steps (id, run_id, step_number, step_type, content, metadata, latency_ms, tokens_used, created_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			uuid.New().String(), runID, s.StepNumber, s.StepType, s.Content,
			defaultJSON(s.Metadata), s.LatencyMs, s.TokensUsed, now,
		)
		if err != nil {
			log.Error().Err(err).Msg("failed to insert reasoning step")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert reasoning step"})
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "commit failed"})
		return
	}

	h.audit(c, "ingest", "run", runID, nil)
	c.JSON(http.StatusCreated, gin.H{"id": runID, "status": status})
}

func (h *Handler) ListRuns(c *gin.Context) {
	tenantID := h.tenantID(c)
	agentID := c.Query("agent_id")
	status := c.Query("status")

	query := `SELECT id, tenant_id, agent_id, identity_id, query, query_metadata,
		user_roles, user_groups, model, model_config, final_answer, citations, side_effects,
		prompt_tokens, completion_tokens, total_tokens, latency_ms, estimated_cost,
		status, error_message, expected_answer, expected_tools, expected_sources,
		context_manifest, started_at, completed_at, created_at
		FROM runs WHERE tenant_id = $1`
	args := []interface{}{tenantID}

	if agentID != "" {
		args = append(args, agentID)
		query += ` AND agent_id = $` + itoa(len(args))
	}
	if status != "" {
		args = append(args, status)
		query += ` AND status = $` + itoa(len(args))
	}
	query += ` ORDER BY created_at DESC LIMIT 100`

	rows, err := h.pool.Query(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	var runs []model.Run
	for rows.Next() {
		var r model.Run
		if err := rows.Scan(&r.ID, &r.TenantID, &r.AgentID, &r.IdentityID, &r.Query, &r.QueryMetadata,
			&r.UserRoles, &r.UserGroups, &r.Model, &r.ModelConfig, &r.FinalAnswer, &r.Citations, &r.SideEffects,
			&r.PromptTokens, &r.CompletionTokens, &r.TotalTokens, &r.LatencyMs, &r.EstimatedCost,
			&r.Status, &r.ErrorMessage, &r.ExpectedAnswer, &r.ExpectedTools, &r.ExpectedSources,
			&r.ContextManifest, &r.StartedAt, &r.CompletedAt, &r.CreatedAt); err != nil {
			continue
		}
		runs = append(runs, r)
	}
	if runs == nil {
		runs = []model.Run{}
	}
	c.JSON(http.StatusOK, runs)
}

func (h *Handler) GetRun(c *gin.Context) {
	id := c.Param("id")
	tenantID := h.tenantID(c)
	r, err := h.fetchRun(c.Request.Context(), id, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return
	}
	c.JSON(http.StatusOK, r)
}

func (h *Handler) GetRunTimeline(c *gin.Context) {
	id := c.Param("id")
	tenantID := h.tenantID(c)

	run, err := h.fetchRun(c.Request.Context(), id, tenantID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return
	}

	ctx := c.Request.Context()
	timeline := model.RunTimeline{Run: *run}

	// Retrieval candidates
	rcRows, _ := h.pool.Query(ctx,
		`SELECT id, run_id, doc_id, chunk_id, title, source, content_preview,
		 score, rank, retrieval_method, acl_passed, acl_reason, acl_rules,
		 selected, rejection_reason, doc_metadata, created_at
		 FROM retrieval_candidates WHERE run_id = $1 ORDER BY score DESC`, id)
	if rcRows != nil {
		defer rcRows.Close()
		for rcRows.Next() {
			var rc model.RetrievalCandidate
			if err := rcRows.Scan(&rc.ID, &rc.RunID, &rc.DocID, &rc.ChunkID, &rc.Title, &rc.Source, &rc.ContentPreview,
				&rc.Score, &rc.Rank, &rc.RetrievalMethod, &rc.ACLPassed, &rc.ACLReason, &rc.ACLRules,
				&rc.Selected, &rc.RejectionReason, &rc.DocMetadata, &rc.CreatedAt); err == nil {
				timeline.RetrievalCandidates = append(timeline.RetrievalCandidates, rc)
			}
		}
	}

	// Memory candidates
	mcRows, _ := h.pool.Query(ctx,
		`SELECT id, run_id, memory_id, memory_type, content,
		 relevance_score, recency_score, memory_created_at, is_stale, stale_reason,
		 selected, rejection_reason, memory_metadata, created_at
		 FROM memory_candidates WHERE run_id = $1 ORDER BY relevance_score DESC`, id)
	if mcRows != nil {
		defer mcRows.Close()
		for mcRows.Next() {
			var mc model.MemoryCandidate
			if err := mcRows.Scan(&mc.ID, &mc.RunID, &mc.MemoryID, &mc.MemoryType, &mc.Content,
				&mc.RelevanceScore, &mc.RecencyScore, &mc.MemoryCreatedAt, &mc.IsStale, &mc.StaleReason,
				&mc.Selected, &mc.RejectionReason, &mc.MemoryMetadata, &mc.CreatedAt); err == nil {
				timeline.MemoryCandidates = append(timeline.MemoryCandidates, mc)
			}
		}
	}

	// Tool calls
	tcRows, _ := h.pool.Query(ctx,
		`SELECT id, run_id, tool_name, tool_args, tool_result,
		 step_number, status, error_message, latency_ms, was_correct,
		 expected_tool, expected_args, requires_approval, approval_status, side_effect_type, created_at
		 FROM tool_calls WHERE run_id = $1 ORDER BY step_number`, id)
	if tcRows != nil {
		defer tcRows.Close()
		for tcRows.Next() {
			var tc model.ToolCall
			if err := tcRows.Scan(&tc.ID, &tc.RunID, &tc.ToolName, &tc.ToolArgs, &tc.ToolResult,
				&tc.StepNumber, &tc.Status, &tc.ErrorMessage, &tc.LatencyMs, &tc.WasCorrect,
				&tc.ExpectedTool, &tc.ExpectedArgs, &tc.RequiresApproval, &tc.ApprovalStatus, &tc.SideEffectType, &tc.CreatedAt); err == nil {
				timeline.ToolCalls = append(timeline.ToolCalls, tc)
			}
		}
	}

	// Reasoning steps
	rsRows, _ := h.pool.Query(ctx,
		`SELECT id, run_id, step_number, step_type, content, metadata, latency_ms, tokens_used, created_at
		 FROM reasoning_steps WHERE run_id = $1 ORDER BY step_number`, id)
	if rsRows != nil {
		defer rsRows.Close()
		for rsRows.Next() {
			var rs model.ReasoningStep
			if err := rsRows.Scan(&rs.ID, &rs.RunID, &rs.StepNumber, &rs.StepType, &rs.Content,
				&rs.Metadata, &rs.LatencyMs, &rs.TokensUsed, &rs.CreatedAt); err == nil {
				timeline.ReasoningSteps = append(timeline.ReasoningSteps, rs)
			}
		}
	}

	// Evaluations
	evRows, _ := h.pool.Query(ctx,
		`SELECT id, run_id, tenant_id, evaluator_name, evaluator_version, category,
		 score, passed, details, reasoning, model_used, eval_tokens, eval_latency_ms, created_at
		 FROM evaluations WHERE run_id = $1 ORDER BY category`, id)
	if evRows != nil {
		defer evRows.Close()
		for evRows.Next() {
			var ev model.Evaluation
			if err := evRows.Scan(&ev.ID, &ev.RunID, &ev.TenantID, &ev.EvaluatorName, &ev.EvaluatorVersion,
				&ev.Category, &ev.Score, &ev.Passed, &ev.Details, &ev.Reasoning, &ev.ModelUsed,
				&ev.EvalTokens, &ev.EvalLatencyMs, &ev.CreatedAt); err == nil {
				timeline.Evaluations = append(timeline.Evaluations, ev)
			}
		}
	}

	// Default empty slices
	if timeline.RetrievalCandidates == nil {
		timeline.RetrievalCandidates = []model.RetrievalCandidate{}
	}
	if timeline.MemoryCandidates == nil {
		timeline.MemoryCandidates = []model.MemoryCandidate{}
	}
	if timeline.ToolCalls == nil {
		timeline.ToolCalls = []model.ToolCall{}
	}
	if timeline.ReasoningSteps == nil {
		timeline.ReasoningSteps = []model.ReasoningStep{}
	}
	if timeline.Evaluations == nil {
		timeline.Evaluations = []model.Evaluation{}
	}

	c.JSON(http.StatusOK, timeline)
}

func (h *Handler) GetContextManifest(c *gin.Context) {
	id := c.Param("id")
	tenantID := h.tenantID(c)

	var manifest json.RawMessage
	err := h.pool.QueryRow(c.Request.Context(),
		`SELECT context_manifest FROM runs WHERE id = $1 AND tenant_id = $2`, id, tenantID,
	).Scan(&manifest)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return
	}
	c.JSON(http.StatusOK, json.RawMessage(manifest))
}

// ============================================================
// EVALUATIONS
// ============================================================

type TriggerEvaluationRequest struct {
	Categories []string `json:"categories"`
}

func (h *Handler) TriggerEvaluation(c *gin.Context) {
	runID := c.Param("id")
	tenantID := h.tenantID(c)

	var req TriggerEvaluationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Categories = []string{
			"answer_correctness", "groundedness", "retrieval_quality",
			"permission_safety", "memory_utility", "tool_correctness",
		}
	}

	// Verify run exists
	var exists bool
	_ = h.pool.QueryRow(c.Request.Context(),
		`SELECT EXISTS(SELECT 1 FROM runs WHERE id = $1 AND tenant_id = $2)`, runID, tenantID,
	).Scan(&exists)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "run not found"})
		return
	}

	// TODO: Queue evaluation job to Python evaluator service via Redis
	// For now, return accepted status
	h.audit(c, "trigger_eval", "run", runID, nil)
	c.JSON(http.StatusAccepted, gin.H{
		"run_id":     runID,
		"categories": req.Categories,
		"status":     "queued",
		"message":    "Evaluation queued. Results will be available via GET /runs/{id}/evaluations",
	})
}

func (h *Handler) GetRunEvaluations(c *gin.Context) {
	runID := c.Param("id")
	tenantID := h.tenantID(c)

	rows, err := h.pool.Query(c.Request.Context(),
		`SELECT id, run_id, tenant_id, evaluator_name, evaluator_version, category,
		 score, passed, details, reasoning, model_used, eval_tokens, eval_latency_ms, created_at
		 FROM evaluations WHERE run_id = $1 AND tenant_id = $2 ORDER BY category`, runID, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	var evals []model.Evaluation
	for rows.Next() {
		var ev model.Evaluation
		if err := rows.Scan(&ev.ID, &ev.RunID, &ev.TenantID, &ev.EvaluatorName, &ev.EvaluatorVersion,
			&ev.Category, &ev.Score, &ev.Passed, &ev.Details, &ev.Reasoning, &ev.ModelUsed,
			&ev.EvalTokens, &ev.EvalLatencyMs, &ev.CreatedAt); err == nil {
			evals = append(evals, ev)
		}
	}
	if evals == nil {
		evals = []model.Evaluation{}
	}
	c.JSON(http.StatusOK, evals)
}

func (h *Handler) ListEvaluations(c *gin.Context) {
	tenantID := h.tenantID(c)
	category := c.Query("category")

	query := `SELECT id, run_id, tenant_id, evaluator_name, evaluator_version, category,
		score, passed, details, reasoning, model_used, eval_tokens, eval_latency_ms, created_at
		FROM evaluations WHERE tenant_id = $1`
	args := []interface{}{tenantID}

	if category != "" {
		args = append(args, category)
		query += ` AND category = $` + itoa(len(args))
	}
	query += ` ORDER BY created_at DESC LIMIT 100`

	rows, err := h.pool.Query(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	var evals []model.Evaluation
	for rows.Next() {
		var ev model.Evaluation
		if err := rows.Scan(&ev.ID, &ev.RunID, &ev.TenantID, &ev.EvaluatorName, &ev.EvaluatorVersion,
			&ev.Category, &ev.Score, &ev.Passed, &ev.Details, &ev.Reasoning, &ev.ModelUsed,
			&ev.EvalTokens, &ev.EvalLatencyMs, &ev.CreatedAt); err == nil {
			evals = append(evals, ev)
		}
	}
	if evals == nil {
		evals = []model.Evaluation{}
	}
	c.JSON(http.StatusOK, evals)
}

// ============================================================
// BENCHMARKS
// ============================================================

type CreateBenchmarkSuiteRequest struct {
	Name        string                       `json:"name" binding:"required"`
	Description *string                      `json:"description"`
	Category    string                       `json:"category" binding:"required"`
	Config      json.RawMessage              `json:"config"`
	Cases       []CreateBenchmarkCaseRequest `json:"cases"`
}

type CreateBenchmarkCaseRequest struct {
	Name               string          `json:"name" binding:"required"`
	Query              string          `json:"query" binding:"required"`
	ExpectedAnswer     *string         `json:"expected_answer"`
	ExpectedSources    json.RawMessage `json:"expected_sources"`
	ExpectedTools      json.RawMessage `json:"expected_tools"`
	SimulateUserRoles  []string        `json:"simulate_user_roles"`
	SimulateUserGroups []string        `json:"simulate_user_groups"`
	ForbiddenSources   json.RawMessage `json:"forbidden_sources"`
	ForbiddenContent   []string        `json:"forbidden_content"`
	SeedMemories       json.RawMessage `json:"seed_memories"`
	StaleMemories      json.RawMessage `json:"stale_memories"`
	PassCriteria       json.RawMessage `json:"pass_criteria"`
}

func (h *Handler) CreateBenchmarkSuite(c *gin.Context) {
	var req CreateBenchmarkSuiteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := h.tenantID(c)
	suiteID := uuid.New().String()
	now := time.Now()
	cfg := req.Config
	if cfg == nil {
		cfg = json.RawMessage(`{}`)
	}

	ctx := c.Request.Context()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "transaction failed"})
		return
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`INSERT INTO benchmark_suites (id, tenant_id, name, description, category, config, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		suiteID, tenantID, req.Name, req.Description, req.Category, cfg, now, now,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create suite"})
		return
	}

	for _, bc := range req.Cases {
		_, err = tx.Exec(ctx,
			`INSERT INTO benchmark_cases (id, suite_id, name, query, expected_answer, expected_sources,
			 expected_tools, simulate_user_roles, simulate_user_groups, forbidden_sources, forbidden_content,
			 seed_memories, stale_memories, pass_criteria, created_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
			uuid.New().String(), suiteID, bc.Name, bc.Query, bc.ExpectedAnswer,
			defaultJSON(bc.ExpectedSources), defaultJSON(bc.ExpectedTools),
			defaultSlice(bc.SimulateUserRoles), defaultSlice(bc.SimulateUserGroups),
			defaultJSON(bc.ForbiddenSources), defaultSlice(bc.ForbiddenContent),
			defaultJSON(bc.SeedMemories), defaultJSON(bc.StaleMemories),
			defaultJSON(bc.PassCriteria), now,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create benchmark case"})
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "commit failed"})
		return
	}

	h.audit(c, "create", "benchmark_suite", suiteID, nil)
	c.JSON(http.StatusCreated, gin.H{"id": suiteID, "cases_created": len(req.Cases)})
}

func (h *Handler) ListBenchmarkSuites(c *gin.Context) {
	tenantID := h.tenantID(c)
	rows, err := h.pool.Query(c.Request.Context(),
		`SELECT id, tenant_id, name, description, category, config, created_at, updated_at
		 FROM benchmark_suites WHERE tenant_id = $1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	var suites []model.BenchmarkSuite
	for rows.Next() {
		var s model.BenchmarkSuite
		if err := rows.Scan(&s.ID, &s.TenantID, &s.Name, &s.Description, &s.Category, &s.Config, &s.CreatedAt, &s.UpdatedAt); err == nil {
			suites = append(suites, s)
		}
	}
	if suites == nil {
		suites = []model.BenchmarkSuite{}
	}
	c.JSON(http.StatusOK, suites)
}

func (h *Handler) GetBenchmarkSuite(c *gin.Context) {
	id := c.Param("id")
	tenantID := h.tenantID(c)
	var s model.BenchmarkSuite
	err := h.pool.QueryRow(c.Request.Context(),
		`SELECT id, tenant_id, name, description, category, config, created_at, updated_at
		 FROM benchmark_suites WHERE id = $1 AND tenant_id = $2`, id, tenantID,
	).Scan(&s.ID, &s.TenantID, &s.Name, &s.Description, &s.Category, &s.Config, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "benchmark suite not found"})
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *Handler) RunBenchmark(c *gin.Context) {
	suiteID := c.Param("id")
	tenantID := h.tenantID(c)

	// Verify suite exists
	var exists bool
	_ = h.pool.QueryRow(c.Request.Context(),
		`SELECT EXISTS(SELECT 1 FROM benchmark_suites WHERE id = $1 AND tenant_id = $2)`, suiteID, tenantID,
	).Scan(&exists)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "benchmark suite not found"})
		return
	}

	// TODO: Queue benchmark execution
	h.audit(c, "run_benchmark", "benchmark_suite", suiteID, nil)
	c.JSON(http.StatusAccepted, gin.H{
		"suite_id": suiteID,
		"status":   "queued",
		"message":  "Benchmark queued. Results will be available via GET /benchmarks/{id}/results",
	})
}

func (h *Handler) GetBenchmarkResults(c *gin.Context) {
	suiteID := c.Param("id")
	tenantID := h.tenantID(c)

	rows, err := h.pool.Query(c.Request.Context(),
		`SELECT id, suite_id, tenant_id, agent_id, total_cases, passed_cases, failed_cases,
		 scores, total_tokens, total_latency_ms, total_cost, baseline_result_id,
		 regression_detected, regression_details, status, started_at, completed_at, created_at
		 FROM benchmark_results WHERE suite_id = $1 AND tenant_id = $2 ORDER BY created_at DESC`, suiteID, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	var results []model.BenchmarkResult
	for rows.Next() {
		var r model.BenchmarkResult
		if err := rows.Scan(&r.ID, &r.SuiteID, &r.TenantID, &r.AgentID,
			&r.TotalCases, &r.PassedCases, &r.FailedCases, &r.Scores,
			&r.TotalTokens, &r.TotalLatencyMs, &r.TotalCost, &r.BaselineResultID,
			&r.RegressionDetected, &r.RegressionDetails, &r.Status,
			&r.StartedAt, &r.CompletedAt, &r.CreatedAt); err == nil {
			results = append(results, r)
		}
	}
	if results == nil {
		results = []model.BenchmarkResult{}
	}
	c.JSON(http.StatusOK, results)
}

// ============================================================
// COMPARE
// ============================================================

type CompareRunsRequest struct {
	RunIDA string `json:"run_id_a" binding:"required"`
	RunIDB string `json:"run_id_b" binding:"required"`
}

func (h *Handler) CompareRuns(c *gin.Context) {
	var req CompareRunsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: Implement full comparison logic
	c.JSON(http.StatusOK, gin.H{
		"run_id_a": req.RunIDA,
		"run_id_b": req.RunIDB,
		"status":   "comparison_pending",
		"message":  "Full comparison will be implemented in Phase 6",
	})
}

// ============================================================
// AUDIT
// ============================================================

func (h *Handler) ListAuditLog(c *gin.Context) {
	tenantID := h.tenantID(c)
	rows, err := h.pool.Query(c.Request.Context(),
		`SELECT id, tenant_id, actor_id, action, resource_type, resource_id, details, ip_address, created_at
		 FROM audit_log WHERE tenant_id = $1 ORDER BY created_at DESC LIMIT 100`, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	var entries []model.AuditEntry
	for rows.Next() {
		var e model.AuditEntry
		if err := rows.Scan(&e.ID, &e.TenantID, &e.ActorID, &e.Action, &e.ResourceType, &e.ResourceID,
			&e.Details, &e.IPAddress, &e.CreatedAt); err == nil {
			entries = append(entries, e)
		}
	}
	if entries == nil {
		entries = []model.AuditEntry{}
	}
	c.JSON(http.StatusOK, entries)
}

// ============================================================
// CANONICAL TRACE INGESTION (nested schema → flat run)
// ============================================================

func (h *Handler) IngestTrace(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read body"})
		return
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	// Detect if this is canonical (nested) format by checking for "input" key
	if _, isCanonical := raw["input"]; isCanonical {
		flat, err := canonicalToFlat(body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to convert canonical trace: " + err.Error()})
			return
		}
		// Replace request body with flat format
		c.Request.Body = io.NopCloser(bytes.NewReader(flat))
	} else {
		// Already flat format, restore body
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
	}

	h.IngestRun(c)
}

// canonicalToFlat converts the nested canonical trace schema to the flat IngestRunRequest format
func canonicalToFlat(data []byte) ([]byte, error) {
	var trace struct {
		TenantID string `json:"tenant_id"`
		RunID    string `json:"run_id"`
		User     *struct {
			ID     string   `json:"id"`
			Role   string   `json:"role"`
			Roles  []string `json:"roles"`
			Groups []string `json:"groups"`
		} `json:"user"`
		Agent *struct {
			ID      string `json:"id"`
			Version string `json:"version"`
		} `json:"agent"`
		Input *struct {
			Query    string          `json:"query"`
			Metadata json.RawMessage `json:"metadata"`
		} `json:"input"`
		Retrieval *struct {
			Candidates []struct {
				DocID           string  `json:"doc_id"`
				ChunkID         *string `json:"chunk_id"`
				Title           string  `json:"title"`
				Source          *string `json:"source"`
				ContentPreview  *string `json:"content_preview"`
				Score           float32 `json:"score"`
				Rank            *int    `json:"rank"`
				RetrievalMethod *string `json:"retrieval_method"`
			} `json:"candidates"`
			Selected []string `json:"selected"`
		} `json:"retrieval"`
		Memory *struct {
			Candidates []struct {
				MemoryID       string  `json:"memory_id"`
				Type           string  `json:"type"`
				Summary        string  `json:"summary"`
				Content        string  `json:"content"`
				RelevanceScore float32 `json:"relevance_score"`
				RecencyScore   float32 `json:"recency_score"`
				IsStale        bool    `json:"is_stale"`
				StaleReason    *string `json:"stale_reason"`
			} `json:"candidates"`
			Selected []string `json:"selected"`
		} `json:"memory"`
		Permissions *struct {
			ACLFilters []struct {
				DocID  string `json:"doc_id"`
				Result string `json:"result"`
				Reason string `json:"reason"`
			} `json:"acl_filters"`
		} `json:"permissions"`
		Tools []struct {
			Name      string          `json:"name"`
			Args      json.RawMessage `json:"args"`
			Result    json.RawMessage `json:"result"`
			Status    string          `json:"status"`
			LatencyMs int             `json:"latency_ms"`
		} `json:"tools"`
		ReasoningSteps []struct {
			Step    int    `json:"step"`
			Type    string `json:"type"`
			Content string `json:"content"`
		} `json:"reasoning_steps"`
		Output *struct {
			FinalAnswer string          `json:"final_answer"`
			Citations   json.RawMessage `json:"citations"`
			SideEffects json.RawMessage `json:"side_effects"`
		} `json:"output"`
		ExpectedOutcomes *struct {
			ExpectedAnswer  *string         `json:"expected_answer"`
			ExpectedTools   json.RawMessage `json:"expected_tools"`
			ExpectedSources json.RawMessage `json:"expected_sources"`
		} `json:"expected_outcomes"`
		Metrics *struct {
			LatencyMs        int     `json:"latency_ms"`
			PromptTokens     int     `json:"prompt_tokens"`
			CompletionTokens int     `json:"completion_tokens"`
			TotalTokens      int     `json:"total_tokens"`
			EstimatedCost    float64 `json:"estimated_cost"`
		} `json:"metrics"`
	}

	if err := json.Unmarshal(data, &trace); err != nil {
		return nil, err
	}

	flat := IngestRunRequest{}

	// Input
	if trace.Input != nil {
		flat.Query = trace.Input.Query
		flat.QueryMetadata = trace.Input.Metadata
	}

	// User
	if trace.User != nil {
		flat.UserRoles = trace.User.Roles
		if flat.UserRoles == nil && trace.User.Role != "" {
			flat.UserRoles = []string{trace.User.Role}
		}
		flat.UserGroups = trace.User.Groups
	}

	// Output
	if trace.Output != nil {
		flat.FinalAnswer = &trace.Output.FinalAnswer
		flat.Citations = trace.Output.Citations
		flat.SideEffects = trace.Output.SideEffects
	}

	// Expected outcomes
	if trace.ExpectedOutcomes != nil {
		flat.ExpectedAnswer = trace.ExpectedOutcomes.ExpectedAnswer
		flat.ExpectedTools = trace.ExpectedOutcomes.ExpectedTools
		flat.ExpectedSources = trace.ExpectedOutcomes.ExpectedSources
	}

	// Metrics
	if trace.Metrics != nil {
		flat.LatencyMs = trace.Metrics.LatencyMs
		flat.PromptTokens = trace.Metrics.PromptTokens
		flat.CompletionTokens = trace.Metrics.CompletionTokens
		flat.TotalTokens = trace.Metrics.TotalTokens
		flat.EstimatedCost = trace.Metrics.EstimatedCost
	}

	flat.Status = "completed"

	// Build selected lookup maps
	selectedDocs := map[string]bool{}
	if trace.Retrieval != nil {
		for _, s := range trace.Retrieval.Selected {
			selectedDocs[s] = true
		}
	}
	selectedMems := map[string]bool{}
	if trace.Memory != nil {
		for _, s := range trace.Memory.Selected {
			selectedMems[s] = true
		}
	}

	// ACL filter lookup
	aclResults := map[string]struct {
		Passed bool
		Reason string
	}{}
	if trace.Permissions != nil {
		for _, f := range trace.Permissions.ACLFilters {
			aclResults[f.DocID] = struct {
				Passed bool
				Reason string
			}{Passed: f.Result != "filtered", Reason: f.Reason}
		}
	}

	// Retrieval candidates
	if trace.Retrieval != nil {
		for _, c := range trace.Retrieval.Candidates {
			rc := IngestRetrievalRequest{
				DocID:           c.DocID,
				ChunkID:         c.ChunkID,
				Title:           &c.Title,
				Source:          c.Source,
				ContentPreview:  c.ContentPreview,
				Score:           c.Score,
				Rank:            c.Rank,
				RetrievalMethod: c.RetrievalMethod,
				Selected:        selectedDocs[c.DocID],
				ACLPassed:       true,
			}
			if acl, ok := aclResults[c.DocID]; ok {
				rc.ACLPassed = acl.Passed
				rc.ACLReason = &acl.Reason
			}
			flat.Retrieval = append(flat.Retrieval, rc)
		}
	}

	// Memory candidates
	if trace.Memory != nil {
		for _, m := range trace.Memory.Candidates {
			content := m.Content
			if content == "" {
				content = m.Summary
			}
			memType := m.Type
			if memType == "" {
				memType = "episodic"
			}
			mc := IngestMemoryRequest{
				MemoryID:       m.MemoryID,
				MemoryType:     memType,
				Content:        content,
				RelevanceScore: m.RelevanceScore,
				RecencyScore:   m.RecencyScore,
				IsStale:        m.IsStale,
				StaleReason:    m.StaleReason,
				Selected:       selectedMems[m.MemoryID],
			}
			flat.Memory = append(flat.Memory, mc)
		}
	}

	// Tools
	for _, t := range trace.Tools {
		status := t.Status
		if status == "" {
			status = "success"
		}
		tc := IngestToolCallRequest{
			ToolName:   t.Name,
			ToolArgs:   t.Args,
			ToolResult: t.Result,
			Status:     status,
			LatencyMs:  t.LatencyMs,
		}
		flat.Tools = append(flat.Tools, tc)
	}

	// Reasoning steps
	for _, s := range trace.ReasoningSteps {
		stepType := s.Type
		if stepType == "" {
			stepType = "think"
		}
		rs := IngestStepRequest{
			StepNumber: s.Step,
			StepType:   stepType,
			Content:    s.Content,
		}
		flat.Steps = append(flat.Steps, rs)
	}

	return json.Marshal(flat)
}

// ============================================================
// DATASETS
// ============================================================

type CreateDatasetRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
	Category    *string `json:"category"`
}

func (h *Handler) CreateDataset(c *gin.Context) {
	var req CreateDatasetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenantID := h.tenantID(c)
	id := uuid.New().String()
	now := time.Now()

	_, err := h.pool.Exec(c.Request.Context(),
		`INSERT INTO datasets (id, tenant_id, name, description, category, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, tenantID, req.Name, req.Description, req.Category, now, now,
	)
	if err != nil {
		log.Error().Err(err).Msg("failed to create dataset")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create dataset"})
		return
	}

	h.audit(c, "create", "dataset", id, nil)
	c.JSON(http.StatusCreated, model.Dataset{
		ID: id, TenantID: tenantID, Name: req.Name,
		Description: req.Description, Category: req.Category,
		Config: json.RawMessage(`{}`), CreatedAt: now, UpdatedAt: now,
	})
}

func (h *Handler) ListDatasets(c *gin.Context) {
	tenantID := h.tenantID(c)
	rows, err := h.pool.Query(c.Request.Context(),
		`SELECT id, tenant_id, name, description, category, case_count, config, created_at, updated_at
		 FROM datasets WHERE tenant_id = $1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
		return
	}
	defer rows.Close()

	var datasets []model.Dataset
	for rows.Next() {
		var d model.Dataset
		if err := rows.Scan(&d.ID, &d.TenantID, &d.Name, &d.Description, &d.Category,
			&d.CaseCount, &d.Config, &d.CreatedAt, &d.UpdatedAt); err == nil {
			datasets = append(datasets, d)
		}
	}
	if datasets == nil {
		datasets = []model.Dataset{}
	}
	c.JSON(http.StatusOK, datasets)
}

func (h *Handler) GetDataset(c *gin.Context) {
	id := c.Param("id")
	tenantID := h.tenantID(c)
	var d model.Dataset
	err := h.pool.QueryRow(c.Request.Context(),
		`SELECT id, tenant_id, name, description, category, case_count, config, created_at, updated_at
		 FROM datasets WHERE id = $1 AND tenant_id = $2`, id, tenantID,
	).Scan(&d.ID, &d.TenantID, &d.Name, &d.Description, &d.Category,
		&d.CaseCount, &d.Config, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "dataset not found"})
		return
	}
	c.JSON(http.StatusOK, d)
}

// ============================================================
// HELPERS
// ============================================================

func (h *Handler) fetchRun(ctx context.Context, id, tenantID string) (*model.Run, error) {
	var r model.Run
	err := h.pool.QueryRow(ctx,
		`SELECT id, tenant_id, agent_id, identity_id, query, query_metadata,
		 user_roles, user_groups, model, model_config, final_answer, citations, side_effects,
		 prompt_tokens, completion_tokens, total_tokens, latency_ms, estimated_cost,
		 status, error_message, expected_answer, expected_tools, expected_sources,
		 context_manifest, started_at, completed_at, created_at
		 FROM runs WHERE id = $1 AND tenant_id = $2`, id, tenantID,
	).Scan(&r.ID, &r.TenantID, &r.AgentID, &r.IdentityID, &r.Query, &r.QueryMetadata,
		&r.UserRoles, &r.UserGroups, &r.Model, &r.ModelConfig, &r.FinalAnswer, &r.Citations, &r.SideEffects,
		&r.PromptTokens, &r.CompletionTokens, &r.TotalTokens, &r.LatencyMs, &r.EstimatedCost,
		&r.Status, &r.ErrorMessage, &r.ExpectedAnswer, &r.ExpectedTools, &r.ExpectedSources,
		&r.ContextManifest, &r.StartedAt, &r.CompletedAt, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (h *Handler) audit(c *gin.Context, action, resourceType, resourceID string, details json.RawMessage) {
	tenantID := h.tenantID(c)
	if details == nil {
		details = json.RawMessage(`{}`)
	}
	_, _ = h.pool.Exec(c.Request.Context(),
		`INSERT INTO audit_log (id, tenant_id, actor_id, action, resource_type, resource_id, details, ip_address, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		uuid.New().String(), tenantID, nil, action, resourceType, resourceID, details, c.ClientIP(), time.Now(),
	)
}

func defaultJSON(v json.RawMessage) json.RawMessage {
	if v == nil {
		return json.RawMessage(`{}`)
	}
	return v
}

func defaultSlice(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func itoa(i int) string {
	return string(rune('0' + i)) // works for 1-9
}
