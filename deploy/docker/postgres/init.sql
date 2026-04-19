-- ContextOps Database Schema
-- Full audit trail with tenant isolation

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "vector";

-- ============================================================
-- TENANTS
-- ============================================================
CREATE TABLE tenants (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        TEXT NOT NULL UNIQUE,
    config      JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- IDENTITIES (users/service accounts that interact with agents)
-- ============================================================
CREATE TABLE identities (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    external_id TEXT NOT NULL,
    name        TEXT NOT NULL,
    email       TEXT,
    roles       TEXT[] NOT NULL DEFAULT '{}',
    groups      TEXT[] NOT NULL DEFAULT '{}',
    metadata    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, external_id)
);

CREATE INDEX idx_identities_tenant ON identities(tenant_id);

-- ============================================================
-- AGENTS (registered agent configurations)
-- ============================================================
CREATE TABLE agents (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    version     TEXT NOT NULL DEFAULT 'v1',
    description TEXT,
    config      JSONB NOT NULL DEFAULT '{}',
    tags        TEXT[] NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, name, version)
);

CREATE INDEX idx_agents_tenant ON agents(tenant_id);

-- ============================================================
-- RUNS (the core entity - one per agent invocation)
-- ============================================================
CREATE TABLE runs (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    agent_id        UUID REFERENCES agents(id),
    identity_id     UUID REFERENCES identities(id),
    
    -- Input
    query           TEXT NOT NULL,
    query_metadata  JSONB NOT NULL DEFAULT '{}',
    
    -- Identity context at time of run
    user_roles      TEXT[] NOT NULL DEFAULT '{}',
    user_groups     TEXT[] NOT NULL DEFAULT '{}',
    
    -- Model info
    model           TEXT,
    model_config    JSONB NOT NULL DEFAULT '{}',
    
    -- Output
    final_answer    TEXT,
    citations       JSONB NOT NULL DEFAULT '[]',
    side_effects    JSONB NOT NULL DEFAULT '[]',
    
    -- Cost tracking
    prompt_tokens   INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens    INTEGER NOT NULL DEFAULT 0,
    latency_ms      INTEGER NOT NULL DEFAULT 0,
    estimated_cost  NUMERIC(10, 6) NOT NULL DEFAULT 0,
    
    -- Status
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed', 'timeout')),
    error_message   TEXT,
    
    -- Ground truth (optional, for benchmarking)
    expected_answer TEXT,
    expected_tools  JSONB,
    expected_sources JSONB,
    
    -- Context manifest
    context_manifest JSONB NOT NULL DEFAULT '{}',
    
    -- Timestamps
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_runs_tenant ON runs(tenant_id);
CREATE INDEX idx_runs_agent ON runs(agent_id);
CREATE INDEX idx_runs_identity ON runs(identity_id);
CREATE INDEX idx_runs_status ON runs(tenant_id, status);
CREATE INDEX idx_runs_created ON runs(tenant_id, created_at DESC);

-- ============================================================
-- RETRIEVAL CANDIDATES (docs considered during retrieval)
-- ============================================================
CREATE TABLE retrieval_candidates (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_id          UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    
    -- Document identification
    doc_id          TEXT NOT NULL,
    chunk_id        TEXT,
    title           TEXT,
    source          TEXT,
    content_preview TEXT,
    
    -- Scoring
    score           REAL NOT NULL DEFAULT 0,
    rank            INTEGER,
    retrieval_method TEXT,
    
    -- ACL evaluation
    acl_passed      BOOLEAN NOT NULL DEFAULT true,
    acl_reason      TEXT,
    acl_rules       JSONB NOT NULL DEFAULT '{}',
    
    -- Selection
    selected        BOOLEAN NOT NULL DEFAULT false,
    rejection_reason TEXT,
    
    -- Embedding (for analysis)
    embedding       vector(1536),
    
    -- Metadata
    doc_metadata    JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_retrieval_run ON retrieval_candidates(run_id);
CREATE INDEX idx_retrieval_selected ON retrieval_candidates(run_id, selected);

-- ============================================================
-- MEMORY CANDIDATES (memory items considered during run)
-- ============================================================
CREATE TABLE memory_candidates (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_id          UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    
    -- Memory identification
    memory_id       TEXT NOT NULL,
    memory_type     TEXT NOT NULL CHECK (memory_type IN ('conversation', 'episodic', 'semantic', 'entity', 'preference', 'task')),
    content         TEXT NOT NULL,
    
    -- Scoring
    relevance_score REAL NOT NULL DEFAULT 0,
    recency_score   REAL NOT NULL DEFAULT 0,
    
    -- Staleness
    memory_created_at TIMESTAMPTZ,
    is_stale        BOOLEAN NOT NULL DEFAULT false,
    stale_reason    TEXT,
    
    -- Selection
    selected        BOOLEAN NOT NULL DEFAULT false,
    rejection_reason TEXT,
    
    -- Metadata
    memory_metadata JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_memory_run ON memory_candidates(run_id);
CREATE INDEX idx_memory_selected ON memory_candidates(run_id, selected);

-- ============================================================
-- TOOL CALLS (tools invoked during agent run)
-- ============================================================
CREATE TABLE tool_calls (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_id          UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    
    -- Tool info
    tool_name       TEXT NOT NULL,
    tool_args       JSONB NOT NULL DEFAULT '{}',
    tool_result     JSONB,
    
    -- Execution
    step_number     INTEGER NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'success' CHECK (status IN ('success', 'failure', 'timeout', 'skipped')),
    error_message   TEXT,
    latency_ms      INTEGER NOT NULL DEFAULT 0,
    
    -- Evaluation hints
    was_correct     BOOLEAN,
    expected_tool   TEXT,
    expected_args   JSONB,
    
    -- Safety
    requires_approval BOOLEAN NOT NULL DEFAULT false,
    approval_status TEXT CHECK (approval_status IN ('pending', 'approved', 'rejected')),
    side_effect_type TEXT,
    
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tool_calls_run ON tool_calls(run_id);

-- ============================================================
-- REASONING STEPS (agent chain-of-thought / trajectory)
-- ============================================================
CREATE TABLE reasoning_steps (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_id          UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    
    step_number     INTEGER NOT NULL,
    step_type       TEXT NOT NULL CHECK (step_type IN ('think', 'retrieve', 'remember', 'tool_call', 'tool_result', 'generate', 'decide', 'act')),
    content         TEXT NOT NULL,
    metadata        JSONB NOT NULL DEFAULT '{}',
    
    latency_ms      INTEGER NOT NULL DEFAULT 0,
    tokens_used     INTEGER NOT NULL DEFAULT 0,
    
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reasoning_run ON reasoning_steps(run_id);
CREATE INDEX idx_reasoning_step ON reasoning_steps(run_id, step_number);

-- ============================================================
-- EVALUATIONS (evaluation results per run)
-- ============================================================
CREATE TABLE evaluations (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_id          UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    
    -- Evaluator
    evaluator_name  TEXT NOT NULL,
    evaluator_version TEXT NOT NULL DEFAULT 'v1',
    
    -- Category
    category        TEXT NOT NULL CHECK (category IN (
        'answer_correctness', 'groundedness', 'retrieval_quality',
        'citation_precision', 'permission_safety', 'memory_utility',
        'tool_correctness', 'trajectory_quality', 'task_completion',
        'cost_efficiency', 'context_poisoning', 'session_coherence',
        'hallucination_risk', 'response_completeness', 'agent_regression'
    )),
    
    -- Result
    score           REAL NOT NULL CHECK (score >= 0 AND score <= 1),
    passed          BOOLEAN NOT NULL,
    details         JSONB NOT NULL DEFAULT '{}',
    reasoning       TEXT,
    
    -- Metadata
    model_used      TEXT,
    eval_tokens     INTEGER NOT NULL DEFAULT 0,
    eval_latency_ms INTEGER NOT NULL DEFAULT 0,
    
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_evaluations_run ON evaluations(run_id);
CREATE INDEX idx_evaluations_tenant ON evaluations(tenant_id);
CREATE INDEX idx_evaluations_category ON evaluations(tenant_id, category);
CREATE UNIQUE INDEX idx_evaluations_run_category ON evaluations(run_id, category);

-- ============================================================
-- BENCHMARK SUITES
-- ============================================================
CREATE TABLE benchmark_suites (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT,
    category    TEXT NOT NULL,
    config      JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, name)
);

-- ============================================================
-- BENCHMARK CASES (individual test cases in a suite)
-- ============================================================
CREATE TABLE benchmark_cases (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    suite_id        UUID NOT NULL REFERENCES benchmark_suites(id) ON DELETE CASCADE,
    
    name            TEXT NOT NULL,
    query           TEXT NOT NULL,
    
    -- Expected outcomes
    expected_answer TEXT,
    expected_sources JSONB,
    expected_tools  JSONB,
    expected_trajectory JSONB,
    
    -- Permission simulation
    simulate_user_roles TEXT[] NOT NULL DEFAULT '{}',
    simulate_user_groups TEXT[] NOT NULL DEFAULT '{}',
    
    -- Forbidden content (for permission leakage testing)
    forbidden_sources JSONB NOT NULL DEFAULT '[]',
    forbidden_content TEXT[] NOT NULL DEFAULT '{}',
    
    -- Memory context
    seed_memories   JSONB NOT NULL DEFAULT '[]',
    stale_memories  JSONB NOT NULL DEFAULT '[]',
    
    -- Thresholds
    pass_criteria   JSONB NOT NULL DEFAULT '{}',
    
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_benchmark_cases_suite ON benchmark_cases(suite_id);

-- ============================================================
-- BENCHMARK RESULTS (aggregate results per suite execution)
-- ============================================================
CREATE TABLE benchmark_results (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    suite_id        UUID NOT NULL REFERENCES benchmark_suites(id) ON DELETE CASCADE,
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    agent_id        UUID REFERENCES agents(id),
    
    -- Aggregate scores
    total_cases     INTEGER NOT NULL DEFAULT 0,
    passed_cases    INTEGER NOT NULL DEFAULT 0,
    failed_cases    INTEGER NOT NULL DEFAULT 0,
    
    scores          JSONB NOT NULL DEFAULT '{}',
    
    -- Cost
    total_tokens    INTEGER NOT NULL DEFAULT 0,
    total_latency_ms INTEGER NOT NULL DEFAULT 0,
    total_cost      NUMERIC(10, 6) NOT NULL DEFAULT 0,
    
    -- Comparison
    baseline_result_id UUID REFERENCES benchmark_results(id),
    regression_detected BOOLEAN NOT NULL DEFAULT false,
    regression_details JSONB NOT NULL DEFAULT '{}',
    
    status          TEXT NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'completed', 'failed')),
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_benchmark_results_suite ON benchmark_results(suite_id);
CREATE INDEX idx_benchmark_results_tenant ON benchmark_results(tenant_id);

-- ============================================================
-- DATASETS (evaluation datasets)
-- ============================================================
CREATE TABLE datasets (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT,
    category    TEXT,
    case_count  INTEGER NOT NULL DEFAULT 0,
    config      JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, name)
);

CREATE INDEX idx_datasets_tenant ON datasets(tenant_id);

CREATE TABLE dataset_cases (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    dataset_id  UUID NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    query       TEXT NOT NULL,
    expected_answer TEXT,
    expected_sources JSONB,
    expected_tools  JSONB,
    pass_criteria   JSONB NOT NULL DEFAULT '{}',
    metadata    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dataset_cases_dataset ON dataset_cases(dataset_id);

-- ============================================================
-- AUDIT LOG
-- ============================================================
CREATE TABLE audit_log (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    actor_id    TEXT,
    action      TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id UUID,
    details     JSONB NOT NULL DEFAULT '{}',
    ip_address  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_tenant ON audit_log(tenant_id);
CREATE INDEX idx_audit_created ON audit_log(tenant_id, created_at DESC);
CREATE INDEX idx_audit_resource ON audit_log(resource_type, resource_id);

-- ============================================================
-- SEED DATA
-- ============================================================
INSERT INTO tenants (id, name, config) VALUES
    ('00000000-0000-0000-0000-000000000001', 'default', '{"plan": "free", "max_runs_per_day": 1000}'),
    ('00000000-0000-0000-0000-000000000002', 'acme-corp', '{"plan": "enterprise", "max_runs_per_day": 100000}');

INSERT INTO identities (tenant_id, external_id, name, email, roles, groups) VALUES
    ('00000000-0000-0000-0000-000000000001', 'user-1', 'Alice Developer', 'alice@example.com', ARRAY['admin', 'developer'], ARRAY['engineering']),
    ('00000000-0000-0000-0000-000000000001', 'user-2', 'Bob Analyst', 'bob@example.com', ARRAY['viewer'], ARRAY['analytics']),
    ('00000000-0000-0000-0000-000000000002', 'user-3', 'Carol Manager', 'carol@acme.com', ARRAY['admin'], ARRAY['management', 'hr']);

INSERT INTO agents (tenant_id, name, version, description, tags) VALUES
    ('00000000-0000-0000-0000-000000000001', 'support-agent', 'v1', 'Customer support assistant', ARRAY['support', 'rag']),
    ('00000000-0000-0000-0000-000000000001', 'policy-agent', 'v1', 'Internal policy Q&A agent', ARRAY['policy', 'enterprise']),
    ('00000000-0000-0000-0000-000000000002', 'contract-reviewer', 'v2', 'Contract analysis and review agent', ARRAY['legal', 'documents']);
