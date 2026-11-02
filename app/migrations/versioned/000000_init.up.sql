-- WellRead Personal baseline schema (PostgreSQL).
-- Squashed from the enterprise-era versioned migration chain.
-- Fresh databases only: apply once, then never edit this file in place.

-- ---- source: 000000_init.up.sql ----
-- Migration: 000000_init
-- Description: Initialize database schema
DO $$ BEGIN RAISE NOTICE '[Migration 000000] Starting initial database setup...'; END $$;

-- Create extensions
DO $$ BEGIN RAISE NOTICE '[Migration 000000] Creating extensions...'; END $$;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create tenant table
DO $$ BEGIN RAISE NOTICE '[Migration 000000] Creating table: tenants'; END $$;
CREATE TABLE IF NOT EXISTS tenants (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    api_key VARCHAR(64) NOT NULL,
    retriever_engines JSONB NOT NULL DEFAULT '[]',
    status VARCHAR(50) DEFAULT 'active',
    business VARCHAR(255) NOT NULL,
    storage_quota BIGINT NOT NULL DEFAULT 10737418240, -- 默认10GB配额(Bytes)
    storage_used BIGINT NOT NULL DEFAULT 0, -- 已使用的存储空间(Bytes)
    agent_config JSONB DEFAULT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Set the starting value for tenants id sequence (only if current value is less than 10000)
DO $$
DECLARE
    current_val BIGINT;
BEGIN
    SELECT last_value INTO current_val FROM tenants_id_seq;
    IF current_val < 10000 THEN
        ALTER SEQUENCE tenants_id_seq RESTART WITH 10000;
        RAISE NOTICE '[Migration 000000] Set tenants_id_seq to start at 10000';
    ELSE
        RAISE NOTICE '[Migration 000000] tenants_id_seq already at % (>= 10000), skipping', current_val;
    END IF;
EXCEPTION
    WHEN undefined_table THEN
        -- Sequence doesn't exist yet, will be created with table
        RAISE NOTICE '[Migration 000000] tenants_id_seq not found, will be created with table';
END $$;

-- Add indexes
CREATE INDEX IF NOT EXISTS idx_tenants_api_key ON tenants(api_key);
CREATE INDEX IF NOT EXISTS idx_tenants_status ON tenants(status);

-- Create model table
DO $$ BEGIN RAISE NOTICE '[Migration 000000] Creating table: models'; END $$;
CREATE TABLE IF NOT EXISTS models (
    id VARCHAR(64) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    type VARCHAR(50) NOT NULL,
    source VARCHAR(50) NOT NULL,
    description TEXT,
    parameters JSONB NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT false,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);  

-- Add indexes for models
CREATE INDEX IF NOT EXISTS idx_models_type ON models(type);
CREATE INDEX IF NOT EXISTS idx_models_source ON models(source);

-- Create knowledge_base table
DO $$ BEGIN RAISE NOTICE '[Migration 000000] Creating table: knowledge_bases'; END $$;
CREATE TABLE IF NOT EXISTS knowledge_bases (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    tenant_id INTEGER NOT NULL,
    chunking_config JSONB NOT NULL DEFAULT '{"chunk_size": 512, "chunk_overlap": 50, "split_markers": ["\n\n", "\n", "。"], "keep_separator": true}',
    image_processing_config JSONB NOT NULL DEFAULT '{"enable_multimodal": false, "model_id": ""}',
    embedding_model_id VARCHAR(64) NOT NULL,
    summary_model_id VARCHAR(64) NOT NULL,
    rerank_model_id VARCHAR(64) NOT NULL,
    cos_config JSONB NOT NULL DEFAULT '{}',
    vlm_config JSONB NOT NULL DEFAULT '{}',
    extract_config JSONB NULL DEFAULT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Add indexes for knowledge_bases
CREATE INDEX IF NOT EXISTS idx_knowledge_bases_tenant_id ON knowledge_bases(tenant_id);

-- Create knowledge table
DO $$ BEGIN RAISE NOTICE '[Migration 000000] Creating table: knowledges'; END $$;
CREATE TABLE IF NOT EXISTS knowledges (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    type VARCHAR(50) NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    source VARCHAR(2048) NOT NULL,
    parse_status VARCHAR(50) NOT NULL DEFAULT 'unprocessed',
    enable_status VARCHAR(50) NOT NULL DEFAULT 'enabled',
    embedding_model_id VARCHAR(64),
    file_name VARCHAR(255),
    file_type VARCHAR(50),
    file_size BIGINT,
    file_path TEXT,
    file_hash VARCHAR(64),
    storage_size BIGINT NOT NULL DEFAULT 0, -- 存储大小(Byte)
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Add indexes for knowledge
CREATE INDEX IF NOT EXISTS idx_knowledges_tenant_id ON knowledges(tenant_id);
CREATE INDEX IF NOT EXISTS idx_knowledges_base_id ON knowledges(knowledge_base_id);
CREATE INDEX IF NOT EXISTS idx_knowledges_parse_status ON knowledges(parse_status);
CREATE INDEX IF NOT EXISTS idx_knowledges_enable_status ON knowledges(enable_status);

-- Create session table
DO $$ BEGIN RAISE NOTICE '[Migration 000000] Creating table: sessions'; END $$;
CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL,
    title VARCHAR(255),
    description TEXT,
    knowledge_base_id VARCHAR(36),
    max_rounds INTEGER NOT NULL DEFAULT 5,
    enable_rewrite BOOLEAN NOT NULL DEFAULT true,
    fallback_strategy VARCHAR(255) NOT NULL DEFAULT 'fixed',
    fallback_response TEXT NOT NULL DEFAULT '很抱歉，我暂时无法回答这个问题。',
    keyword_threshold FLOAT NOT NULL DEFAULT 0.5,
    vector_threshold FLOAT NOT NULL DEFAULT 0.5,
    rerank_model_id VARCHAR(64),
    embedding_top_k INTEGER NOT NULL DEFAULT 10,
    rerank_top_k INTEGER NOT NULL DEFAULT 10,
    rerank_threshold FLOAT NOT NULL DEFAULT 0.65,
    summary_model_id VARCHAR(64),
    summary_parameters JSONB NOT NULL DEFAULT '{}',
    agent_config JSONB DEFAULT NULL,
    context_config JSONB DEFAULT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create Index for sessions
CREATE INDEX IF NOT EXISTS idx_sessions_tenant_id ON sessions(tenant_id);


-- Create message table
DO $$ BEGIN RAISE NOTICE '[Migration 000000] Creating table: messages'; END $$;
CREATE TABLE IF NOT EXISTS messages (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    request_id VARCHAR(36) NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    role VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    knowledge_references JSONB NOT NULL DEFAULT '[]',
    agent_steps JSONB DEFAULT NULL,
    is_completed BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create Index for messages
CREATE INDEX IF NOT EXISTS idx_messages_session_id ON messages(session_id); 

-- Create chunks table
DO $$ BEGIN RAISE NOTICE '[Migration 000000] Creating table: chunks'; END $$;
CREATE TABLE IF NOT EXISTS chunks (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    knowledge_id VARCHAR(36) NOT NULL,
    content TEXT NOT NULL,
    chunk_index INTEGER NOT NULL,
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    start_at INTEGER NOT NULL,
    end_at INTEGER NOT NULL,
    pre_chunk_id VARCHAR(36),
    next_chunk_id VARCHAR(36),
    chunk_type VARCHAR(20) NOT NULL DEFAULT 'text',
    parent_chunk_id VARCHAR(36),
    image_info TEXT,
    relation_chunks JSONB,
    indirect_relation_chunks JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_chunks_tenant_kg ON chunks(tenant_id, knowledge_id);
CREATE INDEX IF NOT EXISTS idx_chunks_parent_id ON chunks(parent_chunk_id);
CREATE INDEX IF NOT EXISTS idx_chunks_chunk_type ON chunks(chunk_type);

DO $$ BEGIN RAISE NOTICE '[Migration 000000] Initial database setup completed successfully!'; END $$;

-- ---- source: 000001_agent.up.sql ----
-- Migration: 000001_agent
-- Description: Add user authentication, agent config, MCP services and other enhancements

DO $$ BEGIN RAISE NOTICE '[Migration 000001] Starting agent and authentication migration...'; END $$;

-- ============================================================================
-- Section 1: User Authentication Tables
-- ============================================================================

DO $$ BEGIN RAISE NOTICE '[Migration 000001] Creating table: users'; END $$;
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    username VARCHAR(100) NOT NULL,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    avatar VARCHAR(500),
    tenant_id INTEGER,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Add unique constraints if not exists
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'users_username_key') THEN
        ALTER TABLE users ADD CONSTRAINT users_username_key UNIQUE (username);
        RAISE NOTICE '[Migration 000001] Added unique constraint on users.username';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'users_email_key') THEN
        ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email);
        RAISE NOTICE '[Migration 000001] Added unique constraint on users.email';
    END IF;
END $$;

COMMENT ON TABLE users IS 'User accounts in the system';
COMMENT ON COLUMN users.id IS 'Unique identifier of the user';
COMMENT ON COLUMN users.username IS 'Username of the user';
COMMENT ON COLUMN users.email IS 'Email address of the user';
COMMENT ON COLUMN users.password_hash IS 'Hashed password of the user';
COMMENT ON COLUMN users.avatar IS 'Avatar URL of the user';
COMMENT ON COLUMN users.tenant_id IS 'Tenant ID that the user belongs to';
COMMENT ON COLUMN users.is_active IS 'Whether the user is active';

-- Add indexes for users
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_tenant_id ON users(tenant_id);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);

-- Add foreign key constraint for tenant_id
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_users_tenant') THEN
        ALTER TABLE users ADD CONSTRAINT fk_users_tenant
            FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE SET NULL;
        RAISE NOTICE '[Migration 000001] Added foreign key constraint fk_users_tenant';
    END IF;
END $$;

-- Add can_access_all_tenants column to users
ALTER TABLE users ADD COLUMN IF NOT EXISTS can_access_all_tenants BOOLEAN NOT NULL DEFAULT FALSE;

DO $$ BEGIN RAISE NOTICE '[Migration 000001] Creating table: auth_tokens'; END $$;
CREATE TABLE IF NOT EXISTS auth_tokens (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id VARCHAR(36) NOT NULL,
    token TEXT NOT NULL,
    token_type VARCHAR(50) NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    is_revoked BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE auth_tokens IS 'Authentication tokens for users';
COMMENT ON COLUMN auth_tokens.id IS 'Unique identifier of the token';
COMMENT ON COLUMN auth_tokens.user_id IS 'User ID that owns this token';
COMMENT ON COLUMN auth_tokens.token IS 'Token value (JWT or other format)';
COMMENT ON COLUMN auth_tokens.token_type IS 'Token type (access_token, refresh_token)';
COMMENT ON COLUMN auth_tokens.expires_at IS 'Token expiration time';
COMMENT ON COLUMN auth_tokens.is_revoked IS 'Whether the token is revoked';

-- Add indexes for auth_tokens
CREATE INDEX IF NOT EXISTS idx_auth_tokens_user_id ON auth_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_auth_tokens_token ON auth_tokens(token);
CREATE INDEX IF NOT EXISTS idx_auth_tokens_token_type ON auth_tokens(token_type);
CREATE INDEX IF NOT EXISTS idx_auth_tokens_expires_at ON auth_tokens(expires_at);

-- Add foreign key constraint for auth_tokens
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_auth_tokens_user') THEN
        ALTER TABLE auth_tokens ADD CONSTRAINT fk_auth_tokens_user
            FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
        RAISE NOTICE '[Migration 000001] Added foreign key constraint fk_auth_tokens_user';
    END IF;
END $$;

-- ============================================================================
-- Section 2: Tenant Configuration Enhancements
-- ============================================================================

DO $$ BEGIN RAISE NOTICE '[Migration 000001] Adding tenant configuration columns...'; END $$;

-- Add context_config column to tenants
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS context_config JSONB;
COMMENT ON COLUMN tenants.context_config IS 'Global Context configuration for this tenant (default for all sessions)';

-- Add conversation_config column to tenants
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS conversation_config JSONB;
COMMENT ON COLUMN tenants.conversation_config IS 'Global Conversation configuration for this tenant (default for normal mode sessions)';

-- Add web_search_config column to tenants
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS web_search_config JSONB DEFAULT NULL;
COMMENT ON COLUMN tenants.web_search_config IS 'Web search configuration for the tenant';

-- Ensure agent_config exists and is JSONB type
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'tenants' AND column_name = 'agent_config'
    ) THEN
        ALTER TABLE tenants ADD COLUMN agent_config JSONB DEFAULT NULL;
        RAISE NOTICE '[Migration 000001] Added agent_config column to tenants table';
    ELSE
        -- If field exists but type is JSON, convert to JSONB
        IF EXISTS (
            SELECT 1 FROM information_schema.columns 
            WHERE table_name = 'tenants' AND column_name = 'agent_config' AND data_type = 'json'
        ) THEN
            ALTER TABLE tenants ALTER COLUMN agent_config TYPE JSONB USING agent_config::jsonb;
            RAISE NOTICE '[Migration 000001] Converted tenants.agent_config from JSON to JSONB';
        END IF;
    END IF;
END $$;
COMMENT ON COLUMN tenants.agent_config IS 'Tenant-level agent configuration in JSON format';

-- ============================================================================
-- Section 3: Session Configuration Enhancements
-- ============================================================================

DO $$ BEGIN RAISE NOTICE '[Migration 000001] Adding session configuration columns...'; END $$;

-- Add agent_config column to sessions
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS agent_config JSONB DEFAULT NULL;
COMMENT ON COLUMN sessions.agent_config IS 'Session-level agent configuration in JSON format';

-- Add context_config column to sessions
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS context_config JSONB DEFAULT NULL;
COMMENT ON COLUMN sessions.context_config IS 'LLM context management configuration (separate from message storage)';

-- ============================================================================
-- Section 4: Messages Enhancements
-- ============================================================================

DO $$ BEGIN RAISE NOTICE '[Migration 000001] Adding messages enhancements...'; END $$;

-- Add agent_steps column to messages
ALTER TABLE messages ADD COLUMN IF NOT EXISTS agent_steps JSONB DEFAULT NULL;
COMMENT ON COLUMN messages.agent_steps IS 'Agent execution steps (reasoning process and tool calls)';

-- ============================================================================
-- Section 5: Knowledge Base Enhancements
-- ============================================================================

DO $$ BEGIN RAISE NOTICE '[Migration 000001] Adding knowledge base enhancements...'; END $$;

-- Add is_temporary column to knowledge_bases
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS is_temporary BOOLEAN NOT NULL DEFAULT false;
COMMENT ON COLUMN knowledge_bases.is_temporary IS 'Whether this knowledge base is temporary (ephemeral) and should be hidden from UI';

-- Add type and faq_config columns
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS type VARCHAR(32) NOT NULL DEFAULT 'document';
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS faq_config JSONB;

-- Add question_generation_config column
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS question_generation_config JSONB NULL;

-- Update existing rows with default type
UPDATE knowledge_bases SET type = 'document' WHERE type IS NULL OR type = '';

-- Drop rerank_model_id column if exists (moved to session level)
ALTER TABLE knowledge_bases DROP COLUMN IF EXISTS rerank_model_id;

-- ============================================================================
-- Section 6: Knowledges Enhancements
-- ============================================================================

DO $$ BEGIN RAISE NOTICE '[Migration 000001] Adding knowledges enhancements...'; END $$;

-- Add tag_id column
ALTER TABLE knowledges ADD COLUMN IF NOT EXISTS tag_id VARCHAR(36);
CREATE INDEX IF NOT EXISTS idx_knowledges_tag ON knowledges(tag_id);

-- Add summary_status column
ALTER TABLE knowledges ADD COLUMN IF NOT EXISTS summary_status VARCHAR(32) DEFAULT 'none';
CREATE INDEX IF NOT EXISTS idx_knowledges_summary_status ON knowledges(summary_status);

-- ============================================================================
-- Section 7: Chunks Enhancements
-- ============================================================================

DO $$ BEGIN RAISE NOTICE '[Migration 000001] Adding chunks enhancements...'; END $$;

-- Add metadata column
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS metadata JSONB;

-- Add tag_id column
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS tag_id VARCHAR(36);
CREATE INDEX IF NOT EXISTS idx_chunks_tag ON chunks(tag_id);

-- Add status field to track chunk processing state
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS status INT NOT NULL DEFAULT 0;

-- Add content_hash field for quick content matching
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS content_hash VARCHAR(64);
CREATE INDEX IF NOT EXISTS idx_chunks_content_hash ON chunks(content_hash);

-- ============================================================================
-- Section 8: Embeddings Enhancements
-- ============================================================================

-- move embeddings to 000002 migrations

-- ============================================================================
-- Section 9: Models Enhancements
-- ============================================================================

DO $$ BEGIN RAISE NOTICE '[Migration 000001] Adding models enhancements...'; END $$;

-- Add is_builtin column
ALTER TABLE models ADD COLUMN IF NOT EXISTS is_builtin BOOLEAN NOT NULL DEFAULT false;
CREATE INDEX IF NOT EXISTS idx_models_is_builtin ON models(is_builtin);

-- ============================================================================
-- Section 10: Knowledge Tags Table
-- ============================================================================

DO $$ BEGIN RAISE NOTICE '[Migration 000001] Creating table: knowledge_tags'; END $$;
CREATE TABLE IF NOT EXISTS knowledge_tags (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    name VARCHAR(128) NOT NULL,
    color VARCHAR(32),
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_knowledge_tags_kb_name ON knowledge_tags(tenant_id, knowledge_base_id, name);
CREATE INDEX IF NOT EXISTS idx_knowledge_tags_kb ON knowledge_tags(tenant_id, knowledge_base_id);

-- ============================================================================
-- Section 11: MCP Services Table
-- ============================================================================

DO $$ BEGIN RAISE NOTICE '[Migration 000001] Creating table: mcp_services'; END $$;
CREATE TABLE IF NOT EXISTS mcp_services (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    enabled BOOLEAN DEFAULT true,
    transport_type VARCHAR(50) NOT NULL,
    url VARCHAR(512),
    headers JSONB,
    auth_config JSONB,
    advanced_config JSONB,
    stdio_config JSONB,
    env_vars JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_mcp_services_tenant_id ON mcp_services(tenant_id);
CREATE INDEX IF NOT EXISTS idx_mcp_services_enabled ON mcp_services(enabled);
CREATE INDEX IF NOT EXISTS idx_mcp_services_deleted_at ON mcp_services(deleted_at);

COMMENT ON TABLE mcp_services IS 'MCP service configurations';

-- Create or replace trigger function for updated_at
CREATE OR REPLACE FUNCTION update_mcp_services_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger if not exists
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trigger_mcp_services_updated_at') THEN
        CREATE TRIGGER trigger_mcp_services_updated_at
            BEFORE UPDATE ON mcp_services
            FOR EACH ROW
            EXECUTE FUNCTION update_mcp_services_updated_at();
        RAISE NOTICE '[Migration 000001] Created trigger trigger_mcp_services_updated_at';
    END IF;
END $$;

-- ============================================================================
-- Section 12: GIN Indexes for JSONB Fields
-- ============================================================================

DO $$ BEGIN RAISE NOTICE '[Migration 000001] Creating GIN indexes for JSONB fields...'; END $$;

DO $$
BEGIN
    -- For tenants.agent_config
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_tenants_agent_config') THEN
        IF EXISTS (
            SELECT 1 FROM information_schema.columns 
            WHERE table_name = 'tenants' AND column_name = 'agent_config' AND data_type = 'jsonb'
        ) THEN
            CREATE INDEX idx_tenants_agent_config ON tenants USING GIN (agent_config);
            RAISE NOTICE '[Migration 000001] Created index idx_tenants_agent_config';
        END IF;
    END IF;
    
    -- For sessions.agent_config
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_sessions_agent_config') THEN
        CREATE INDEX idx_sessions_agent_config ON sessions USING GIN (agent_config);
        RAISE NOTICE '[Migration 000001] Created index idx_sessions_agent_config';
    END IF;
    
    -- For sessions.context_config
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_sessions_context_config') THEN
        CREATE INDEX idx_sessions_context_config ON sessions USING GIN (context_config);
        RAISE NOTICE '[Migration 000001] Created index idx_sessions_context_config';
    END IF;
    
    -- For messages.agent_steps
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_messages_agent_steps') THEN
        CREATE INDEX idx_messages_agent_steps ON messages USING GIN (agent_steps);
        RAISE NOTICE '[Migration 000001] Created index idx_messages_agent_steps';
    END IF;
END $$;

-- ============================================================================
-- Section 13: Data Migrations
-- ============================================================================

DO $$ BEGIN RAISE NOTICE '[Migration 000001] Running data migrations...'; END $$;

-- Clean up unreferenced models (soft delete)
DO $$
DECLARE
    affected_rows INTEGER;
BEGIN
    WITH referenced_models AS (
        SELECT embedding_model_id AS model_id FROM knowledge_bases WHERE deleted_at IS NULL AND embedding_model_id != ''
        UNION
        SELECT summary_model_id FROM knowledge_bases WHERE deleted_at IS NULL AND summary_model_id != ''
        UNION
        SELECT vlm_config ->> 'model_id'
        FROM knowledge_bases
        WHERE deleted_at IS NULL
          AND vlm_config ->> 'model_id' IS NOT NULL
          AND vlm_config ->> 'model_id' != ''
        UNION
        SELECT embedding_model_id FROM knowledges WHERE deleted_at IS NULL AND embedding_model_id IS NOT NULL AND embedding_model_id != ''
    )
    UPDATE models m
    SET deleted_at = CURRENT_TIMESTAMP
    WHERE m.deleted_at IS NULL
      AND m.is_default = FALSE
      AND m.tenant_id != 0
      AND m.id NOT IN (SELECT model_id FROM referenced_models WHERE model_id IS NOT NULL);
    
    GET DIAGNOSTICS affected_rows = ROW_COUNT;
    IF affected_rows > 0 THEN
        RAISE NOTICE '[Migration 000001] Soft deleted % unreferenced models', affected_rows;
    END IF;
END $$;

-- Update models tenant_id from knowledge_bases references
DO $$
DECLARE
    affected_rows INTEGER;
BEGIN
    WITH tenant_source AS (
        SELECT kb.embedding_model_id AS model_id, kb.tenant_id
        FROM knowledge_bases kb
        WHERE kb.tenant_id IS NOT NULL AND kb.embedding_model_id IS NOT NULL AND kb.embedding_model_id <> ''
        UNION
        SELECT kb.summary_model_id AS model_id, kb.tenant_id
        FROM knowledge_bases kb
        WHERE kb.tenant_id IS NOT NULL AND kb.summary_model_id IS NOT NULL AND kb.summary_model_id <> ''
    )
    UPDATE models m
    SET tenant_id = ts.tenant_id
    FROM tenant_source ts
    WHERE m.id = ts.model_id AND m.tenant_id = 0;
    
    GET DIAGNOSTICS affected_rows = ROW_COUNT;
    IF affected_rows > 0 THEN
        RAISE NOTICE '[Migration 000001] Updated tenant_id for % models', affected_rows;
    END IF;
END $$;

DO $$ BEGIN RAISE NOTICE '[Migration 000001] Migration completed successfully!'; END $$;

-- ---- source: 000002_embeddings.up.sql ----
-- Migration: embeddings (conditional)
-- Description: Create embeddings table and indexes (only for postgres retrieve driver)

DO $$
BEGIN
    -- Check if we should skip this migration
    IF current_setting('app.skip_embedding', true) = 'true' THEN
        RAISE NOTICE 'Skipping migration embeddings (app.skip_embedding=true)';
        RETURN;
    END IF;

    -- If we reach here, proceed with migration
    RAISE NOTICE '[Conditional Migration: embeddings] Creating embeddings table...';

    -- Create required extensions
    CREATE EXTENSION IF NOT EXISTS vector;
    CREATE EXTENSION IF NOT EXISTS pg_trgm;
    CREATE EXTENSION IF NOT EXISTS pg_search;

    -- Create embeddings table
    RAISE NOTICE '[Conditional Migration: embeddings] Creating indexes for embeddings (this may take a while)...';
    
    CREATE TABLE IF NOT EXISTS embeddings (
        id SERIAL PRIMARY KEY,
        created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

        source_id VARCHAR(64) NOT NULL,
        source_type INTEGER NOT NULL,
        chunk_id VARCHAR(64),
        knowledge_id VARCHAR(64),
        knowledge_base_id VARCHAR(64),
        content TEXT,
        dimension INTEGER NOT NULL,
        embedding halfvec
    );

    CREATE UNIQUE INDEX IF NOT EXISTS embeddings_unique_source ON embeddings(source_id, source_type);

    -- Create BM25 search index (check if exists first)
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'embeddings_search_idx') THEN
        CREATE INDEX embeddings_search_idx ON embeddings
        USING bm25 (id, knowledge_base_id, content, knowledge_id, chunk_id)
        WITH (
            key_field = 'id',
            text_fields = '{
                "content": {
                  "tokenizer": {"type": "chinese_lindera"}
                }
            }'
        );
        RAISE NOTICE '[Conditional Migration: embeddings] Created BM25 index embeddings_search_idx';
    ELSE
        RAISE NOTICE '[Conditional Migration: embeddings] BM25 index embeddings_search_idx already exists';
    END IF;

    -- Create HNSW indexes for vector search (check if exists first)
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'embeddings_embedding_idx' OR indexname LIKE 'embeddings_embedding%3584%') THEN
        CREATE INDEX embeddings_embedding_idx_3584 ON embeddings 
        USING hnsw ((embedding::halfvec(3584)) halfvec_cosine_ops) 
        WITH (m = 16, ef_construction = 64) 
        WHERE (dimension = 3584);
        RAISE NOTICE '[Conditional Migration: embeddings] Created HNSW index for dimension 3584';
    ELSE
        RAISE NOTICE '[Conditional Migration: embeddings] HNSW index for dimension 3584 already exists';
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'embeddings_embedding_idx_798' OR indexname LIKE 'embeddings_embedding%798%') THEN
        CREATE INDEX embeddings_embedding_idx_798 ON embeddings 
        USING hnsw ((embedding::halfvec(798)) halfvec_cosine_ops) 
        WITH (m = 16, ef_construction = 64) 
        WHERE (dimension = 798);
        RAISE NOTICE '[Conditional Migration: embeddings] Created HNSW index for dimension 798';
    ELSE
        RAISE NOTICE '[Conditional Migration: embeddings] HNSW index for dimension 798 already exists';
    END IF;

    RAISE NOTICE '[Migration 000002] Adding embeddings enhancements...';

    -- Add is_enabled column
    ALTER TABLE embeddings ADD COLUMN IF NOT EXISTS is_enabled BOOLEAN DEFAULT TRUE;
    CREATE INDEX IF NOT EXISTS idx_embeddings_is_enabled ON embeddings(is_enabled);

    -- Add index for knowledge_base_id
    CREATE INDEX IF NOT EXISTS idx_embeddings_knowledge_base_id ON embeddings(knowledge_base_id);

    -- Reindex BM25 search index (idempotent - will rebuild if exists)
    IF EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'embeddings_search_idx') THEN
        REINDEX INDEX embeddings_search_idx;
        RAISE NOTICE '[Migration 000002] Reindexed embeddings_search_idx';
    END IF;

    RAISE NOTICE '[Conditional Migration: embeddings] Embeddings table setup completed successfully!';
END $$;

-- ---- source: 000003_chunk_flags.up.sql ----
-- Migration: chunk_flags
-- Description: Add flags column to chunks table for managing multiple boolean states

DO $$ 
BEGIN
    RAISE NOTICE '[Migration 000003] Adding flags column to chunks table...';
    
    -- Add flags column with default value 1 (ChunkFlagRecommended)
    -- This means all existing chunks will be recommended by default
    ALTER TABLE chunks ADD COLUMN IF NOT EXISTS flags INTEGER NOT NULL DEFAULT 1;
    
    RAISE NOTICE '[Migration 000003] Flags column added successfully';
END $$;

-- ---- source: 000004_drop_vlm_model_id.up.sql ----
-- Migration: drop_vlm_model_id
-- Description: Drop vlm_model_id column from knowledge_bases table (field moved to vlm_config JSON)

DO $$ 
BEGIN
    RAISE NOTICE '[Migration 000004] Dropping vlm_model_id column from knowledge_bases table...';
    
    -- Drop vlm_model_id column if exists (this field was moved to vlm_config JSON)
    ALTER TABLE knowledge_bases DROP COLUMN IF EXISTS vlm_model_id;
    
    RAISE NOTICE '[Migration 000004] vlm_model_id column dropped successfully';
END $$;

-- ---- source: 000005_mentioned_items.up.sql ----
-- Add mentioned_items column to messages table
-- This column stores @mentioned knowledge bases and files when user sends a message

ALTER TABLE messages ADD COLUMN IF NOT EXISTS mentioned_items JSONB DEFAULT '[]';

-- Add comment for the column
COMMENT ON COLUMN messages.mentioned_items IS 'Stores @mentioned knowledge bases and files (id, name, type) when user sends a message';

-- ---- source: 000006_custom_agents.up.sql ----
-- Migration: 000006_custom_agents
-- Description: Add custom agents table for GPTs-like agent configuration and migrate tenant config to built-in agents
DO $$ BEGIN RAISE NOTICE '[Migration 000006] Starting custom agents setup...'; END $$;

-- Create custom_agents table with composite primary key (id, tenant_id)
-- This allows the same agent ID to exist for different tenants (e.g., 'builtin-normal' for each tenant)
DO $$ BEGIN RAISE NOTICE '[Migration 000006] Creating table: custom_agents'; END $$;
CREATE TABLE IF NOT EXISTS custom_agents (
    id VARCHAR(36) NOT NULL DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    avatar VARCHAR(64),
    is_builtin BOOLEAN NOT NULL DEFAULT false,
    tenant_id INTEGER NOT NULL,
    created_by VARCHAR(36),
    config JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    PRIMARY KEY (id, tenant_id)
);

-- Add indexes for custom_agents
CREATE INDEX IF NOT EXISTS idx_custom_agents_tenant_id ON custom_agents(tenant_id);
CREATE INDEX IF NOT EXISTS idx_custom_agents_is_builtin ON custom_agents(is_builtin);
CREATE INDEX IF NOT EXISTS idx_custom_agents_deleted_at ON custom_agents(deleted_at);

-- Add agent_id column to sessions table to track which agent was used
DO $$ BEGIN RAISE NOTICE '[Migration 000006] Adding agent_id column to sessions table'; END $$;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS agent_id VARCHAR(36);
CREATE INDEX IF NOT EXISTS idx_sessions_agent_id ON sessions(agent_id);

-- Helper function to unify prompt placeholders from Go template format to simple format
CREATE OR REPLACE FUNCTION unify_prompt_placeholder(input TEXT) RETURNS TEXT AS $$
DECLARE
    result TEXT := COALESCE(input, '');
    replacements TEXT[][] := ARRAY[
        -- Go template variables -> simple placeholders
        ['{{.Query}}', '{{query}}'],
        ['{{.Answer}}', '{{answer}}'],
        ['{{.CurrentTime}}', '{{current_time}}'],
        ['{{.CurrentWeek}}', '{{current_week}}'],
        ['{{.Yesterday}}', '{{yesterday}}'],
        ['{{.Contexts}}', '{{contexts}}'],
        -- Go template control structures -> simple placeholders or remove
        ['{{range .Contexts}}', '{{contexts}}'],
        -- Remove Go template syntax
        ['{{if .Contexts}}', ''],
        ['{{else}}', ''],
        ['{{.}}', '']
    ];
    r TEXT[];
BEGIN
    FOREACH r SLICE 1 IN ARRAY replacements LOOP
        result := REPLACE(result, r[1], r[2]);
    END LOOP;
    
    -- Handle {{range .Conversation}}...{{end}} block specially
    -- Replace the entire block with just {{conversation}}
    -- The pattern matches: {{range .Conversation}} followed by any content until {{end}}
    result := regexp_replace(
        result,
        '\{\{range \.Conversation\}\}[\s\S]*?\{\{end\}\}',
        '{{conversation}}',
        'g'
    );
    
    -- Clean up any remaining {{end}} tags
    result := REPLACE(result, '{{end}}', '');
    
    RETURN result;
END;
$$ LANGUAGE plpgsql;

-- Migrate tenant AgentConfig and ConversationConfig to built-in custom agents
DO $$ BEGIN RAISE NOTICE '[Migration 000006] Migrating tenant config to built-in agents...'; END $$;

-- Insert builtin-quick-answer agent for tenants with ConversationConfig
INSERT INTO custom_agents (id, name, description, avatar, is_builtin, tenant_id, config, created_at, updated_at)
SELECT 
    'builtin-quick-answer',
    '快速问答',
    '基于知识库的 RAG 问答，快速准确地回答问题',
    '💬',
    true,
    t.id,
    jsonb_build_object(
        'agent_mode', 'quick-answer',
        'system_prompt', unify_prompt_placeholder(t.conversation_config->>'prompt'),
        'context_template', unify_prompt_placeholder(t.conversation_config->>'context_template'),
        'model_id', COALESCE(t.conversation_config->>'summary_model_id', ''),
        'rerank_model_id', COALESCE(t.conversation_config->>'rerank_model_id', ''),
        'temperature', COALESCE((t.conversation_config->>'temperature')::float, 0.7),
        'max_completion_tokens', COALESCE((t.conversation_config->>'max_completion_tokens')::int, 2048),
        'max_iterations', 10,
        'allowed_tools', '[]'::jsonb,
        'reflection_enabled', false,
        'kb_selection_mode', 'all',
        'knowledge_bases', '[]'::jsonb,
        'web_search_enabled', false,
        'web_search_max_results', COALESCE((t.web_search_config->>'max_results')::int, 5),
        'multi_turn_enabled', COALESCE((t.conversation_config->>'multi_turn_enabled')::bool, true),
        'history_turns', COALESCE((t.conversation_config->>'max_rounds')::int, 5),
        'embedding_top_k', COALESCE((t.conversation_config->>'embedding_top_k')::int, 10),
        'keyword_threshold', COALESCE((t.conversation_config->>'keyword_threshold')::float, 0.3),
        'vector_threshold', COALESCE((t.conversation_config->>'vector_threshold')::float, 0.5),
        'rerank_top_k', COALESCE((t.conversation_config->>'rerank_top_k')::int, 5),
        'rerank_threshold', COALESCE((t.conversation_config->>'rerank_threshold')::float, 0.5),
        'enable_query_expansion', COALESCE((t.conversation_config->>'enable_query_expansion')::bool, true),
        'enable_rewrite', COALESCE((t.conversation_config->>'enable_rewrite')::bool, true),
        'rewrite_prompt_system', unify_prompt_placeholder(t.conversation_config->>'rewrite_prompt_system'),
        'rewrite_prompt_user', unify_prompt_placeholder(t.conversation_config->>'rewrite_prompt_user'),
        'fallback_strategy', COALESCE(t.conversation_config->>'fallback_strategy', 'model'),
        'fallback_response', unify_prompt_placeholder(t.conversation_config->>'fallback_response'),
        'fallback_prompt', unify_prompt_placeholder(t.conversation_config->>'fallback_prompt')
    ),
    NOW(),
    NOW()
FROM tenants t
WHERE t.conversation_config IS NOT NULL
  AND t.deleted_at IS NULL
ON CONFLICT (id, tenant_id) DO UPDATE SET
    config = EXCLUDED.config,
    updated_at = NOW();

-- Insert builtin-smart-reasoning agent for tenants with AgentConfig
INSERT INTO custom_agents (id, name, description, avatar, is_builtin, tenant_id, config, created_at, updated_at)
SELECT 
    'builtin-smart-reasoning',
    '智能推理',
    'ReAct 推理框架，支持多步思考和工具调用',
    '🤖',
    true,
    t.id,
    jsonb_build_object(
        'agent_mode', 'smart-reasoning',
        'system_prompt', unify_prompt_placeholder(t.agent_config->>'system_prompt_web_disabled'),
        'system_prompt_web_enabled', unify_prompt_placeholder(t.agent_config->>'system_prompt_web_enabled'),
        'context_template', '',
        'model_id', COALESCE(t.conversation_config->>'summary_model_id', ''),
        'rerank_model_id', COALESCE(t.conversation_config->>'rerank_model_id', ''),
        'temperature', COALESCE((t.agent_config->>'temperature')::float, 0.7),
        'max_completion_tokens', 2048,
        'max_iterations', COALESCE((t.agent_config->>'max_iterations')::int, 50),
        'allowed_tools', COALESCE(t.agent_config->'allowed_tools', '["thinking", "todo_write", "knowledge_search", "grep_chunks", "list_knowledge_chunks", "query_knowledge_graph", "get_document_info"]'::jsonb),
        'reflection_enabled', COALESCE((t.agent_config->>'reflection_enabled')::bool, false),
        'mcp_selection_mode', 'all',
        'mcp_services', '[]'::jsonb,
        'kb_selection_mode', 'all',
        'knowledge_bases', COALESCE(t.agent_config->'knowledge_bases', '[]'::jsonb),
        'web_search_enabled', COALESCE((t.agent_config->>'web_search_enabled')::bool, true),
        'web_search_max_results', COALESCE((t.agent_config->>'web_search_max_results')::int, COALESCE((t.web_search_config->>'max_results')::int, 5)),
        'multi_turn_enabled', COALESCE((t.agent_config->>'multi_turn_enabled')::bool, true),
        'history_turns', COALESCE((t.agent_config->>'history_turns')::int, 5),
        'embedding_top_k', 10,
        'keyword_threshold', 0.3,
        'vector_threshold', 0.5,
        'rerank_top_k', 5,
        'rerank_threshold', 0.5,
        'enable_query_expansion', false,
        'enable_rewrite', false,
        'rewrite_prompt_system', '',
        'rewrite_prompt_user', '',
        'fallback_strategy', 'model',
        'fallback_response', '',
        'fallback_prompt', ''
    ),
    NOW(),
    NOW()
FROM tenants t
WHERE t.agent_config IS NOT NULL
  AND t.deleted_at IS NULL
ON CONFLICT (id, tenant_id) DO UPDATE SET
    config = EXCLUDED.config,
    updated_at = NOW();

-- ---- source: 000007_embeddings_tag_id.up.sql ----
-- Add tag_id column to embeddings table for FAQ priority filtering
DO $$
BEGIN
    -- Check if table exists first
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'embeddings') THEN
        -- Add tag_id column if not exists
        IF NOT EXISTS (
            SELECT 1 FROM information_schema.columns 
            WHERE table_name = 'embeddings' AND column_name = 'tag_id'
        ) THEN
            ALTER TABLE embeddings ADD COLUMN tag_id VARCHAR(36);
            CREATE INDEX IF NOT EXISTS idx_embeddings_tag_id ON embeddings(tag_id);
            RAISE NOTICE '[Migration 000007] Added tag_id column and index to embeddings table';
        ELSE
            RAISE NOTICE '[Migration 000007] tag_id column already exists in embeddings table, skipping';
        END IF;
    ELSE
        RAISE NOTICE '[Migration 000007] embeddings table does not exist, skipping';
    END IF;
END $$;

-- ---- source: 000008_migrate_untagged_faq.up.sql ----
-- Migration: Create "未分类" tag for each knowledge base that has untagged entries
-- and update chunks, knowledges, and embeddings to reference the new tag
DO $$
BEGIN
    IF current_setting('app.skip_embedding', true) = 'true' THEN
        RAISE NOTICE 'Skipping pg_search update (app.skip_embedding=true)';
        RETURN;
    END IF;

    ALTER EXTENSION pg_search UPDATE;
END $$;

DO $$
DECLARE
    kb_record RECORD;
    new_tag_id VARCHAR(36);
    updated_chunks INT;
    updated_knowledges INT;
    updated_embeddings INT;
BEGIN
    -- Find all knowledge bases that have untagged chunks or knowledges
    FOR kb_record IN 
        SELECT DISTINCT tenant_id, knowledge_base_id 
        FROM (
            -- Untagged chunks (FAQ or document)
            SELECT c.tenant_id, c.knowledge_base_id 
            FROM chunks c
            WHERE c.chunk_type IN ('faq', 'document')
            AND (c.tag_id = '' OR c.tag_id IS NULL)
            UNION
            -- Untagged knowledges (documents)
            SELECT k.tenant_id, k.knowledge_base_id 
            FROM knowledges k
            WHERE k.deleted_at IS NULL
            AND (k.tag_id = '' OR k.tag_id IS NULL)
        ) AS untagged
    LOOP
        -- Check if "未分类" tag already exists for this knowledge base
        SELECT id INTO new_tag_id
        FROM knowledge_tags
        WHERE tenant_id = kb_record.tenant_id 
        AND knowledge_base_id = kb_record.knowledge_base_id 
        AND name = '未分类'
        LIMIT 1;

        -- If not exists, create the tag
        IF new_tag_id IS NULL THEN
            new_tag_id := gen_random_uuid()::VARCHAR(36);
            INSERT INTO knowledge_tags (id, tenant_id, knowledge_base_id, name, color, sort_order, created_at, updated_at)
            VALUES (new_tag_id, kb_record.tenant_id, kb_record.knowledge_base_id, '未分类', '', 0, NOW(), NOW());
            RAISE NOTICE '[Migration 000008] Created "未分类" tag (id: %) for tenant_id: %, kb_id: %', 
                new_tag_id, kb_record.tenant_id, kb_record.knowledge_base_id;
        ELSE
            RAISE NOTICE '[Migration 000008] "未分类" tag already exists (id: %) for tenant_id: %, kb_id: %', 
                new_tag_id, kb_record.tenant_id, kb_record.knowledge_base_id;
        END IF;

        -- Update chunks with empty tag_id (both faq and document types)
        UPDATE chunks 
        SET tag_id = new_tag_id, updated_at = NOW()
        WHERE tenant_id = kb_record.tenant_id 
        AND knowledge_base_id = kb_record.knowledge_base_id 
        AND chunk_type IN ('faq', 'document')
        AND (tag_id = '' OR tag_id IS NULL);
        
        GET DIAGNOSTICS updated_chunks = ROW_COUNT;
        RAISE NOTICE '[Migration 000008] Updated % chunks for tenant_id: %, kb_id: %', 
            updated_chunks, kb_record.tenant_id, kb_record.knowledge_base_id;

        -- Update knowledges with empty tag_id (documents)
        UPDATE knowledges 
        SET tag_id = new_tag_id, updated_at = NOW()
        WHERE tenant_id = kb_record.tenant_id 
        AND knowledge_base_id = kb_record.knowledge_base_id 
        AND deleted_at IS NULL
        AND (tag_id = '' OR tag_id IS NULL);
        
        GET DIAGNOSTICS updated_knowledges = ROW_COUNT;
        RAISE NOTICE '[Migration 000008] Updated % knowledges for tenant_id: %, kb_id: %', 
            updated_knowledges, kb_record.tenant_id, kb_record.knowledge_base_id;

        -- Update embeddings with empty tag_id (if embeddings table exists and has tag_id column)
        IF EXISTS (
            SELECT 1 FROM information_schema.columns 
            WHERE table_name = 'embeddings' AND column_name = 'tag_id'
        ) THEN
            UPDATE embeddings 
            SET tag_id = new_tag_id
            WHERE knowledge_base_id = kb_record.knowledge_base_id 
            AND (tag_id = '' OR tag_id IS NULL)
            AND chunk_id IN (
                SELECT id FROM chunks 
                WHERE tenant_id = kb_record.tenant_id 
                AND knowledge_base_id = kb_record.knowledge_base_id 
                AND chunk_type IN ('faq', 'document')
            );
            
            GET DIAGNOSTICS updated_embeddings = ROW_COUNT;
            RAISE NOTICE '[Migration 000008] Updated % embeddings for kb_id: %', 
                updated_embeddings, kb_record.knowledge_base_id;
        END IF;
    END LOOP;

    RAISE NOTICE '[Migration 000008] Completed migration of untagged entries';
END $$;

-- ---- source: 000009_add_last_faq_import_result.up.sql ----
-- Add last_faq_import_result column to knowledge table
-- This field stores the latest FAQ import result for FAQ type knowledge

ALTER TABLE knowledges ADD COLUMN IF NOT EXISTS last_faq_import_result JSON DEFAULT NULL;

-- ---- source: 000010_add_seq_id.up.sql ----
-- Migration 000010: Add seq_id (auto-increment integer ID) to chunks and knowledge_tags tables
-- This provides integer IDs for FAQ entries and tags for external API usage

-- ============================================================================
-- Section 1: Add seq_id to chunks table
-- ============================================================================

DO $$ BEGIN RAISE NOTICE '[Migration 000010] Adding seq_id column to chunks table'; END $$;

-- Create sequence for chunks with starting value > 72528124
CREATE SEQUENCE IF NOT EXISTS chunks_seq_id_seq START WITH 100000000;

-- Add seq_id column to chunks table
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS seq_id BIGINT;

-- Set default value using sequence
ALTER TABLE chunks ALTER COLUMN seq_id SET DEFAULT nextval('chunks_seq_id_seq');

-- Populate existing rows with sequence values
UPDATE chunks SET seq_id = nextval('chunks_seq_id_seq') WHERE seq_id IS NULL;

-- Make seq_id NOT NULL after populating
ALTER TABLE chunks ALTER COLUMN seq_id SET NOT NULL;

-- Create unique index on seq_id
CREATE UNIQUE INDEX IF NOT EXISTS idx_chunks_seq_id ON chunks(seq_id);

-- ============================================================================
-- Section 2: Add seq_id to knowledge_tags table
-- ============================================================================

DO $$ BEGIN RAISE NOTICE '[Migration 000010] Adding seq_id column to knowledge_tags table'; END $$;

-- Create sequence for knowledge_tags with starting value > 2924026
CREATE SEQUENCE IF NOT EXISTS knowledge_tags_seq_id_seq START WITH 10000000;

-- Add seq_id column to knowledge_tags table
ALTER TABLE knowledge_tags ADD COLUMN IF NOT EXISTS seq_id BIGINT;

-- Set default value using sequence
ALTER TABLE knowledge_tags ALTER COLUMN seq_id SET DEFAULT nextval('knowledge_tags_seq_id_seq');

-- Populate existing rows with sequence values
UPDATE knowledge_tags SET seq_id = nextval('knowledge_tags_seq_id_seq') WHERE seq_id IS NULL;

-- Make seq_id NOT NULL after populating
ALTER TABLE knowledge_tags ALTER COLUMN seq_id SET NOT NULL;

-- Create unique index on seq_id
CREATE UNIQUE INDEX IF NOT EXISTS idx_knowledge_tags_seq_id ON knowledge_tags(seq_id);

DO $$ BEGIN RAISE NOTICE '[Migration 000010] seq_id columns added successfully!'; END $$;

-- ---- source: 000011_pg_search_update.up.sql ----
-- Migration 000011: Update pg_search extension to latest version
-- Equivalent to: psql -c 'ALTER EXTENSION pg_search UPDATE;'

DO $$
BEGIN
    IF current_setting('app.skip_embedding', true) = 'true' THEN
        RAISE NOTICE 'Skipping pg_search update (app.skip_embedding=true)';
        RETURN;
    END IF;

    ALTER EXTENSION pg_search UPDATE;
END $$;

-- ---- source: 000013_engine_configs.up.sql ----
-- Description: Add parser_engine_config and storage_engine_config to tenants for UI-configured overrides.
DO $$ BEGIN RAISE NOTICE '[Migration 000013] Adding engine config columns to tenants'; END $$;

ALTER TABLE tenants ADD COLUMN IF NOT EXISTS parser_engine_config JSONB DEFAULT NULL;
COMMENT ON COLUMN tenants.parser_engine_config IS 'Parser engine overrides (mineru_endpoint, mineru_api_key, etc.); takes precedence over env when parsing';

ALTER TABLE tenants ADD COLUMN IF NOT EXISTS storage_engine_config JSONB DEFAULT NULL;
COMMENT ON COLUMN tenants.storage_engine_config IS 'Storage engine parameters for Local, MinIO, COS; used for document/file storage and docreader';

-- ---- source: 000014_storage_provider_config.up.sql ----
-- Description: Add storage_provider_config column to knowledge_bases, migrating from legacy cos_config.
-- This separates the storage provider selection (KB-level) from storage credentials (tenant-level).
DO $$ BEGIN RAISE NOTICE '[Migration 000014] Adding storage_provider_config to knowledge_bases'; END $$;

-- Step 1: Add new column
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS storage_provider_config JSONB DEFAULT NULL;
COMMENT ON COLUMN knowledge_bases.storage_provider_config IS 'Storage provider config for this KB. Only stores provider name; credentials come from tenant StorageEngineConfig.';

-- Step 2: Migrate existing provider from legacy cos_config
UPDATE knowledge_bases
SET storage_provider_config = jsonb_build_object('provider', cos_config->>'provider')
WHERE cos_config IS NOT NULL
  AND cos_config->>'provider' IS NOT NULL
  AND cos_config->>'provider' != ''
  AND (storage_provider_config IS NULL
       OR storage_provider_config->>'provider' IS NULL
       OR storage_provider_config->>'provider' = '');

-- Step 3: For KBs that have documents but no provider set, mark them with the
-- sentinel value so the application can fill in the actual STORAGE_TYPE on startup.
-- We use 'pending_migration' as a marker; the app replaces it with the real env value.
UPDATE knowledge_bases kb
SET storage_provider_config = '{"provider": "__pending_env__"}'
WHERE (kb.storage_provider_config IS NULL
       OR kb.storage_provider_config->>'provider' IS NULL
       OR kb.storage_provider_config->>'provider' = '')
  AND EXISTS (
    SELECT 1 FROM knowledges k
    WHERE k.knowledge_base_id = kb.id
      AND k.deleted_at IS NULL
  );

-- ---- source: 000015_add_is_fallback.up.sql ----
-- Add is_fallback column to messages table
-- Tracks whether a response was generated using fallback logic (no knowledge base match found)
ALTER TABLE messages ADD COLUMN IF NOT EXISTS is_fallback BOOLEAN DEFAULT FALSE;

-- ---- source: 000016_add_kb_pinned.up.sql ----
-- Add pin (置顶) support for knowledge bases
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS is_pinned BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS pinned_at TIMESTAMP WITH TIME ZONE NULL;

-- ---- source: 000017_mcp_builtin.up.sql ----
-- ============================================================================
-- Migration 000017: Add is_builtin support for MCP services
-- ============================================================================

DO $$ BEGIN RAISE NOTICE '[Migration 000017] Adding is_builtin column to mcp_services...'; END $$;

-- Add is_builtin column to mcp_services
ALTER TABLE mcp_services ADD COLUMN IF NOT EXISTS is_builtin BOOLEAN NOT NULL DEFAULT false;
CREATE INDEX IF NOT EXISTS idx_mcp_services_is_builtin ON mcp_services(is_builtin);

DO $$ BEGIN RAISE NOTICE '[Migration 000017] is_builtin column added to mcp_services'; END $$;

-- ---- source: 000018_extend_tenant_api_key.up.sql ----
-- Migration 000018: Extend tenant api_key column to support encrypted values
ALTER TABLE tenants ALTER COLUMN api_key TYPE varchar(256);

-- ---- source: 000019_add_agent_duration_ms.up.sql ----
-- Add agent_duration_ms column to messages table
-- Stores the total agent execution duration in milliseconds (from query start to answer start)
ALTER TABLE messages ADD COLUMN IF NOT EXISTS agent_duration_ms BIGINT DEFAULT 0;

-- ---- source: 000020_add_message_knowledge_id.up.sql ----
-- Add knowledge_id column to messages table for linking messages to chat history knowledge base entries
ALTER TABLE messages ADD COLUMN IF NOT EXISTS knowledge_id VARCHAR(36);
CREATE INDEX IF NOT EXISTS idx_messages_knowledge_id ON messages(knowledge_id);

-- Add chat_history_config JSONB column to tenants table
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS chat_history_config JSONB;

-- Add retrieval_config JSONB column to tenants table
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS retrieval_config JSONB;

-- ---- source: 000021_im_channel.up.sql ----
-- Migration: 000021_im_channel_sessions
-- Description: Create IM channel session mapping and IM channel configuration tables
DO $$ BEGIN RAISE NOTICE '[Migration 000021] Creating IM channel integration tables'; END $$;

CREATE TABLE IF NOT EXISTS im_channel_sessions (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    platform VARCHAR(20) NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    chat_id VARCHAR(128) NOT NULL DEFAULT '',
    session_id VARCHAR(36) NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    tenant_id BIGINT NOT NULL,
    agent_id VARCHAR(36) DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Partial unique index: only enforce uniqueness for non-deleted rows
CREATE UNIQUE INDEX IF NOT EXISTS idx_channel_lookup
    ON im_channel_sessions (platform, user_id, chat_id, tenant_id)
    WHERE deleted_at IS NULL;

-- Index for tenant-based queries
CREATE INDEX IF NOT EXISTS idx_im_channel_tenant ON im_channel_sessions (tenant_id);

-- Index for session-based queries
CREATE INDEX IF NOT EXISTS idx_im_channel_session ON im_channel_sessions (session_id);

-- Partial index for soft deletes (only index deleted rows)
CREATE INDEX IF NOT EXISTS idx_im_channel_deleted ON im_channel_sessions (deleted_at) WHERE deleted_at IS NOT NULL;

COMMENT ON TABLE im_channel_sessions IS 'Maps IM platform channels to Gaze conversation sessions';
COMMENT ON COLUMN im_channel_sessions.platform IS 'IM platform identifier: wecom, feishu, etc.';
COMMENT ON COLUMN im_channel_sessions.user_id IS 'Platform-specific user identifier';
COMMENT ON COLUMN im_channel_sessions.chat_id IS 'Platform-specific chat/group identifier, empty for direct messages';
COMMENT ON COLUMN im_channel_sessions.session_id IS 'Associated Gaze session ID';
COMMENT ON COLUMN im_channel_sessions.tenant_id IS 'Tenant that owns this channel mapping';
COMMENT ON COLUMN im_channel_sessions.agent_id IS 'Custom agent ID used for this channel, empty for default';
COMMENT ON COLUMN im_channel_sessions.status IS 'Channel status: active, paused, expired';
COMMENT ON COLUMN im_channel_sessions.metadata IS 'Platform-specific extra data (JSON)';

DO $$ BEGIN RAISE NOTICE '[Migration 000021] Creating table: im_channels'; END $$;

CREATE TABLE IF NOT EXISTS im_channels (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id BIGINT NOT NULL,
    agent_id VARCHAR(36) NOT NULL,
    platform VARCHAR(20) NOT NULL,
    name VARCHAR(255) NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT true,
    mode VARCHAR(20) NOT NULL DEFAULT 'websocket',
    output_mode VARCHAR(20) NOT NULL DEFAULT 'stream',
    credentials JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_im_channels_tenant ON im_channels (tenant_id);
CREATE INDEX IF NOT EXISTS idx_im_channels_agent ON im_channels (agent_id);
CREATE INDEX IF NOT EXISTS idx_im_channels_deleted ON im_channels (deleted_at) WHERE deleted_at IS NOT NULL;

COMMENT ON TABLE im_channels IS 'IM platform channel configurations bound to agents';
COMMENT ON COLUMN im_channels.agent_id IS 'Agent ID this channel is bound to';
COMMENT ON COLUMN im_channels.platform IS 'IM platform: wecom, feishu';
COMMENT ON COLUMN im_channels.name IS 'User-defined channel name for identification';
COMMENT ON COLUMN im_channels.mode IS 'Connection mode: webhook or websocket';
COMMENT ON COLUMN im_channels.output_mode IS 'Output mode: stream (real-time) or full (wait for complete answer)';
COMMENT ON COLUMN im_channels.credentials IS 'Platform credentials (JSONB): WeCom webhook={corp_id,agent_secret,token,encoding_aes_key,corp_agent_id}, WeCom ws={bot_id,bot_secret}, Feishu={app_id,app_secret,verification_token,encrypt_key}';

-- Add im_channel_id column to im_channel_sessions for linking
ALTER TABLE im_channel_sessions ADD COLUMN IF NOT EXISTS im_channel_id VARCHAR(36) DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_im_channel_sessions_channel ON im_channel_sessions (im_channel_id) WHERE im_channel_id != '';

DO $$ BEGIN RAISE NOTICE '[Migration 000021] IM channel integration setup completed successfully!'; END $$;

-- ---- source: 000022_message_images.up.sql ----
ALTER TABLE messages ADD COLUMN IF NOT EXISTS images JSONB DEFAULT '[]';

-- ---- source: 000023_im_channel_kb_id.up.sql ----
-- Add knowledge_base_id column to im_channels table.
-- When set, file messages received on this channel will be saved to the specified knowledge base.
ALTER TABLE im_channels ADD COLUMN IF NOT EXISTS knowledge_base_id VARCHAR(36) DEFAULT '';

-- ---- source: 000024_im_channel_bot_identity.up.sql ----
-- Migration: 000024_im_channel_bot_identity
-- Description: Add bot_identity column to im_channels for duplicate bot prevention.
-- bot_identity is a computed unique key derived from credentials, ensuring each bot
-- can only be connected to one active (non-deleted) channel.
DO $$ BEGIN RAISE NOTICE '[Migration 000024] Adding bot_identity column to im_channels'; END $$;

ALTER TABLE im_channels ADD COLUMN IF NOT EXISTS bot_identity VARCHAR(255) NOT NULL DEFAULT '';

-- Partial unique index: only enforce uniqueness for non-deleted rows with a non-empty bot_identity.
-- Empty bot_identity (unknown credential format) is excluded from the constraint.
CREATE UNIQUE INDEX IF NOT EXISTS idx_im_channels_bot_identity
    ON im_channels (bot_identity)
    WHERE deleted_at IS NULL AND bot_identity != '';

COMMENT ON COLUMN im_channels.bot_identity IS 'Unique bot identity derived from credentials (e.g. wecom:ws:{bot_id}, feishu:{app_id}). Used to prevent duplicate bot bindings.';

DO $$ BEGIN RAISE NOTICE '[Migration 000024] bot_identity column and unique index created successfully'; END $$;

-- ---- source: 000025_message_channel.up.sql ----
-- Migration: 000025_message_channel
-- Description: Add channel column to messages and knowledge to track the source channel (web, api, im, etc.)
DO $$ BEGIN RAISE NOTICE '[Migration 000025] Adding channel column to messages and knowledge'; END $$;

ALTER TABLE messages ADD COLUMN IF NOT EXISTS channel VARCHAR(50) NOT NULL DEFAULT '';
COMMENT ON COLUMN messages.channel IS 'Source channel of the message: web, api, im, etc.';

ALTER TABLE knowledges ADD COLUMN IF NOT EXISTS channel VARCHAR(50) NOT NULL DEFAULT 'web';
COMMENT ON COLUMN knowledges.channel IS 'Source channel of the knowledge: web, api, browser_extension, wechat, etc.';

DO $$ BEGIN RAISE NOTICE '[Migration 000025] channel column added to messages and knowledge successfully'; END $$;

-- ---- source: 000026_chunks_query_indexes.up.sql ----
-- ============================================================================
-- Migration 000026: Add missing indexes for chunks query performance
-- ============================================================================
-- Covers grep_chunks tool queries that filter by (knowledge_base_id, tenant_id)
-- and aggregate by (knowledge_id) with is_enabled/deleted_at filters.
-- ============================================================================

DO $$ BEGIN RAISE NOTICE '[Migration 000026] Adding chunks query indexes'; END $$;

-- Index for main query: WHERE knowledge_base_id = ? AND tenant_id = ?
-- Also benefits JOIN and OR conditions across multiple KB-tenant pairs
CREATE INDEX IF NOT EXISTS idx_chunks_kb_tenant
    ON chunks(knowledge_base_id, tenant_id);

-- Index for count query: WHERE knowledge_id IN (...) AND is_enabled = true AND deleted_at IS NULL
-- GROUP BY knowledge_id
CREATE INDEX IF NOT EXISTS idx_chunks_knowledge_enabled
    ON chunks(knowledge_id, is_enabled, deleted_at);

-- ---- source: 000027_message_rendered_content.up.sql ----
-- Migration: 000027_message_rendered_content
-- Description: Add rendered_content column to messages to persist RAG-augmented user content for multi-turn context
DO $$ BEGIN RAISE NOTICE '[Migration 000027] Adding rendered_content column to messages'; END $$;

ALTER TABLE messages ADD COLUMN IF NOT EXISTS rendered_content TEXT NOT NULL DEFAULT '';
COMMENT ON COLUMN messages.rendered_content IS 'Full RAG-augmented user message sent to LLM, preserving retrieval context across turns';

DO $$ BEGIN RAISE NOTICE '[Migration 000027] rendered_content column added to messages successfully'; END $$;

-- ---- source: 000028_im_thread_session.up.sql ----
-- Migration: 000028_im_thread_session
-- Description: Add thread-based session mode support for IM channels
DO $$ BEGIN RAISE NOTICE '[Migration 000028] Adding thread session support'; END $$;

-- 1. im_channels: add session_mode column
ALTER TABLE im_channels
    ADD COLUMN IF NOT EXISTS session_mode VARCHAR(20) NOT NULL DEFAULT 'user';

-- 2. CHECK constraint (separate statement: ADD COLUMN IF NOT EXISTS does not support inline CONSTRAINT)
DO $$ BEGIN
    ALTER TABLE im_channels
        ADD CONSTRAINT chk_im_channels_session_mode
        CHECK (session_mode IN ('user', 'thread'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

COMMENT ON COLUMN im_channels.session_mode IS
    'Session resolution mode: user (per user+chat, default) or thread (per thread)';

-- 3. im_channel_sessions: add thread_id column
ALTER TABLE im_channel_sessions
    ADD COLUMN IF NOT EXISTS thread_id VARCHAR(128) NOT NULL DEFAULT '';

COMMENT ON COLUMN im_channel_sessions.thread_id IS
    'Platform thread identifier for thread-based sessions. Empty for user-mode sessions.';

-- 4. Unique index for thread-mode session lookup
--    Includes chat_id because Telegram message_thread_id is per-chat (topic ID within a group).
CREATE UNIQUE INDEX IF NOT EXISTS idx_channel_thread_lookup
    ON im_channel_sessions (platform, chat_id, thread_id, tenant_id)
    WHERE deleted_at IS NULL AND thread_id != '';

DO $$ BEGIN RAISE NOTICE '[Migration 000028] Thread session support added'; END $$;

-- ---- source: 000029_datasource_tables.up.sql ----
-- Migration: 000029_datasource_tables
-- Description: Create data_sources and sync_logs tables for external data source management
DO $$ BEGIN RAISE NOTICE '[Migration 000029] Creating data_sources and sync_logs tables'; END $$;

-- Create data_sources table for managing external data source configurations
CREATE TABLE IF NOT EXISTS data_sources (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    config JSONB,
    sync_schedule VARCHAR(100),
    sync_mode VARCHAR(20) DEFAULT 'incremental',
    status VARCHAR(32) DEFAULT 'active',
    conflict_strategy VARCHAR(32) DEFAULT 'overwrite',
    sync_deletions BOOLEAN DEFAULT true,
    last_sync_at TIMESTAMP NULL,
    last_sync_cursor JSONB,
    last_sync_result JSONB,
    error_message TEXT,
    sync_log_retention_days INT DEFAULT 30,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

CREATE INDEX IF NOT EXISTS idx_data_sources_tenant_id ON data_sources (tenant_id);
CREATE INDEX IF NOT EXISTS idx_data_sources_knowledge_base_id ON data_sources (knowledge_base_id);
CREATE INDEX IF NOT EXISTS idx_data_sources_type ON data_sources (type);
CREATE INDEX IF NOT EXISTS idx_data_sources_status ON data_sources (status);
CREATE INDEX IF NOT EXISTS idx_data_sources_deleted_at ON data_sources (deleted_at);

-- Create sync_logs table for tracking sync operation history
CREATE TABLE IF NOT EXISTS sync_logs (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    data_source_id VARCHAR(36) NOT NULL REFERENCES data_sources(id) ON DELETE CASCADE,
    tenant_id BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    finished_at TIMESTAMP NULL,
    items_total INT DEFAULT 0,
    items_created INT DEFAULT 0,
    items_updated INT DEFAULT 0,
    items_deleted INT DEFAULT 0,
    items_skipped INT DEFAULT 0,
    items_failed INT DEFAULT 0,
    error_message TEXT,
    result JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sync_logs_data_source_id ON sync_logs (data_source_id);
CREATE INDEX IF NOT EXISTS idx_sync_logs_tenant_id ON sync_logs (tenant_id);
CREATE INDEX IF NOT EXISTS idx_sync_logs_status ON sync_logs (status);
CREATE INDEX IF NOT EXISTS idx_sync_logs_started_at ON sync_logs (started_at);


DO $$ BEGIN RAISE NOTICE '[Migration 000029] data_sources and sync_logs tables created successfully'; END $$;

-- ---- source: 000030_web_search_providers.up.sql ----
-- Migration: 000030_web_search_providers
-- Description: Create web_search_providers table for tenant-specific search engine configurations
DO $$ BEGIN RAISE NOTICE '[Migration 000030] Creating web_search_providers table'; END $$;

-- Create web_search_providers table for managing tenant search engine configurations
-- Each row represents a configured search provider instance (e.g., "Production Bing", "Test Google")
-- Agents reference these by ID via custom_agents.config.web_search_provider_id
CREATE TABLE IF NOT EXISTS web_search_providers (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    name VARCHAR(255) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    description TEXT,
    parameters JSONB,
    is_default BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

CREATE INDEX IF NOT EXISTS idx_web_search_providers_tenant_id ON web_search_providers (tenant_id);
CREATE INDEX IF NOT EXISTS idx_web_search_providers_provider ON web_search_providers (provider);
CREATE INDEX IF NOT EXISTS idx_web_search_providers_deleted_at ON web_search_providers (deleted_at);

DO $$ BEGIN RAISE NOTICE '[Migration 000030] web_search_providers table created successfully'; END $$;

-- ---- source: 000031_add_asr_config.up.sql ----
-- Migration: 000031_add_asr_config
-- Description: Add asr_config column to knowledge_bases for ASR (Automatic Speech Recognition) configuration.
-- ASR config stores the model ID and language settings for audio transcription.
DO $$ BEGIN RAISE NOTICE '[Migration 000031] Adding asr_config column to knowledge_bases'; END $$;

ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS asr_config JSONB;

COMMENT ON COLUMN knowledge_bases.asr_config IS 'ASR (Automatic Speech Recognition) configuration: {"enabled": bool, "model_id": string, "language": string}';

DO $$ BEGIN RAISE NOTICE '[Migration 000031] asr_config column added successfully'; END $$;

-- ---- source: 000032_vector_stores.up.sql ----
-- Migration: 000032_vector_stores
-- Description: Create vector_stores table for tenant-specific vector database configurations
DO $$ BEGIN RAISE NOTICE '[Migration 000032] Creating vector_stores table'; END $$;

CREATE TABLE IF NOT EXISTS vector_stores (
    id                VARCHAR(36)  NOT NULL PRIMARY KEY,
    name              VARCHAR(255) NOT NULL,
    engine_type       VARCHAR(50)  NOT NULL,
    connection_config JSONB        NOT NULL DEFAULT '{}',
    index_config      JSONB        NOT NULL DEFAULT '{}',
    tenant_id         BIGINT       NOT NULL,
    created_at        TIMESTAMP    DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMP    DEFAULT CURRENT_TIMESTAMP,
    deleted_at        TIMESTAMP    NULL
);

-- Name uniqueness per tenant (excluding soft-deleted rows)
CREATE UNIQUE INDEX IF NOT EXISTS idx_vector_stores_name_tenant
    ON vector_stores(name, tenant_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_vector_stores_tenant_id ON vector_stores(tenant_id);
CREATE INDEX IF NOT EXISTS idx_vector_stores_engine_type ON vector_stores(engine_type);
CREATE INDEX IF NOT EXISTS idx_vector_stores_deleted_at ON vector_stores(deleted_at);

-- Note: updated_at is managed by GORM hooks, no database trigger needed.

DO $$ BEGIN RAISE NOTICE '[Migration 000032] vector_stores table created successfully'; END $$;

-- ---- source: 000033_add_video_info_to_chunks.up.sql ----
-- Migration: 000033_add_video_info_to_chunks
-- Description: Add video_info column to chunks table for video multimodal processing.
-- This column stores JSON-serialized VideoInfo struct containing video analysis results.
DO $$ BEGIN RAISE NOTICE '[Migration 000033] Adding video_info column to chunks'; END $$;

ALTER TABLE chunks ADD COLUMN IF NOT EXISTS video_info TEXT;

COMMENT ON COLUMN chunks.video_info IS 'Video information in JSON format: {"url": string, "frame_count": int, "has_vlm_analysis": bool, "has_asr": bool, "video_summary": string, "asr_text": string, "frame_descriptions": string[]}';

DO $$ BEGIN RAISE NOTICE '[Migration 000033] video_info column added successfully'; END $$;

-- ---- source: 000034_add_attachments.up.sql ----
ALTER TABLE messages ADD COLUMN IF NOT EXISTS attachments JSONB DEFAULT '[]'::jsonb;

-- ---- source: 000035_add_credentials.up.sql ----
-- Description: Add credentials column to tenants for third-party provider credentials (e.g. GazeCloud AppID/AppSecret).
DO $$ BEGIN RAISE NOTICE '[Migration 000035] Adding credentials column to tenants'; END $$;

ALTER TABLE tenants ADD COLUMN IF NOT EXISTS credentials JSONB DEFAULT NULL;
COMMENT ON COLUMN tenants.credentials IS 'Third-party provider credentials (e.g. GazeCloud AppID/AppSecret); encrypted at application level';

-- ---- source: 000036_kb_vector_store_id.up.sql ----
-- Migration: 000036_kb_vector_store_id
-- Description: Add vector_store_id column to knowledge_bases for per-KB VectorStore binding.
-- NULL means "use tenant's effective engines" (env store derived from RETRIEVE_DRIVER),
-- which preserves existing behavior for all pre-existing rows.
-- No foreign key: env store IDs are virtual (not present in vector_stores), and
-- VectorStore soft-delete must not cascade into KB references. Referential integrity is
-- enforced at the application layer.
-- On PostgreSQL 11+ this ALTER runs as metadata-only (O(1), no row rewrite) because the
-- column is nullable with no DEFAULT. PostgreSQL 10 and earlier are not supported.
DO $$ BEGIN RAISE NOTICE '[Migration 000036] Adding vector_store_id column to knowledge_bases'; END $$;

ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS vector_store_id VARCHAR(36);

COMMENT ON COLUMN knowledge_bases.vector_store_id IS
    'References vector_stores.id. NULL means tenant default (env store derived from RETRIEVE_DRIVER). '
    'Immutable after creation (enforced at ORM and service layer). No FK by design.';

-- Composite index: tenant_id first to align with the dominant query pattern. All KB queries
-- start with `WHERE tenant_id = $1`, so the composite index enforces tenant-scoped plans and
-- hardens against accidental cross-tenant access. A full (non-partial) index is used because
-- the target end-state has most KBs bound to a store, making a partial index short-lived.
-- Storage cost is negligible at typical KB row counts.
CREATE INDEX IF NOT EXISTS idx_knowledge_bases_tenant_vector_store
    ON knowledge_bases(tenant_id, vector_store_id);

-- Note: CREATE INDEX here holds a SHARE lock briefly. For very large knowledge_bases tables
-- (hundreds of thousands of rows or more), operators may prefer CREATE INDEX CONCURRENTLY.
-- This requires running the statement outside a transaction, which the standard migration
-- runner does not support; execute the CONCURRENTLY variant from a separate operational
-- script in that case. Simply replacing the statement below with CONCURRENTLY inside this
-- file will cause the migration to fail and leave schema_migrations in a dirty state.

DO $$
DECLARE non_null_count BIGINT;
BEGIN
    SELECT COUNT(*) INTO non_null_count
    FROM knowledge_bases WHERE vector_store_id IS NOT NULL;
    RAISE NOTICE '[Migration 000036] knowledge_bases.vector_store_id added (% non-NULL rows)', non_null_count;
END $$;

-- ---- source: 000037_gaze_and_indexing.up.sql ----
-- Migration: 000037_gaze_and_indexing
-- Description: Gaze feature schema (gaze_pages table + gaze_config column + gaze_page_issues table)
-- and the indexing_strategy column on knowledge_bases.
DO $$ BEGIN RAISE NOTICE '[Migration 000037] Applying gaze + indexing schema'; END $$;

-- ---------------------------------------------------------------------------
-- 1) gaze_pages table and gaze_config column
-- Gaze pages are LLM-generated, interlinked markdown documents that form a
-- persistent gaze for a knowledge base.
-- ---------------------------------------------------------------------------
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS gaze_config JSONB;

COMMENT ON COLUMN knowledge_bases.gaze_config IS 'Gaze configuration: {"auto_ingest": bool, "synthesis_model_id": string, "gaze_language": string, "max_pages_per_ingest": int}';

CREATE TABLE IF NOT EXISTS gaze_pages (
    id              VARCHAR(36) PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    slug            VARCHAR(255) NOT NULL,
    title           VARCHAR(512) NOT NULL DEFAULT '',
    page_type       VARCHAR(32) NOT NULL DEFAULT 'summary',
    status          VARCHAR(32) NOT NULL DEFAULT 'published',
    content         TEXT NOT NULL DEFAULT '',
    summary         TEXT NOT NULL DEFAULT '',
    parent_slug     VARCHAR(255) NOT NULL DEFAULT '',
    -- folder_id is the single source of truth for a page's placement in the
    -- directory tree (FK to gaze_folders.id; '' = gaze root). category_path /
    -- gaze_path / depth below are denormalized projections of the folder chain,
    -- kept in sync on write so list/index/search queries need no folder join.
    folder_id       VARCHAR(36) NOT NULL DEFAULT '',
    category_path   JSONB DEFAULT '[]'::JSONB,
    gaze_path       VARCHAR(1024) NOT NULL DEFAULT '',
    depth           INT NOT NULL DEFAULT 0,
    sort_order      INT NOT NULL DEFAULT 0,
    source_refs     JSONB DEFAULT '[]'::JSONB,
    chunk_refs      JSONB DEFAULT '[]'::JSONB,
    in_links        JSONB DEFAULT '[]'::JSONB,
    out_links       JSONB DEFAULT '[]'::JSONB,
    page_metadata   JSONB DEFAULT '{}'::JSONB,
    aliases         JSONB DEFAULT '[]'::JSONB,
    version         INT NOT NULL DEFAULT 1,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMP WITH TIME ZONE
);

-- slug must be unique within a knowledge base (for non-deleted pages)
CREATE UNIQUE INDEX IF NOT EXISTS idx_gaze_pages_kb_slug
    ON gaze_pages (knowledge_base_id, slug)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_gaze_pages_kb_id
    ON gaze_pages (knowledge_base_id);

CREATE INDEX IF NOT EXISTS idx_gaze_pages_page_type
    ON gaze_pages (knowledge_base_id, page_type);

CREATE INDEX IF NOT EXISTS idx_gaze_pages_parent_slug
    ON gaze_pages (knowledge_base_id, parent_slug);

CREATE INDEX IF NOT EXISTS idx_gaze_pages_tree
    ON gaze_pages (knowledge_base_id, page_type, gaze_path, sort_order, title);

CREATE INDEX IF NOT EXISTS idx_gaze_pages_folder
    ON gaze_pages (knowledge_base_id, folder_id);

CREATE INDEX IF NOT EXISTS idx_gaze_pages_tenant_id
    ON gaze_pages (tenant_id);

CREATE INDEX IF NOT EXISTS idx_gaze_pages_deleted_at
    ON gaze_pages (deleted_at);

CREATE INDEX IF NOT EXISTS idx_gaze_pages_fulltext
    ON gaze_pages USING GIN (to_tsvector('simple', coalesce(title, '') || ' ' || coalesce(content, '')));

-- ---------------------------------------------------------------------------
-- 1b) gaze_folders table
-- First-class directory nodes for the gaze browser. A folder exists
-- independently of any page, so empty folders persist (the user can build a
-- skeleton and file pages into it later). parent_id forms an adjacency-list
-- tree ('' = root); path is the materialized "/"-joined name chain kept for
-- cheap display/sort. gaze_pages.folder_id references id; renaming/moving a
-- folder updates this row's subtree and the affected pages' cached path.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS gaze_folders (
    id                VARCHAR(36) PRIMARY KEY,
    tenant_id         BIGINT NOT NULL DEFAULT 0,
    knowledge_base_id VARCHAR(36) NOT NULL,
    parent_id         VARCHAR(36) NOT NULL DEFAULT '',
    name              VARCHAR(255) NOT NULL,
    path              VARCHAR(1024) NOT NULL DEFAULT '',
    depth             INT NOT NULL DEFAULT 0,
    sort_order        INT NOT NULL DEFAULT 0,
    created_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMP WITH TIME ZONE
);

-- A folder name is unique among its live siblings under the same parent.
CREATE UNIQUE INDEX IF NOT EXISTS idx_gaze_folders_parent_name
    ON gaze_folders (knowledge_base_id, parent_id, name)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_gaze_folders_parent
    ON gaze_folders (knowledge_base_id, parent_id);

CREATE INDEX IF NOT EXISTS idx_gaze_folders_deleted_at
    ON gaze_folders (deleted_at);

-- ---------------------------------------------------------------------------
-- 2) gaze_page_issues table
-- Reports issues against generated gaze pages (LLM-flagged or user-flagged).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS gaze_page_issues (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    issue_type VARCHAR(50) NOT NULL,
    description TEXT NOT NULL,
    suspected_knowledge_ids JSONB,
    status VARCHAR(20) DEFAULT 'pending' NOT NULL,
    reported_by VARCHAR(100) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_gaze_page_issues_tenant_id ON gaze_page_issues(tenant_id);
CREATE INDEX IF NOT EXISTS idx_gaze_page_issues_knowledge_base_id ON gaze_page_issues(knowledge_base_id);
CREATE INDEX IF NOT EXISTS idx_gaze_page_issues_slug ON gaze_page_issues(slug);
CREATE INDEX IF NOT EXISTS idx_gaze_page_issues_status ON gaze_page_issues(status);

-- ---------------------------------------------------------------------------
-- 3) indexing_strategy column on knowledge_bases
-- Controls which indexing pipelines are active (vector, keyword, gaze, graph).
-- Backfill: existing rows get vector+keyword=true (legacy default behavior);
-- gaze_enabled / graph_enabled stay false until explicitly enabled.
-- ---------------------------------------------------------------------------
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS indexing_strategy JSONB;

COMMENT ON COLUMN knowledge_bases.indexing_strategy IS 'Indexing pipelines strategy: {"vector_enabled": bool, "keyword_enabled": bool, "gaze_enabled": bool, "graph_enabled": bool}';

UPDATE knowledge_bases
SET indexing_strategy = jsonb_build_object(
    'vector_enabled',  TRUE,
    'keyword_enabled', TRUE,
    'gaze_enabled',    FALSE,
    'graph_enabled',   FALSE
)
WHERE indexing_strategy IS NULL;

DO $$ BEGIN RAISE NOTICE '[Migration 000037] gaze + indexing schema applied successfully'; END $$;

-- ---- source: 000038_im_session_agent_id.up.sql ----
-- Migration: 000038_im_session_agent_id
-- Description: Add agent_id to IM channel session unique indexes to isolate sessions per agent.
-- Fixes: GitHub #1066 (cross-agent session contamination)
DO $$ BEGIN RAISE NOTICE '[Migration 000038] Adding agent_id to IM channel session indexes'; END $$;

-- 1. User-mode lookup: include agent_id so the same user talking to
--    different agents gets separate sessions.
DROP INDEX IF EXISTS idx_channel_lookup;
CREATE UNIQUE INDEX idx_channel_lookup
    ON im_channel_sessions (platform, user_id, chat_id, tenant_id, agent_id)
    WHERE deleted_at IS NULL;

-- 2. Thread-mode lookup: same fix for thread-based sessions.
DROP INDEX IF EXISTS idx_channel_thread_lookup;
CREATE UNIQUE INDEX idx_channel_thread_lookup
    ON im_channel_sessions (platform, chat_id, thread_id, tenant_id, agent_id)
    WHERE deleted_at IS NULL AND thread_id != '';

DO $$ BEGIN RAISE NOTICE '[Migration 000038] IM session agent_id indexes updated'; END $$;

-- ---- source: 000039_session_user_id_and_pin.up.sql ----
-- Migration: 000039_session_user_id_and_pin
-- Description: Add user_id, is_pinned, pinned_at to sessions for per-user
--              session ownership and user-level pinning. Existing rows keep
--              user_id = NULL and stay visible at the tenant level for
--              backward compatibility.

DO $$ BEGIN RAISE NOTICE '[Migration 000039] Adding user_id/is_pinned/pinned_at to sessions'; END $$;

ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS user_id VARCHAR(36),
    ADD COLUMN IF NOT EXISTS is_pinned BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS pinned_at TIMESTAMP WITH TIME ZONE;

-- Index for the common list query:
--   WHERE tenant_id = ? AND (user_id = ? OR user_id IS NULL) AND deleted_at IS NULL
--   ORDER BY is_pinned DESC, pinned_at DESC NULLS LAST, updated_at DESC
CREATE INDEX IF NOT EXISTS idx_sessions_tenant_user_pin
    ON sessions (tenant_id, user_id, is_pinned DESC, pinned_at DESC, updated_at DESC)
    WHERE deleted_at IS NULL;

DO $$ BEGIN RAISE NOTICE '[Migration 000039] sessions user_id/pin columns added'; END $$;

-- ---- source: 000040_gaze_log_entries.up.sql ----
-- Migration: 000040_gaze_log_entries
-- Description: Dedicated append-only event table for gaze operation log entries.
--              Replaces the "single giant TEXT column on slug='log' gaze_pages
--              row" model that caused O(n^2) write amplification as KBs grew:
--              every ingest/retract op previously did GetLog + UpdatePage
--              rewriting the entire (potentially multi-MB) TEXT column.
--              Here each event is one INSERT; reads paginate by (kb_id, id DESC).

DO $$ BEGIN RAISE NOTICE '[Migration 000040] Applying gaze_log_entries schema'; END $$;

CREATE TABLE IF NOT EXISTS gaze_log_entries (
    id                BIGSERIAL PRIMARY KEY,
    tenant_id         BIGINT NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    action            VARCHAR(32) NOT NULL,
    knowledge_id      VARCHAR(36) NOT NULL DEFAULT '',
    doc_title         TEXT NOT NULL DEFAULT '',
    summary           TEXT NOT NULL DEFAULT '',
    pages_affected    JSONB NOT NULL DEFAULT '[]'::JSONB,
    created_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Primary list query: cursor-paginated feed per KB, newest first.
-- Using id (BIGSERIAL, monotonic) as the cursor sidesteps duplicate-timestamp
-- tie-breaking that a created_at cursor would require.
CREATE INDEX IF NOT EXISTS idx_gaze_log_entries_kb_id_desc
    ON gaze_log_entries (knowledge_base_id, id DESC);

CREATE INDEX IF NOT EXISTS idx_gaze_log_entries_tenant_id
    ON gaze_log_entries (tenant_id);

DO $$ BEGIN RAISE NOTICE '[Migration 000040] gaze_log_entries schema applied successfully'; END $$;

-- ---- source: 000041_task_queue_and_gaze_indexes.up.sql ----
-- Migration: 000041_task_queue_and_gaze_indexes
-- Description: Generic task pending queue + dead-letter table, and three
--              additional GIN indexes on gaze_pages to back the optimized
--              ingest pipeline (4w-document-scale).
--
-- Schemas introduced:
--   1. task_pending_ops    — durable replacement for the Redis-list-backed
--                            gaze:pending:<kbID> queue. Designed generically
--                            (task_type + scope + scope_id) so future
--                            "debounced batch" task types can reuse it
--                            without another migration.
--   2. task_dead_letters   — generic archived-task record. Written by both
--                            (a) the asynq middleware when a task exhausts
--                            its retry budget, and (b) the gaze ingest
--                            service when a per-document op exceeds its
--                            in-batch retry quota.
--   3. gaze_pages indexes  — GIN on source_refs (containment + text), and
--                            trigram on lower(title) for the new pg_trgm
--                            dedup pre-filter.

DO $$ BEGIN RAISE NOTICE '[Migration 000041] Applying task queue + gaze indexes schema'; END $$;

-- Ensure pg_trgm is loaded before creating the trigram GIN index below.
-- Migration 000002 also creates this extension, but only inside the
-- conditional embeddings block (app.skip_embedding gate). Environments that
-- skip 000002's body — or had pg_trgm install silently fail there — would
-- otherwise blow up on the gin_trgm_ops index further down. Re-issuing
-- CREATE EXTENSION IF NOT EXISTS here is a no-op when it's already present
-- and surfaces a clear error early when the extension genuinely isn't
-- available, instead of failing midway and leaving task_pending_ops /
-- task_dead_letters uncreated (see issue #1319).
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ---------------------------------------------------------------------------
-- 1) task_pending_ops
--
-- Generic replacement for ad-hoc Redis-list pending queues. Each row is one
-- pending operation scoped to a (task_type, scope, scope_id) tuple. The
-- consumer (e.g. gaze ingest batch handler) uses PeekBatch to pull the head
-- of the list, processes the ops, then DeleteByIDs the consumed rows.
--
-- claimed_at backs the concurrent-claim workflow (ClaimBatch): standard-mode
-- gaze ingest dropped the exclusive per-KB lock in favour of multiple batches
-- claiming DISJOINT dedup_keys via SELECT ... FOR UPDATE SKIP LOCKED, stamping
-- claimed_at = NOW() on the rows they take. A claim older than the consumer's
-- stale threshold (a crashed/abandoned worker) is recoverable by the next
-- claimer; a fresh claim blocks its whole dedup_key so same-document ops never
-- split across concurrent batches. Lite mode (no Redis) still serializes per
-- KB in-process and leaves claimed_at NULL.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS task_pending_ops (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   BIGINT NOT NULL,
    task_type   VARCHAR(64) NOT NULL,
    scope       VARCHAR(32) NOT NULL,
    scope_id    VARCHAR(64) NOT NULL,
    op          VARCHAR(32) NOT NULL,
    dedup_key   VARCHAR(128) NOT NULL DEFAULT '',
    payload     JSONB NOT NULL DEFAULT '{}'::JSONB,
    fail_count  INT NOT NULL DEFAULT 0,
    enqueued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    claimed_at  TIMESTAMPTZ
);

COMMENT ON TABLE task_pending_ops IS 'Generic durable pending-op queue keyed by (task_type, scope, scope_id). Replaces ad-hoc Redis-list queues that were vulnerable to TTL eviction.';
COMMENT ON COLUMN task_pending_ops.task_type IS 'Free-form task identifier, e.g. "gaze:ingest" — should match an asynq task type when applicable.';
COMMENT ON COLUMN task_pending_ops.scope IS 'Logical scope, e.g. "knowledge_base" / "knowledge" / "tenant". Read together with scope_id.';
COMMENT ON COLUMN task_pending_ops.dedup_key IS 'Optional service-defined key used by the consumer to de-duplicate equivalent ops within a single batch peek. Empty means no de-dup.';
COMMENT ON COLUMN task_pending_ops.fail_count IS 'In-batch retry counter: the consumer increments it via IncrFailCount and dead-letters once it exceeds a service-defined cap.';
COMMENT ON COLUMN task_pending_ops.claimed_at IS 'Concurrent-claim marker: set to NOW() when a consumer claims the row (SELECT ... FOR UPDATE SKIP LOCKED). NULL = unclaimed; a value older than the consumer stale threshold is a crashed/abandoned claim and is recoverable. A fresh claim blocks its whole dedup_key so same-document ops never split across concurrent batches.';

-- Cover the PeekBatch query: the consumer scans rows for one
-- (task_type, scope, scope_id) tuple ordered by id ASC.
CREATE INDEX IF NOT EXISTS idx_task_pending_ops_scope
    ON task_pending_ops (task_type, scope, scope_id, id);

CREATE INDEX IF NOT EXISTS idx_task_pending_ops_tenant
    ON task_pending_ops (tenant_id);

-- ---------------------------------------------------------------------------
-- 2) task_dead_letters
--
-- Permanent archive for tasks that exhausted retries. Written from two
-- distinct paths:
--   (a) The asynq dead-letter middleware (internal/middleware/asynq_dead_letter.go)
--       inserts a row whenever a task's retry count equals MaxRetry on the
--       way out — covers every asynq task type uniformly.
--   (b) The gaze ingest service inserts a row directly when a per-document
--       op exceeds gazeMaxFailRetries inside a batch — these never escalate
--       to an asynq retry because they're handled inline.
--
-- Operations queries it by scope (e.g. all dead letters for a KB) or by
-- task_type (e.g. all summary:generation failures in the last 24h). No
-- TTL — rows are kept for postmortem until manually pruned. failed_at DESC
-- so newest-first scans are cheap.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS task_dead_letters (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   BIGINT NOT NULL,
    task_type   VARCHAR(64) NOT NULL,
    scope       VARCHAR(32) NOT NULL,
    scope_id    VARCHAR(64) NOT NULL,
    related_id  VARCHAR(64) NOT NULL DEFAULT '',
    payload     JSONB NOT NULL,
    last_error  TEXT NOT NULL DEFAULT '',
    fail_count  INT NOT NULL,
    failed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE task_dead_letters IS 'Permanent archive of tasks that exhausted retries. Written by the asynq dead-letter middleware and by service-level retry handlers (e.g. gaze ingest per-doc retries).';
COMMENT ON COLUMN task_dead_letters.related_id IS 'Optional secondary identifier. Gaze ingest puts knowledge_id here so retract/ingest dead letters cluster around the source document.';
COMMENT ON COLUMN task_dead_letters.payload IS 'Raw task payload (asynq.Task.Payload) at the time of failure. Allows manual requeue via SQL + asynq.Client.Enqueue.';
COMMENT ON COLUMN task_dead_letters.last_error IS 'String form of the error that caused the final retry to fail. Long stack traces are kept verbatim.';

CREATE INDEX IF NOT EXISTS idx_task_dead_letters_scope
    ON task_dead_letters (scope, scope_id, failed_at DESC);

CREATE INDEX IF NOT EXISTS idx_task_dead_letters_tenant
    ON task_dead_letters (tenant_id, failed_at DESC);

CREATE INDEX IF NOT EXISTS idx_task_dead_letters_task_type
    ON task_dead_letters (task_type, failed_at DESC);

-- ---------------------------------------------------------------------------
-- 3) gaze_pages: source_refs / title trigram indexes
--
-- (a) source_refs containment GIN — covers `source_refs @> ?::jsonb`. Used
--     by ListBySourceRef when looking up pages that cite a given knowledge
--     id (delete flow, retract reconciliation, getExistingPageSlugsForKnowledge).
--
-- (b) source_refs text-fulltext GIN — fallback for the legacy "kid|title"
--     ref form, which the runtime LIKE pattern needs. Same shape as the
--     existing fulltext index in 000037 (line 54-55).
--
-- (c) title trigram GIN — backs the new pg_trgm-based dedup pre-filter
--     (selectDedupCandidatePages). pg_trgm extension is already loaded by
--     migration 000002.
-- ---------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_gaze_pages_source_refs
    ON gaze_pages USING GIN (source_refs jsonb_path_ops);

CREATE INDEX IF NOT EXISTS idx_gaze_pages_source_refs_text
    ON gaze_pages USING GIN (to_tsvector('simple', source_refs::text));

CREATE INDEX IF NOT EXISTS idx_gaze_pages_title_trgm
    ON gaze_pages USING GIN (lower(title) gin_trgm_ops);

DO $$ BEGIN RAISE NOTICE '[Migration 000041] task queue + gaze indexes schema applied successfully'; END $$;

-- ---- source: 000042_mcp_tool_approval.up.sql ----
-- MCP tool human-approval flags (per service + tool name, issue #1173)
DO $$ BEGIN RAISE NOTICE '[Migration 000042] Creating mcp_tool_approvals...'; END $$;

CREATE TABLE IF NOT EXISTS mcp_tool_approvals (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    service_id VARCHAR(36) NOT NULL REFERENCES mcp_services(id) ON DELETE CASCADE,
    tool_name VARCHAR(512) NOT NULL,
    require_approval BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_mcp_tool_approvals_tenant_svc_tool
    ON mcp_tool_approvals(tenant_id, service_id, tool_name);

CREATE INDEX IF NOT EXISTS idx_mcp_tool_approvals_service_id ON mcp_tool_approvals(service_id);

DO $$ BEGIN RAISE NOTICE '[Migration 000042] mcp_tool_approvals ready'; END $$;

-- ---- source: 000043_tenant_rbac.up.sql ----
-- Personal edition keeps only the per-resource creator columns and the
-- agent visibility switch from the enterprise RBAC migration. Membership and
-- role tables do not exist: the deployment has exactly one owner account.
DO $$ BEGIN RAISE NOTICE '[Migration 000043] Starting creator/tracking setup...'; END $$;

-- 3. knowledge_bases.creator_id
DO $$ BEGIN RAISE NOTICE '[Migration 000043] Adding creator_id to knowledge_bases'; END $$;
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS creator_id VARCHAR(36);
CREATE INDEX IF NOT EXISTS idx_knowledge_bases_tenant_creator
    ON knowledge_bases(tenant_id, creator_id);

-- 4. custom_agents.runnable_by_viewer
DO $$ BEGIN RAISE NOTICE '[Migration 000043] Adding runnable_by_viewer to custom_agents'; END $$;
ALTER TABLE custom_agents
    ADD COLUMN IF NOT EXISTS runnable_by_viewer BOOLEAN NOT NULL DEFAULT TRUE;

DO $$ BEGIN RAISE NOTICE '[Migration 000043] tenant RBAC setup ready'; END $$;

-- ---- source: 000049_user_preferences.up.sql ----
-- Migration: 000049_user_preferences
-- Adds a user-scoped JSON preferences blob so per-user UI toggles
-- (memory feature, future preferences) can be persisted server-side and
-- sync across devices/browsers. Previously the only carrier for such
-- toggles was the browser's localStorage, which intentionally does NOT
-- cross devices — switching machines silently reset every preference.
--
-- A single jsonb column keeps the schema flat: new keys can be added by
-- writing through the existing PUT /auth/me/preferences endpoint, no DDL
-- per knob. Server reads/writes are partial updates (key-merge), so an
-- old client that only knows about a subset of keys can't accidentally
-- wipe newer ones it doesn't understand.
--
-- Defaults to '{}' (NOT NULL) so existing rows behave as "no preference
-- set yet" without forcing every read path to handle NULL.

DO $$ BEGIN RAISE NOTICE '[Migration 000049] Adding users.preferences column...'; END $$;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS preferences JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN users.preferences IS 'Per-user JSON preferences (memory toggle, future UI knobs)';

DO $$ BEGIN RAISE NOTICE '[Migration 000049] Done.'; END $$;

-- ---- source: 000050_user_kb_pins.up.sql ----
-- Migration: 000050_user_kb_pins
-- Per-(user, tenant, kb) pin state for the knowledge-base list.
--
-- Previously KB pinning lived on the `knowledge_bases.is_pinned` /
-- `pinned_at` columns and was therefore tenant-wide: one admin's pin
-- reordered the list for every member of the tenant, and the RBAC guard
-- on PUT /knowledge-bases/:id/pin (OwnedKBOrAdmin) hid the affordance
-- entirely from Viewer / Contributor users who could see the KB but not
-- edit it. Both behaviours were confusing and inconsistent with the
-- session-pin model (which has always been per-user).
--
-- This migration introduces a dedicated mapping table and backfills
-- existing tenant-wide pins onto the KB's CreatorID, so the admin who
-- pinned a KB still sees it pinned after upgrade while other tenant
-- members get a clean per-user starting point. The legacy columns on
-- knowledge_bases are left in place for one release to keep rollback
-- safe; new code stops writing them and reads pin state from this
-- table instead.
DO $$ BEGIN RAISE NOTICE '[Migration 000050] Creating table: user_kb_pins'; END $$;

CREATE TABLE IF NOT EXISTS user_kb_pins (
    tenant_id  BIGINT      NOT NULL,
    user_id    VARCHAR(36) NOT NULL,
    kb_id      VARCHAR(36) NOT NULL,
    pinned_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, user_id, kb_id)
);

-- Primary read path: "give me this user's pinned KB ids in this tenant,
-- newest pin first". The PK already covers equality on (tenant_id,
-- user_id, kb_id); the secondary index lets us avoid an in-memory sort
-- when stamping the list response.
CREATE INDEX IF NOT EXISTS idx_user_kb_pins_user_tenant_pinned_at
    ON user_kb_pins (tenant_id, user_id, pinned_at DESC);

-- Backfill: every KB that is currently pinned at the tenant level gets
-- carried over as a personal pin for its creator. KBs whose creator_id
-- is empty / NULL (legacy rows that predate the RBAC migration) are
-- intentionally skipped — we have no user to attribute the pin to, and
-- silently dropping these is preferable to attributing them to a tenant
-- owner who may not actually want the KB pinned in their view.
INSERT INTO user_kb_pins (tenant_id, user_id, kb_id, pinned_at)
SELECT kb.tenant_id,
       kb.creator_id,
       kb.id,
       COALESCE(kb.pinned_at, CURRENT_TIMESTAMP)
  FROM knowledge_bases kb
 WHERE kb.is_pinned = TRUE
   AND kb.creator_id IS NOT NULL
   AND kb.creator_id <> ''
ON CONFLICT (tenant_id, user_id, kb_id) DO NOTHING;

DO $$ BEGIN RAISE NOTICE '[Migration 000050] user_kb_pins table ready'; END $$;

-- ---- source: 000052_models_managed_by.up.sql ----
-- Migration: 000052_models_managed_by
-- Add a `managed_by` column to `models` so the YAML built-in models loader can
-- own a slice of the table without disturbing rows created via the UI/API or
-- seeded by hand-written SQL.
--
-- Background:
--   * 000051-era PR #1453 introduced config/builtin_models.yaml as a
--     declarative source of truth for built-in models. The first version
--     could add/update rows but had no way to remove them: deleting an entry
--     from YAML left the corresponding row in `models`, breaking the
--     "declarative" contract and forcing operators back into ad-hoc SQL.
--   * Indiscriminately deleting `is_builtin=true` rows on each startup is
--     unsafe because some deployments seed built-ins via direct SQL, and
--     those rows must not be touched.
--
-- Solution:
--   * Add `managed_by` (varchar(32), default ''). YAML loader writes "yaml";
--     anything else (UI / API / SQL seed) keeps the empty default.
--   * On every startup the loader UPSERTs YAML entries with managed_by='yaml'
--     and soft-deletes rows where (is_builtin=true AND managed_by='yaml' AND
--     id NOT IN <current YAML id set>). Manual rows are never inspected.
--
-- This migration is idempotent (IF NOT EXISTS) and safe to re-run.

DO $$ BEGIN RAISE NOTICE '[Migration 000052] Adding models.managed_by column'; END $$;

ALTER TABLE models
    ADD COLUMN IF NOT EXISTS managed_by VARCHAR(32) NOT NULL DEFAULT '';

-- A partial index on the YAML-managed slice keeps the startup sweep cheap
-- even if the table grows large. Manual rows (managed_by='') are excluded
-- so the index stays small.
CREATE INDEX IF NOT EXISTS idx_models_managed_by_yaml
    ON models (managed_by)
    WHERE managed_by <> '';

DO $$ BEGIN RAISE NOTICE '[Migration 000052] Done'; END $$;

-- ---- source: 000053_system_admin_and_settings.up.sql ----
-- Migration: 000053_system_admin_and_settings
-- Two related changes for the system-admin / platform-settings feature, merged
-- into a single migration since they ship together and have no intermediate
-- consumer:
--
--   1. users.is_system_admin
--      System-level administrator flag, independent of tenant-scoped roles.
--      System admins have platform-wide privileges (e.g. editing
--      system_settings below). Indexed for efficient privilege checks and
--      admin listing.
--
--   2. system_settings table
--      System-scoped (NOT tenant-scoped) settings for platform-wide runtime
--      tunables, gated by SystemAdmin.
--
--      Deliberately do not seed values here. For migrated deployments, a DB
--      row has higher precedence than ENV, so inserting built-in defaults
--      would silently override existing operator configuration such as
--      DISABLE_REGISTRATION, SSRF_WHITELIST, and MAX_FILE_SIZE_MB. The
--      service exposes registry-backed virtual rows to the management UI
--      until an admin explicitly saves a value.
--
--      Why JSONB for `value`?
--        We want to support int / string / bool / arrays / objects under one
--        schema without a separate table per type. The `value_type` column
--        tells the service layer how to parse the raw JSON. Booleans
--        roundtrip as `true`/`false`, ints as `42`, strings as `"foo"`.
--
--      Indexes:
--        - UNIQUE on (key) — primary lookup pattern, every Get hits this
--        - (category) — for the management UI's grouped list view

DO $$ BEGIN RAISE NOTICE '[Migration 000053] Adding users.is_system_admin column...'; END $$;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS is_system_admin BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_users_is_system_admin ON users (is_system_admin);

COMMENT ON COLUMN users.is_system_admin IS 'Whether the user is a system administrator (independent of tenant roles)';

DO $$ BEGIN RAISE NOTICE '[Migration 000053] Creating table: system_settings'; END $$;

CREATE TABLE IF NOT EXISTS system_settings (
    id               BIGSERIAL PRIMARY KEY,
    key              VARCHAR(128) NOT NULL UNIQUE,
    value            JSONB NOT NULL,
    value_type       VARCHAR(16)  NOT NULL,
    category         VARCHAR(32)  NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    is_secret        BOOLEAN NOT NULL DEFAULT false,
    requires_restart BOOLEAN NOT NULL DEFAULT false,
    last_modified_by VARCHAR(36) NOT NULL DEFAULT '',
    created_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_system_settings_category
    ON system_settings (category);

DO $$ BEGIN RAISE NOTICE '[Migration 000053] Done.'; END $$;

-- ---- source: 000055_knowledge_processing_spans.up.sql ----
-- Migration: 000054_knowledge_processing_spans
-- Per-(knowledge, attempt) span tree for the document parsing pipeline,
-- inspired by Langfuse's trace / span / generation hierarchy.
--
-- Background: knowledge.parse_status is a 4-state field (pending /
-- processing / completed / failed). When users hit "stuck in processing",
-- they have no way to tell which stage is actually running — DocReader,
-- chunking, embedding, multimodal OCR/VLM, or the final post-process
-- handoff. Operators face the same problem.
--
-- This table persists per-stage progress (and finer-grained subspans) so:
--   1. The frontend can render a five-segment timeline showing where
--      each document is in the pipeline, with collapsible per-image /
--      per-batch subspans underneath.
--   2. Failures carry a stable error_code (DOCREADER_TIMEOUT,
--      EMBEDDING_RATE_LIMIT, ...) the UI can map to localized
--      remediation text.
--   3. Reparse history is preserved across attempts — operators can
--      navigate ?attempt=N to post-mortem "why did it fail twice?".
--   4. Cascade-cancel rules use the parent_span_id tree to flip
--      dependent subspans/stages to "cancelled" when an upstream
--      span fails, so the UI shows a clear blast radius instead of
--      orphan spinners.
--
-- Schema mirrors Langfuse's vocabulary:
--   * one ROOT span per (knowledge_id, attempt) acting as the trace
--   * STAGE spans (docreader/chunking/embedding/multimodal/postprocess)
--     are children of root
--   * SUBSPANs (multimodal.image[i], embedding.batch[i], postprocess.spawn.X)
--     hang off their stage. The kind="generation" subset corresponds 1:1 to
--     a Langfuse generation; metadata.langfuse_trace_id stitches them.
DO $$ BEGIN RAISE NOTICE '[Migration 000054] Creating table: knowledge_processing_spans'; END $$;

CREATE TABLE IF NOT EXISTS knowledge_processing_spans (
    id              BIGSERIAL                PRIMARY KEY,
    knowledge_id    VARCHAR(64)              NOT NULL,
    attempt         INT                      NOT NULL DEFAULT 1,
    span_id         VARCHAR(64)              NOT NULL,
    parent_span_id  VARCHAR(64),
    name            VARCHAR(64)              NOT NULL,
    kind            VARCHAR(16)              NOT NULL,                       -- root / stage / subspan / generation
    status          VARCHAR(16)              NOT NULL,                       -- pending/running/done/failed/skipped/cancelled
    input           JSONB,
    output          JSONB,
    metadata        JSONB,
    error_code      VARCHAR(64),
    error_message   TEXT,
    error_detail    TEXT,
    started_at      TIMESTAMP WITH TIME ZONE,
    finished_at     TIMESTAMP WITH TIME ZONE,
    duration_ms     BIGINT,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_kpspan_attempt_span UNIQUE (knowledge_id, attempt, span_id)
);

-- Primary read path: fetch every span for a (knowledge, attempt) tuple in
-- one indexed range scan, then build the tree in memory. The unique
-- constraint above already covers point lookups.
CREATE INDEX IF NOT EXISTS idx_kpspan_knowledge_attempt
    ON knowledge_processing_spans (knowledge_id, attempt);

-- Operator query: "find spans stuck in running too long". Used by ad-hoc
-- diagnostics and the housekeeping sweep.
CREATE INDEX IF NOT EXISTS idx_kpspan_status_started
    ON knowledge_processing_spans (status, started_at);

-- Lineage walks: cascade-cancel a stage's downstream needs to find every
-- child by parent_span_id. The cardinality is small (≤ tens per attempt)
-- so we don't need a covering index, just B-tree on parent.
CREATE INDEX IF NOT EXISTS idx_kpspan_parent
    ON knowledge_processing_spans (parent_span_id)
    WHERE parent_span_id IS NOT NULL;

DO $$ BEGIN RAISE NOTICE '[Migration 000054] knowledge_processing_spans table ready'; END $$;

-- ---- source: 000056_knowledge_pending_subtasks.up.sql ----
-- Migration: 000056_knowledge_pending_subtasks
--
-- Add pending_subtasks_count to support the "finalizing" parse status,
-- which gates parse_status='completed' until enrichment subtasks
-- (summary, question generation, graph extract) finish.
--
-- Previously, parse_status flipped to 'completed' as soon as primary
-- chunks + embeddings were written, even though the user-cancellable
-- "expensive" tasks (graph extract = N LLM calls per chunk, question
-- gen, summary) were still in flight. That broke the user's intuition
-- that 'completed' means "no more resources will be spent on this".
--
-- New lifecycle:
--   pending -> processing -> finalizing -> completed
--                              ^
--                              | parse_status='finalizing' AND
--                              | pending_subtasks_count > 0 mean
--                              | enrichment is still running and
--                              | CancelKnowledgeParse can interrupt it.
--
-- Existing rows: column defaults to 0, so all historical 'completed'
-- rows look like "no pending subtasks" — which is correct (they're
-- past the enrichment phase by definition).
--
-- Gaze ingest is NOT counted here: it is debounced and KB-scoped, with
-- its own dedup queue; cancelling a single knowledge cannot meaningfully
-- shorten an in-flight gaze batch and the gaze worker already short-
-- circuits per-knowledge once parse_status is aborted.

ALTER TABLE knowledges
    ADD COLUMN IF NOT EXISTS pending_subtasks_count INT NOT NULL DEFAULT 0;

-- ---- source: 000057_models_display_name.up.sql ----
-- Migration: 000057_models_display_name
-- Add an optional user-facing display name for model rows. Runtime model
-- calls continue to use models.name; this field is presentation-only.

DO $$ BEGIN RAISE NOTICE '[Migration 000057] Adding models.display_name column'; END $$;

ALTER TABLE models
    ADD COLUMN IF NOT EXISTS display_name VARCHAR(255) NOT NULL DEFAULT '';

DO $$ BEGIN RAISE NOTICE '[Migration 000057] Done'; END $$;

-- ---- source: 000058_expand_knowledge_source.up.sql ----
ALTER TABLE knowledges
    ALTER COLUMN source TYPE VARCHAR(2048);

-- ---- source: 000059_embeddings_hnsw_1024.up.sql ----
-- Migration 000059: HNSW index for bge-m3 / 1024-dim embeddings
--
-- Upstream's 000002 only created HNSW partial indexes for dim=3584 (OpenAI
-- text-embedding-3-large) and dim=798. Both queries fall through to a
-- sequential scan on the 1024-dim halfvec column when the bound embedder
-- is bge-m3 — which is the default for any on-device deployment using
-- localhub-embedder.
--
-- Symptom: vector recall stays "correct but slow" (seq scan of all rows in
-- the embeddings table per query, ~100-500ms by KB size). Downstream agents
-- that probe latency-sensitive endpoints assumed degradation.
--
-- Fix: same partial-index pattern, dim=1024. Matches the exact expression
-- used in repository.go SearchEmbedding query (embedding::halfvec(1024) <=>
-- $1::halfvec(1024)) so the planner picks it up automatically.
--
-- Note: CONCURRENTLY is not used here because plpgsql DO blocks open an
-- implicit transaction and CREATE INDEX CONCURRENTLY can't run inside one.
-- For a fresh install this is a no-op (empty table); for production
-- backfill the operator should build the index manually using
-- CREATE INDEX CONCURRENTLY before applying this migration, then this
-- block becomes idempotent via IF NOT EXISTS.

-- Guard on the embeddings table's existence (not the vector extension):
-- deployments whose RETRIEVE_DRIVER excludes postgres run 000002 with
-- app.skip_embedding=true, so neither the table nor the vector extension is
-- created. Checking the table directly matches the established convention
-- (see 000007) and avoids a hard CREATE INDEX failure that would leave
-- schema_migrations dirty and block every later migration.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'embeddings') THEN
        IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'embeddings_embedding_idx_1024' OR indexname LIKE 'embeddings_embedding%1024%') THEN
            CREATE INDEX embeddings_embedding_idx_1024 ON embeddings
            USING hnsw ((embedding::halfvec(1024)) halfvec_cosine_ops)
            WITH (m = 16, ef_construction = 64)
            WHERE (dimension = 1024);
            RAISE NOTICE '[Migration 000059] Created HNSW index for dimension 1024 (bge-m3)';
        ELSE
            RAISE NOTICE '[Migration 000059] HNSW index for dimension 1024 already exists';
        END IF;
    ELSE
        RAISE NOTICE '[Migration 000059] embeddings table does not exist · skipping';
    END IF;
END $$;

-- ---- source: 000060_embed_channels.up.sql ----
-- Migration: 000060_embed_channels
-- Web embed channels for publishing agent chat to external sites.
DO $$ BEGIN RAISE NOTICE '[Migration 000060] Creating embed_channels table'; END $$;

CREATE TABLE IF NOT EXISTS embed_channels (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id BIGINT NOT NULL,
    agent_id VARCHAR(36) NOT NULL DEFAULT 'builtin-quick-answer',
    name VARCHAR(255) NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT true,
    publish_token VARCHAR(64) NOT NULL DEFAULT '',
    allowed_origins JSONB NOT NULL DEFAULT '[]',
    welcome_message TEXT NOT NULL DEFAULT '',
    rate_limit_per_minute INTEGER NOT NULL DEFAULT 30,
    rate_limit_per_day INTEGER NOT NULL DEFAULT 10000,
    primary_color VARCHAR(32) NOT NULL DEFAULT '',
    page_title VARCHAR(255) NOT NULL DEFAULT '',
    header_title_mode VARCHAR(32) NOT NULL DEFAULT 'channel',
    show_suggested_questions BOOLEAN NOT NULL DEFAULT true,
    widget_position VARCHAR(32) NOT NULL DEFAULT 'bottom-right',
    allow_web_search BOOLEAN NOT NULL DEFAULT false,
    allow_memory BOOLEAN NOT NULL DEFAULT false,
    allow_file_upload BOOLEAN NOT NULL DEFAULT false,
    default_locale VARCHAR(16) NOT NULL DEFAULT '',
    webhook_url VARCHAR(512) NOT NULL DEFAULT '',
    webhook_secret VARCHAR(128) NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_embed_channels_tenant ON embed_channels (tenant_id);
CREATE INDEX IF NOT EXISTS idx_embed_channels_agent ON embed_channels (agent_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_embed_channels_publish_token
    ON embed_channels (publish_token)
    WHERE publish_token <> '' AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_embed_channels_deleted ON embed_channels (deleted_at) WHERE deleted_at IS NOT NULL;

COMMENT ON TABLE embed_channels IS 'Web embed channels for publishing agent chat to external sites via iframe';
COMMENT ON COLUMN embed_channels.publish_token IS 'Plaintext scoped token (em_ prefix); rotatable from management UI';
COMMENT ON COLUMN embed_channels.allowed_origins IS 'JSON array of allowed HTTP(S) origins for embed API requests; empty rejects all (management UI requires at least one)';
COMMENT ON COLUMN embed_channels.rate_limit_per_minute IS 'Per-IP per-minute request cap for the public embed endpoints';
COMMENT ON COLUMN embed_channels.rate_limit_per_day IS
    'Channel-wide daily total request cap across all IPs; bounds cost/abuse since the publish token is publicly visible';
COMMENT ON COLUMN embed_channels.primary_color IS 'CSS color for embed widget accent (e.g. #0052d9)';
COMMENT ON COLUMN embed_channels.page_title IS 'Browser tab title for the embed page';
COMMENT ON COLUMN embed_channels.header_title_mode IS
    'Embed header title source: channel (fixed page title) or session (auto-generated after first message)';
COMMENT ON COLUMN embed_channels.show_suggested_questions IS
    'When true, embed chat shows suggested starter questions before the first visitor message';
COMMENT ON COLUMN embed_channels.widget_position IS 'Floating widget corner: bottom-right, bottom-left, top-right, top-left';
COMMENT ON COLUMN embed_channels.allow_web_search IS
    'When true, embed chat may show web search toggle; visitor must opt in per message';
COMMENT ON COLUMN embed_channels.allow_memory IS
    'When true, embed chat may use agent memory; client cannot override when false';
COMMENT ON COLUMN embed_channels.allow_file_upload IS
    'When true, embed chat may send images; client cannot override when false';
COMMENT ON COLUMN embed_channels.default_locale IS
    'Default visitor UI locale (zh-CN, en-US, ko-KR, ru-RU); empty follows browser';
COMMENT ON COLUMN embed_channels.webhook_url IS 'HTTPS endpoint for outbound message_sent / message_received events';
COMMENT ON COLUMN embed_channels.webhook_secret IS 'Optional HMAC-SHA256 secret for X-Gaze-Signature header';

DO $$ BEGIN RAISE NOTICE '[Migration 000060] embed_channels created'; END $$;

-- ---- source: 000061_gaze_page_hierarchy.up.sql ----
-- Migration: 000061_gaze_page_hierarchy
-- Description: Add structured directory hierarchy fields to gaze_pages.

DO $$ BEGIN RAISE NOTICE '[Migration 000061] Applying gaze page hierarchy schema'; END $$;

ALTER TABLE gaze_pages ADD COLUMN IF NOT EXISTS parent_slug VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE gaze_pages ADD COLUMN IF NOT EXISTS category_path JSONB DEFAULT '[]'::JSONB;
ALTER TABLE gaze_pages ADD COLUMN IF NOT EXISTS gaze_path VARCHAR(1024) NOT NULL DEFAULT '';
ALTER TABLE gaze_pages ADD COLUMN IF NOT EXISTS depth INT NOT NULL DEFAULT 0;
ALTER TABLE gaze_pages ADD COLUMN IF NOT EXISTS sort_order INT NOT NULL DEFAULT 0;

ALTER TABLE gaze_pages ADD COLUMN IF NOT EXISTS folder_id VARCHAR(36) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_gaze_pages_folder_id
    ON gaze_pages (folder_id);
-- ---------------------------------------------------------------------------
-- 1) gaze_folders table
-- First-class directory nodes for the gaze browser. A folder exists
-- independently of any page, so empty folders persist (the user can build a
-- skeleton and file pages into it later). parent_id forms an adjacency-list
-- tree ('' = root); path is the materialized "/"-joined name chain kept for
-- cheap display/sort. gaze_pages.folder_id references id; renaming/moving a
-- folder updates this row's subtree and the affected pages' cached path.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS gaze_folders (
    id                VARCHAR(36) PRIMARY KEY,
    tenant_id         BIGINT NOT NULL DEFAULT 0,
    knowledge_base_id VARCHAR(36) NOT NULL,
    parent_id         VARCHAR(36) NOT NULL DEFAULT '',
    name              VARCHAR(255) NOT NULL,
    path              VARCHAR(1024) NOT NULL DEFAULT '',
    depth             INT NOT NULL DEFAULT 0,
    sort_order        INT NOT NULL DEFAULT 0,
    created_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMP WITH TIME ZONE
);

-- A folder name is unique among its live siblings under the same parent.
CREATE UNIQUE INDEX IF NOT EXISTS idx_gaze_folders_parent_name
    ON gaze_folders (knowledge_base_id, parent_id, name)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_gaze_folders_parent
    ON gaze_folders (knowledge_base_id, parent_id);

CREATE INDEX IF NOT EXISTS idx_gaze_folders_deleted_at
    ON gaze_folders (deleted_at);

UPDATE gaze_pages
SET
    category_path = COALESCE(category_path, '[]'::JSONB),
    depth = COALESCE(depth, 0),
    gaze_path = CASE
        WHEN COALESCE(gaze_path, '') <> '' THEN gaze_path
        WHEN page_type IN ('index', 'log') THEN page_type || '/' || COALESCE(NULLIF(title, ''), slug)
        ELSE page_type || '/' || COALESCE(NULLIF(title, ''), slug)
    END
WHERE gaze_path = '' OR gaze_path IS NULL OR category_path IS NULL OR depth IS NULL;

CREATE INDEX IF NOT EXISTS idx_gaze_pages_parent_slug
    ON gaze_pages (knowledge_base_id, parent_slug);

CREATE INDEX IF NOT EXISTS idx_gaze_pages_tree
    ON gaze_pages (knowledge_base_id, page_type, gaze_path, sort_order, title);

DO $$ BEGIN RAISE NOTICE '[Migration 000061] gaze page hierarchy schema applied successfully'; END $$;

-- ---- source: 000062_mcp_oauth.up.sql ----
-- MCP OAuth2 support: per-service dynamically-registered clients and
-- per-user access/refresh tokens (issue: MCP OAuth2 authorization-code flow).
DO $$ BEGIN RAISE NOTICE '[Migration 000062] Creating mcp_oauth_clients / mcp_oauth_tokens...'; END $$;

CREATE TABLE IF NOT EXISTS mcp_oauth_clients (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    service_id VARCHAR(36) NOT NULL REFERENCES mcp_services(id) ON DELETE CASCADE,
    client_id VARCHAR(512) NOT NULL,
    client_secret TEXT,
    redirect_uri VARCHAR(1024),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_mcp_oauth_clients_tenant_svc
    ON mcp_oauth_clients(tenant_id, service_id);
CREATE INDEX IF NOT EXISTS idx_mcp_oauth_clients_service_id ON mcp_oauth_clients(service_id);

CREATE TABLE IF NOT EXISTS mcp_oauth_tokens (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    user_id VARCHAR(64) NOT NULL,
    service_id VARCHAR(36) NOT NULL REFERENCES mcp_services(id) ON DELETE CASCADE,
    access_token TEXT,
    refresh_token TEXT,
    token_type VARCHAR(32),
    expires_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_mcp_oauth_tokens_tenant_user_svc
    ON mcp_oauth_tokens(tenant_id, user_id, service_id);
CREATE INDEX IF NOT EXISTS idx_mcp_oauth_tokens_service_id ON mcp_oauth_tokens(service_id);
CREATE INDEX IF NOT EXISTS idx_mcp_oauth_tokens_user_id ON mcp_oauth_tokens(user_id);

DO $$ BEGIN RAISE NOTICE '[Migration 000062] mcp_oauth tables ready'; END $$;

-- ---- source: 000063_knowledge_multi_tags.up.sql ----
-- Migration: 000063_knowledge_multi_tags
-- Description: Add multi-tag support for document knowledge via a join table.
DO $$ BEGIN RAISE NOTICE '[Migration 000063] Creating knowledge_tag_relations and migrating data...'; END $$;

-- Create the many-to-many join table
CREATE TABLE IF NOT EXISTS knowledge_tag_relations (
    knowledge_id VARCHAR(36) NOT NULL,
    tag_id        VARCHAR(36) NOT NULL,
    created_at    TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (knowledge_id, tag_id)
);

-- Index: find all tag IDs for a given knowledge
CREATE INDEX IF NOT EXISTS idx_ktr_knowledge
    ON knowledge_tag_relations(knowledge_id);

-- Index: find all knowledge IDs for a given tag
CREATE INDEX IF NOT EXISTS idx_ktr_tag
    ON knowledge_tag_relations(tag_id);

-- Migrate existing single-tag data into the join table
INSERT INTO knowledge_tag_relations (knowledge_id, tag_id, created_at)
SELECT id, tag_id, updated_at
FROM knowledges
WHERE tag_id IS NOT NULL AND tag_id != ''
  AND deleted_at IS NULL;

-- Drop the old index and column
DROP INDEX IF EXISTS idx_knowledges_tag;
ALTER TABLE knowledges DROP COLUMN IF EXISTS tag_id;

DO $$ BEGIN RAISE NOTICE '[Migration 000063] knowledge_tag_relations ready'; END $$;

-- ---- source: 000064_principal_model.up.sql ----
-- Migration: 000064_principal_model
-- Description: Principal-based identity for MCP OAuth, tenant API config, and session ownership.

DO $$ BEGIN RAISE NOTICE '[Migration 000064] Adding principal columns to MCP OAuth tokens...'; END $$;

ALTER TABLE mcp_oauth_tokens
    ADD COLUMN IF NOT EXISTS principal_type VARCHAR(32),
    ADD COLUMN IF NOT EXISTS principal_id VARCHAR(512);

ALTER TABLE mcp_oauth_tokens
    ALTER COLUMN user_id TYPE VARCHAR(512);

UPDATE mcp_oauth_tokens
SET principal_type = 'web_user',
    principal_id = user_id
WHERE (principal_type IS NULL OR principal_type = '')
  AND user_id IS NOT NULL
  AND user_id <> '';

ALTER TABLE mcp_oauth_tokens
    ALTER COLUMN principal_type SET NOT NULL,
    ALTER COLUMN principal_id SET NOT NULL;

DROP INDEX IF EXISTS idx_mcp_oauth_tokens_tenant_user_svc;

CREATE UNIQUE INDEX IF NOT EXISTS idx_mcp_oauth_tokens_tenant_principal_svc
    ON mcp_oauth_tokens(tenant_id, principal_type, principal_id, service_id);

CREATE INDEX IF NOT EXISTS idx_mcp_oauth_tokens_principal
    ON mcp_oauth_tokens(principal_type, principal_id);

DO $$ BEGIN RAISE NOTICE '[Migration 000064] MCP OAuth principal columns ready'; END $$;

DO $$ BEGIN RAISE NOTICE '[Migration 000064] Adding tenant API principal config...'; END $$;

ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS api_principal_config JSONB;

DO $$ BEGIN RAISE NOTICE '[Migration 000064] tenant API principal config ready'; END $$;

DO $$ BEGIN RAISE NOTICE '[Migration 000064] Widening sessions.user_id to VARCHAR(512)'; END $$;

ALTER TABLE sessions
    ALTER COLUMN user_id TYPE VARCHAR(512);

DO $$ BEGIN RAISE NOTICE '[Migration 000064] principal model migration complete'; END $$;

-- ---- source: 000065_tenant_api_keys.up.sql ----
DO $$ BEGIN RAISE NOTICE '[Migration 000065] Creating tenant_api_keys...'; END $$;

CREATE TABLE IF NOT EXISTS tenant_api_keys (
    id BIGSERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    api_key TEXT NOT NULL DEFAULT '',
    full_access BOOLEAN NOT NULL DEFAULT FALSE,
    knowledge_base_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    -- Bounded per-key grants for non-full-access keys. KB allow-list still
    -- constrains which knowledge bases a scoped key may touch.
    capabilities JSONB NOT NULL DEFAULT '[]'::jsonb,
    last_used_at TIMESTAMP,
    expires_at TIMESTAMP,
    revoked_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_tenant_api_keys_tenant
    ON tenant_api_keys(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenant_api_keys_revoked_at
    ON tenant_api_keys(revoked_at);

INSERT INTO tenant_api_keys (
    tenant_id,
    name,
    key_hash,
    api_key,
    full_access,
    knowledge_base_ids,
    created_at,
    updated_at
)
SELECT
    id,
    'Tenant API key',
    'migrated-tenant-' || id::text,
    api_key,
    TRUE,
    '[]'::jsonb,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
FROM tenants
WHERE COALESCE(api_key, '') <> ''
ON CONFLICT (key_hash) DO NOTHING;

DROP INDEX IF EXISTS idx_tenants_api_key;
ALTER TABLE tenants DROP COLUMN IF EXISTS api_key;

DO $$ BEGIN RAISE NOTICE '[Migration 000065] tenant_api_keys ready'; END $$;

-- ---- source: 000066_expand_knowledge_span_name.up.sql ----
DO $$ BEGIN RAISE NOTICE '[Migration 000066] Expanding knowledge_processing_spans.name to VARCHAR(255)...'; END $$;

ALTER TABLE knowledge_processing_spans
    ALTER COLUMN name TYPE VARCHAR(255);

DO $$ BEGIN RAISE NOTICE '[Migration 000066] knowledge_processing_spans.name expanded'; END $$;

-- ---- source: 000067_question_suggestions.up.sql ----
-- Migration: 000067_question_suggestions
-- Description: Persist per-message execution context and attributable follow-up suggestions.

DO $$ BEGIN RAISE NOTICE '[Migration 000067] Adding message execution context...'; END $$;

ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS agent_id VARCHAR(36) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS agent_tenant_id INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS model_id VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS execution_context JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS idx_messages_agent_id ON messages(agent_id);

CREATE TABLE IF NOT EXISTS message_suggestion_sets (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    session_id VARCHAR(36) NOT NULL,
    assistant_message_id VARCHAR(36) NOT NULL,
    agent_id VARCHAR(36) NOT NULL DEFAULT '',
    agent_tenant_id INTEGER NOT NULL DEFAULT 0,
    placement VARCHAR(32) NOT NULL,
    config_hash VARCHAR(64) NOT NULL,
    locale VARCHAR(16) NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL,
    allow_regenerate BOOLEAN NOT NULL DEFAULT FALSE,
    suppression_reason VARCHAR(64) NOT NULL DEFAULT '',
    questions JSONB NOT NULL DEFAULT '[]'::jsonb,
    model_id VARCHAR(64) NOT NULL DEFAULT '',
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    latency_ms BIGINT NOT NULL DEFAULT 0,
    error_code VARCHAR(64) NOT NULL DEFAULT '',
    lease_until TIMESTAMP WITH TIME ZONE,
    generated_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_message_suggestion_sets_cache_key
    ON message_suggestion_sets (
        tenant_id,
        assistant_message_id,
        placement,
        config_hash,
        locale
    );
CREATE INDEX IF NOT EXISTS idx_message_suggestion_sets_session
    ON message_suggestion_sets(tenant_id, session_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_message_suggestion_sets_status
    ON message_suggestion_sets(status, lease_until);

CREATE TABLE IF NOT EXISTS message_suggestion_events (
    id BIGSERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    session_id VARCHAR(36) NOT NULL,
    suggestion_set_id VARCHAR(36) NOT NULL REFERENCES message_suggestion_sets(id) ON DELETE CASCADE,
    question_id VARCHAR(64) NOT NULL DEFAULT '',
    event_type VARCHAR(32) NOT NULL,
    actor_id VARCHAR(512) NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_message_suggestion_events_set
    ON message_suggestion_events(suggestion_set_id, created_at);
CREATE INDEX IF NOT EXISTS idx_message_suggestion_events_session
    ON message_suggestion_events(tenant_id, session_id, created_at);
CREATE INDEX IF NOT EXISTS idx_message_suggestion_events_type
    ON message_suggestion_events(event_type, created_at);

-- Promote the legacy starter-only field into the unified agent-owned policy.
-- Follow-up generation remains off for existing agents until an owner opts in.
UPDATE custom_agents
SET config = (config - 'suggested_prompts') || jsonb_build_object(
    'question_suggestions',
    COALESCE(
        config->'question_suggestions',
        jsonb_build_object(
            'starters', jsonb_build_object(
                'enabled', true,
                'mode', 'hybrid',
                'items', COALESCE(config->'suggested_prompts', '[]'::jsonb),
                'count', 6
            ),
            'follow_ups', jsonb_build_object(
                'enabled', false,
                'mode', 'hybrid',
                'count', 3,
                'categories', jsonb_build_array('clarify', 'deepen', 'action'),
                'max_context_turns', 2,
                'suppress_on_fallback', true,
                'suppress_when_answer_asks_question', true,
                'knowledge_fallback', true,
                'allow_regenerate', false
            )
        )
    )
)
WHERE config IS NOT NULL;

DO $$ BEGIN RAISE NOTICE '[Migration 000067] Question suggestions ready'; END $$;

-- ---- source: 000068_storage_backends.up.sql ----
CREATE TABLE IF NOT EXISTS storage_backends (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    name VARCHAR(255) NOT NULL,
    provider VARCHAR(32) NOT NULL,
    config JSONB NOT NULL DEFAULT '{}',
    source VARCHAR(16) NOT NULL DEFAULT 'user',
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    legacy_alias BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_storage_backends_name_tenant ON storage_backends(tenant_id, name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_storage_backends_legacy_alias ON storage_backends(tenant_id, provider) WHERE deleted_at IS NULL AND legacy_alias = TRUE;
CREATE INDEX IF NOT EXISTS idx_storage_backends_tenant ON storage_backends(tenant_id);
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS default_storage_backend_id VARCHAR(36);
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS storage_backend_id VARCHAR(36);
CREATE INDEX IF NOT EXISTS idx_knowledge_bases_storage_backend ON knowledge_bases(tenant_id, storage_backend_id);

-- ---- source: 000069_resource_registry.up.sql ----
CREATE TABLE IF NOT EXISTS resources (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    handle VARCHAR(22) NOT NULL UNIQUE,
    tenant_id BIGINT NOT NULL,
    storage_backend_id VARCHAR(36),
    provider VARCHAR(32) NOT NULL,
    physical_path TEXT NOT NULL,
    location_hash VARCHAR(64) NOT NULL,
    kind VARCHAR(32) NOT NULL DEFAULT 'file',
    mime_type VARCHAR(255) NOT NULL DEFAULT '',
    original_name VARCHAR(1024) NOT NULL DEFAULT '',
    size BIGINT NOT NULL DEFAULT 0,
    content_hash VARCHAR(64) NOT NULL DEFAULT '',
    lifecycle VARCHAR(16) NOT NULL DEFAULT 'persistent',
    expires_at TIMESTAMP NULL,
    state VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_resources_tenant_location
    ON resources(tenant_id, location_hash) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_resources_tenant ON resources(tenant_id);
CREATE INDEX IF NOT EXISTS idx_resources_backend ON resources(storage_backend_id);

CREATE TABLE IF NOT EXISTS resource_bindings (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    resource_id VARCHAR(36) NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    tenant_id BIGINT NOT NULL,
    owner_type VARCHAR(32) NOT NULL,
    owner_id VARCHAR(64) NOT NULL,
    relation VARCHAR(32) NOT NULL DEFAULT 'attachment',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_resource_bindings_unique
    ON resource_bindings(resource_id, owner_type, owner_id, relation);
CREATE INDEX IF NOT EXISTS idx_resource_bindings_owner
    ON resource_bindings(tenant_id, owner_type, owner_id);

-- ---- source: 000070_temporary_documents.up.sql ----
CREATE TABLE IF NOT EXISTS temporary_documents (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    resource_ref TEXT NOT NULL,
    file_name VARCHAR(1024) NOT NULL,
    file_type VARCHAR(32) NOT NULL,
    mime_type VARCHAR(255) NOT NULL DEFAULT '',
    file_size BIGINT NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'uploaded',
    content TEXT NOT NULL DEFAULT '',
    chunks JSONB NOT NULL DEFAULT '[]'::jsonb,
    image_refs JSONB NOT NULL DEFAULT '[]'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    processing_options JSONB NOT NULL DEFAULT '{}'::jsonb,
    token_count INTEGER NOT NULL DEFAULT 0,
    chunk_count INTEGER NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMP NOT NULL,
    started_at TIMESTAMP NULL,
    ready_at TIMESTAMP NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

CREATE INDEX IF NOT EXISTS idx_temporary_documents_scope ON temporary_documents(tenant_id, session_id);
CREATE INDEX IF NOT EXISTS idx_temporary_documents_status ON temporary_documents(status);
CREATE INDEX IF NOT EXISTS idx_temporary_documents_expires ON temporary_documents(expires_at);

-- ---- source: 000071_platform_api_keys.up.sql ----
DO $$ BEGIN RAISE NOTICE '[Migration 000071] Adding platform API key scope...'; END $$;

ALTER TABLE tenant_api_keys
    ADD COLUMN IF NOT EXISTS scope_type VARCHAR(16) NOT NULL DEFAULT 'tenant';

ALTER TABLE tenant_api_keys
    ALTER COLUMN tenant_id DROP NOT NULL;

ALTER TABLE tenant_api_keys
    DROP CONSTRAINT IF EXISTS chk_tenant_api_keys_scope;

ALTER TABLE tenant_api_keys
    ADD CONSTRAINT chk_tenant_api_keys_scope CHECK (
        (scope_type = 'tenant' AND tenant_id IS NOT NULL)
        OR (scope_type = 'platform' AND tenant_id IS NULL AND full_access = FALSE)
    );

CREATE INDEX IF NOT EXISTS idx_tenant_api_keys_scope_type
    ON tenant_api_keys(scope_type);

DO $$ BEGIN RAISE NOTICE '[Migration 000071] Platform API key scope ready'; END $$;

-- ---- source: 000072_auth_timestamp_tz.up.sql ----
DO $$ BEGIN RAISE NOTICE '[Migration 000072] Converting authorization timestamps to timestamptz...'; END $$;

-- Naive TIMESTAMP values were read through a GORM session with TimeZone=UTC.
-- Treat existing literals as UTC when promoting to timestamptz.
ALTER TABLE tenant_api_keys
    ALTER COLUMN last_used_at TYPE TIMESTAMP WITH TIME ZONE
        USING last_used_at AT TIME ZONE 'UTC';

ALTER TABLE tenant_api_keys
    ALTER COLUMN expires_at TYPE TIMESTAMP WITH TIME ZONE
        USING expires_at AT TIME ZONE 'UTC';

ALTER TABLE tenant_api_keys
    ALTER COLUMN revoked_at TYPE TIMESTAMP WITH TIME ZONE
        USING revoked_at AT TIME ZONE 'UTC';

ALTER TABLE tenant_api_keys
    ALTER COLUMN created_at TYPE TIMESTAMP WITH TIME ZONE
        USING created_at AT TIME ZONE 'UTC';

ALTER TABLE tenant_api_keys
    ALTER COLUMN updated_at TYPE TIMESTAMP WITH TIME ZONE
        USING updated_at AT TIME ZONE 'UTC';

DO $$ BEGIN RAISE NOTICE '[Migration 000072] Authorization timestamps ready'; END $$;

-- ---- source: 000074_mcp_oauth_refresh_lease.up.sql ----
DO $$ BEGIN RAISE NOTICE '[Migration 000074] Adding MCP OAuth refresh lease...'; END $$;

ALTER TABLE mcp_oauth_tokens
    ADD COLUMN IF NOT EXISTS refresh_lease_id VARCHAR(36),
    ADD COLUMN IF NOT EXISTS refresh_lease_until TIMESTAMP WITH TIME ZONE;

DO $$ BEGIN RAISE NOTICE '[Migration 000074] MCP OAuth refresh lease ready'; END $$;

-- ---- source: 000075_gaze_page_revisions.up.sql ----
-- Migration: 000075_gaze_page_revisions
-- Description: Gaze page revision history (content snapshots for diff/rollback)
-- plus edit provenance columns on gaze_pages.
DO $$ BEGIN RAISE NOTICE '[Migration 000075] Applying gaze page revisions schema'; END $$;

-- ---------------------------------------------------------------------------
-- 1) Edit provenance on gaze_pages
-- last_edit_source records who authored the CURRENT version of the page:
-- 'pipeline' (gaze ingest), 'agent' (gaze fixer tools), 'user' (manual edit
-- via the editor UI) or 'revert' (rollback to an earlier revision). Empty for
-- legacy rows, which are treated as 'pipeline'.
-- ---------------------------------------------------------------------------
ALTER TABLE gaze_pages ADD COLUMN IF NOT EXISTS last_edit_source VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE gaze_pages ADD COLUMN IF NOT EXISTS last_editor_id VARCHAR(64) NOT NULL DEFAULT '';

COMMENT ON COLUMN gaze_pages.last_edit_source IS 'Author kind of the current version: pipeline | agent | user | revert ('''' = legacy, treated as pipeline)';
COMMENT ON COLUMN gaze_pages.last_editor_id IS 'User id of the caller that produced the current version (empty for background pipeline writes)';

-- ---------------------------------------------------------------------------
-- 2) gaze_page_revisions table
-- One immutable snapshot per superseded page version. The CURRENT version
-- lives only in gaze_pages; when an edit replaces version V, the pre-edit
-- state is inserted here as (page_id, V) before the row is rewritten, so
-- every historical version stays diffable and revertable.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS gaze_page_revisions (
    id              VARCHAR(36) PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    page_id         VARCHAR(36) NOT NULL,
    slug            VARCHAR(255) NOT NULL,
    version         INT NOT NULL,
    title           VARCHAR(512) NOT NULL DEFAULT '',
    page_type       VARCHAR(32) NOT NULL DEFAULT 'summary',
    status          VARCHAR(32) NOT NULL DEFAULT 'published',
    content         TEXT NOT NULL DEFAULT '',
    summary         TEXT NOT NULL DEFAULT '',
    aliases         JSONB DEFAULT '[]'::JSONB,
    -- Author of THIS version (mirrors gaze_pages.last_edit_source semantics).
    edit_source     VARCHAR(16) NOT NULL DEFAULT '',
    editor_id       VARCHAR(64) NOT NULL DEFAULT '',
    -- When this version was authored (the page's updated_at while current).
    edited_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Exactly one snapshot per page version; ON CONFLICT DO NOTHING keeps the
-- snapshot-then-update write path idempotent under retries.
CREATE UNIQUE INDEX IF NOT EXISTS idx_gaze_page_revisions_page_version
    ON gaze_page_revisions (page_id, version);

CREATE INDEX IF NOT EXISTS idx_gaze_page_revisions_kb_slug
    ON gaze_page_revisions (knowledge_base_id, slug);

-- ---- source: 000076_knowledge_metadata_external_id_index.up.sql ----
DO $$ BEGIN RAISE NOTICE '[Migration 000076] Adding knowledge metadata external_id prefix index...'; END $$;

-- Speeds up the datasource subtree sweep (repository FindByMetadataKeyPrefix),
-- which runs on every re-sync of a multi-item source node:
--   WHERE knowledge_base_id = $1 AND deleted_at IS NULL
--     AND metadata->>'external_id' LIKE '<parent>#%' ESCAPE '\'
--
-- The knowledge_base_id equality is already served by idx_knowledges_base_id,
-- but the metadata->>'external_id' LIKE-prefix is only a post-filter (Bitmap
-- Heap Scan recheck). This composite expression index lets the prefix match be
-- served by the index too. Two deliberate choices:
--   * text_pattern_ops — a plain btree opclass is NOT used for LIKE 'x%' in a
--     non-C collation; text_pattern_ops compares raw bytes, so it drives the
--     prefix match regardless of the database collation (this DB even carries a
--     collation-version mismatch, which text_pattern_ops sidesteps entirely).
--   * partial on deleted_at IS NULL — matches the query's own predicate and
--     keeps the index limited to live rows.
-- Non-destructive: index-only, no schema or data change. IF NOT EXISTS makes it
-- idempotent and safe to re-run.
CREATE INDEX IF NOT EXISTS idx_knowledges_kb_metadata_external_id
    ON knowledges (knowledge_base_id, (metadata->>'external_id') text_pattern_ops)
    WHERE deleted_at IS NULL;

-- ---- source: 000077_remove_gaze_log.up.sql ----
-- Migration: 000077_remove_gaze_log
-- Description: Remove the duplicate Gaze-only operation feed. Knowledge-base
--              activity is the authoritative mutation history, and Gaze
--              retrieval never reads this table.

DO $$ BEGIN RAISE NOTICE '[Migration 000077] Removing gaze operation log storage'; END $$;

DROP TABLE IF EXISTS gaze_log_entries;

-- Very old installations may still have the pre-000040 log.md row. It is not
-- backed by the activity feed and the model no longer treats it as a special
-- retrieval page, so remove it with the obsolete page type.
DELETE FROM gaze_pages WHERE page_type = 'log';

DO $$ BEGIN RAISE NOTICE '[Migration 000077] Gaze operation log storage removed'; END $$;

-- ---- source: 000078_chunk_editing_and_custom_metadata.up.sql ----
-- Migration 000078: editable chunks, revision history, and custom document metadata.
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS source_content TEXT NOT NULL DEFAULT '';
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS content_revision INT NOT NULL DEFAULT 0;
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS index_status VARCHAR(16) NOT NULL DEFAULT 'ready';
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS last_editor_id VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS context_header TEXT NOT NULL DEFAULT '';
UPDATE chunks SET source_content = content WHERE source_content = '';

ALTER TABLE knowledges ADD COLUMN IF NOT EXISTS custom_metadata JSONB NOT NULL DEFAULT '{}'::JSONB;

CREATE TABLE IF NOT EXISTS chunk_revisions (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    knowledge_id VARCHAR(36) NOT NULL,
    chunk_id VARCHAR(36) NOT NULL,
    revision INT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    editor_id VARCHAR(64) NOT NULL DEFAULT '',
    edit_source VARCHAR(16) NOT NULL DEFAULT 'user',
    edited_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_chunk_revisions_chunk_revision ON chunk_revisions (chunk_id, revision);
CREATE INDEX IF NOT EXISTS idx_chunk_revisions_tenant_chunk ON chunk_revisions (tenant_id, chunk_id);

-- ---- source: 000079_knowledge_folder_path.up.sql ----
-- Folder tree support for knowledge bases.
--
-- Folder uploads used to encode their relative directory inside file_name
-- (e.g. "docs/spec/design.md"), which made the document list show the whole
-- path as the title and left no way to query a single folder. The path now
-- lives in its own column and file_name keeps only the base name.
ALTER TABLE knowledges
    ADD COLUMN IF NOT EXISTS folder_path VARCHAR(1024) NOT NULL DEFAULT '';

-- Backfill legacy folder uploads so existing knowledge bases render the same
-- tree as newly uploaded folders.
UPDATE knowledges
SET folder_path = LEFT(
        REGEXP_REPLACE(file_name, '/[^/]*$', ''),
        1024
    ),
    file_name = REGEXP_REPLACE(file_name, '^.*/', '')
WHERE file_name LIKE '%/%';

CREATE INDEX IF NOT EXISTS idx_knowledges_folder_path
    ON knowledges (tenant_id, knowledge_base_id, folder_path);

-- ---- source: 000080_knowledge_base_auto_tag_config.up.sql ----
-- Migration 000080: opt-in automatic association of existing knowledge-base tags.
ALTER TABLE knowledge_bases ADD COLUMN IF NOT EXISTS auto_tag_config JSONB;

-- ---- source: 000081_message_artifacts.up.sql ----
ALTER TABLE messages ADD COLUMN IF NOT EXISTS artifacts JSONB DEFAULT '[]'::jsonb;

-- ---- source: 000082_tenant_sandbox_config.up.sql ----
-- Description: Store multiple named sandbox backend configs per workspace.
-- Replaces the earlier single tenants.tenant_sandbox_config JSONB column: one
-- workspace can now point different agents at different sandbox backends.
DO $$ BEGIN RAISE NOTICE '[Migration 000082] Creating tenant_sandbox_configs'; END $$;

CREATE TABLE IF NOT EXISTS tenant_sandbox_configs (
    id           VARCHAR(36)  PRIMARY KEY,
    tenant_id    BIGINT       NOT NULL,
    name         VARCHAR(255) NOT NULL,
    description  TEXT,
    sandbox_type VARCHAR(32)  NOT NULL,
    config       JSONB        NOT NULL,
    cordoned_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ
);

COMMENT ON COLUMN tenant_sandbox_configs.config IS 'Sandbox backend config (endpoints, encrypted API keys, env vars, volume mount); shape matches types.TenantSandboxConfig';
COMMENT ON COLUMN tenant_sandbox_configs.cordoned_at IS 'Short lease taken while identity fields (provider/URL/API key) are being changed; sandbox resolution is refused while it is fresh';

CREATE INDEX IF NOT EXISTS idx_tenant_sandbox_configs_tenant
    ON tenant_sandbox_configs (tenant_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_tenant_sandbox_configs_tenant_name
    ON tenant_sandbox_configs (tenant_id, name) WHERE deleted_at IS NULL;

-- ---- source: 000083_session_sandbox_config.up.sql ----
-- Description: Pin which sandbox config a session's live sandbox was created on.
-- NULL means "no live sandbox"; '-' means "the deployment-wide default config".
-- This is an ephemeral pin that dies with the sandbox, not a permanent owner.
DO $$ BEGIN RAISE NOTICE '[Migration 000083] Adding sessions.sandbox_config_id'; END $$;

ALTER TABLE sessions ADD COLUMN IF NOT EXISTS sandbox_config_id VARCHAR(36) DEFAULT NULL;
COMMENT ON COLUMN sessions.sandbox_config_id IS 'Sandbox config the session current live sandbox was created on; NULL = no live sandbox, ''-'' = deployment default';

-- ---- source: 000084_memory.up.sql ----
-- Migration 000084: cross-session long-term memory.
--
-- Two tables only. memory_subjects is one row per (workspace, principal) and
-- caches the resident block that every turn injects; memory_items holds the
-- individual remembered statements.
--
-- Contradictions are resolved by bi-temporal supersede rather than deletion:
-- the outdated row keeps its content and gets invalid_at + superseded_by, so
-- "what did it believe last month" stays answerable and the user can see why a
-- memory changed. Rows are only physically removed when the user forgets them.

CREATE TABLE IF NOT EXISTS memory_subjects (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL,
    -- Principal.StorageID(), e.g. "web_user:<uuid>" or "im_user:wecom:<ch>:<u>".
    subject_id VARCHAR(512) NOT NULL,
    -- Per-user opt out. The workspace-level switch lives on tenants.memory_config.
    enabled BOOLEAN NOT NULL DEFAULT true,
    -- Rendered profile/preference block, recomputed on write and read as-is on
    -- every turn so the read path stays a single primary-key lookup.
    block_text TEXT NOT NULL DEFAULT '',
    block_updated_at TIMESTAMP WITH TIME ZONE,
    item_count INTEGER NOT NULL DEFAULT 0,
    last_extracted_at TIMESTAMP WITH TIME ZONE,
    -- Watermark: everything this subject said up to here has been considered
    -- for distillation. Runs walk forward from it, so a message cannot be
    -- skipped by a timer or by a burst of turns.
    extract_cursor TIMESTAMP WITH TIME ZONE,
    -- Sessions with turns past the cursor, recorded when a turn arrives while
    -- a run is already in flight.
    pending_sessions JSONB,
    -- Set while a distillation task is queued or running, so concurrent turns
    -- enqueue one task rather than one per turn.
    extract_scheduled_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_memory_subjects_scope
    ON memory_subjects (tenant_id, subject_id);

CREATE TABLE IF NOT EXISTS memory_items (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL,
    subject_id VARCHAR(512) NOT NULL,
    -- profile | preference | fact | task
    kind VARCHAR(32) NOT NULL,
    content TEXT NOT NULL,
    -- Readable subject the statement is about, kept verbatim because a question
    -- often names the topic while the statement carries only the value.
    topic VARCHAR(255) NOT NULL DEFAULT '',
    -- Normalized topic key used to detect that a new statement contradicts an
    -- existing one. Two items sharing a key are the same fact at different times.
    normalized_key VARCHAR(255) NOT NULL DEFAULT '',
    importance SMALLINT NOT NULL DEFAULT 3,
    -- explicit (user asked) | extracted (background) | manual (memory editor)
    origin VARCHAR(16) NOT NULL DEFAULT 'extracted',
    -- active | superseded | archived
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    source_session_id VARCHAR(36),
    source_message_id VARCHAR(36),
    valid_from TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    invalid_at TIMESTAMP WITH TIME ZONE,
    -- When the statement stops being worth recalling. Without it an in-flight
    -- task stays in context long after the user finished it.
    expires_at TIMESTAMP WITH TIME ZONE,
    superseded_by VARCHAR(36),
    last_used_at TIMESTAMP WITH TIME ZONE,
    use_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_memory_items_scope
    ON memory_items (tenant_id, subject_id, status);
CREATE INDEX IF NOT EXISTS idx_memory_items_key
    ON memory_items (tenant_id, subject_id, normalized_key);

-- A statement the user deliberately forgot. Only the topic and a fingerprint of
-- the statement are kept, never the statement itself: this table exists to stop
-- distillation from re-adding what was dropped, not to retain it.
CREATE TABLE IF NOT EXISTS memory_tombstones (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL,
    subject_id VARCHAR(512) NOT NULL,
    topic VARCHAR(255) NOT NULL DEFAULT '',
    fingerprint VARCHAR(64) NOT NULL,
    source_message_id VARCHAR(36),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_memory_tombstones_scope
    ON memory_tombstones (tenant_id, subject_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_mem_tomb_fp
    ON memory_tombstones (tenant_id, subject_id, fingerprint);

-- How often this person has asked about a topic. A single question is noise;
-- the same subject across several conversations is a signal, so topics are
-- counted here and only promoted into memory_items once they recur.
CREATE TABLE IF NOT EXISTS memory_topic_stats (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL,
    subject_id VARCHAR(512) NOT NULL,
    normalized_key VARCHAR(255) NOT NULL,
    topic VARCHAR(255) NOT NULL DEFAULT '',
    hits INTEGER NOT NULL DEFAULT 0,
    last_seen_at TIMESTAMP WITH TIME ZONE,
    promoted_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_mem_topic_scope
    ON memory_topic_stats (tenant_id, subject_id, normalized_key);

-- How often this person's answers drew on a document. Read by the reranker to
-- prefer material they keep coming back to.
CREATE TABLE IF NOT EXISTS memory_doc_affinity (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id INTEGER NOT NULL,
    subject_id VARCHAR(512) NOT NULL,
    knowledge_id VARCHAR(36) NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL DEFAULT '',
    title VARCHAR(512) NOT NULL DEFAULT '',
    hits INTEGER NOT NULL DEFAULT 0,
    last_used_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_mem_affinity_scope
    ON memory_doc_affinity (tenant_id, subject_id, knowledge_id);

-- Workspace-level switch, write mode, extraction model and capacity.
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS memory_config JSONB;

-- Which memories were injected into this answer. Persisted rather than only
-- streamed so reopening a conversation still shows what the answer saw.
ALTER TABLE messages ADD COLUMN IF NOT EXISTS used_memories JSONB;

ALTER TABLE memory_subjects ADD COLUMN IF NOT EXISTS consolidated_at TIMESTAMP WITH TIME ZONE;

-- A review someone asked for is rate limited separately from the daily pass:
-- one clock for both would let the daily pass refuse the button.
ALTER TABLE memory_subjects ADD COLUMN IF NOT EXISTS forced_consolidated_at TIMESTAMP WITH TIME ZONE;

ALTER TABLE memory_topic_stats ADD COLUMN IF NOT EXISTS aliases JSONB NOT NULL DEFAULT '[]';

-- Vectors live apart from the items so that listing, capacity enforcement and
-- the resident block never pay to load them.
CREATE TABLE IF NOT EXISTS memory_item_embeddings (
    item_id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    subject_id VARCHAR(512) NOT NULL,
    model_id VARCHAR(64) NOT NULL DEFAULT '',
    dims INTEGER NOT NULL DEFAULT 0,
    vector BYTEA,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_mem_emb_scope
    ON memory_item_embeddings (tenant_id, subject_id);

-- ---- source: 000085_message_usage.up.sql ----
ALTER TABLE messages ADD COLUMN IF NOT EXISTS usage JSONB;

-- ---- source: 000086_tenant_skills.up.sql ----
-- Description: Skills installed onto a sandbox config, plus the snapshot chain
-- ledger. Skills live inside the config's snapshot image; these tables are the
-- metadata projection and the audit trail for the provider-side snapshots.
DO $$ BEGIN RAISE NOTICE '[Migration 000086] Creating tenant_skills'; END $$;

CREATE TABLE IF NOT EXISTS tenant_skills (
    id                    VARCHAR(36)  PRIMARY KEY,
    tenant_id             BIGINT       NOT NULL,
    sandbox_config_id     VARCHAR(36)  NOT NULL,
    name                  VARCHAR(255) NOT NULL,
    version               VARCHAR(64),
    description           TEXT,
    instructions          TEXT,
    bundle_ref            VARCHAR(1024),
    bundle_sha256         VARCHAR(64),
    enabled               BOOLEAN      NOT NULL DEFAULT TRUE,
    installed_snapshot_id VARCHAR(255),
    status                VARCHAR(32)  NOT NULL,
    error                 TEXT,
    installing_since      TIMESTAMPTZ,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at            TIMESTAMPTZ
);

COMMENT ON COLUMN tenant_skills.name IS 'Also the directory name inside the image: /opt/gaze/tenant/skills/<name>';
COMMENT ON COLUMN tenant_skills.enabled IS 'Visibility to the agent only; files stay in the image until the skill is removed';

-- The unique index doubles as the "list skills of a config" index: its leftmost
-- prefix is sandbox_config_id. status/enabled are low-cardinality and are
-- filtered in memory (a config holds tens of skills, not millions).
CREATE UNIQUE INDEX IF NOT EXISTS uq_tenant_skills_config_name
    ON tenant_skills (sandbox_config_id, name) WHERE deleted_at IS NULL;

DO $$ BEGIN RAISE NOTICE '[Migration 000086] Creating tenant_skill_snapshots'; END $$;

CREATE TABLE IF NOT EXISTS tenant_skill_snapshots (
    id                  VARCHAR(36)  PRIMARY KEY,
    tenant_id           BIGINT       NOT NULL,
    sandbox_config_id   VARCHAR(36)  NOT NULL,
    skill_id            VARCHAR(36),
    snapshot_id         VARCHAR(255),
    parent_snapshot_id  VARCHAR(255),
    generation          INTEGER      NOT NULL DEFAULT 0,
    trigger             VARCHAR(16)  NOT NULL,
    state               VARCHAR(16)  NOT NULL,
    superseded_at       TIMESTAMPTZ,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE tenant_skill_snapshots IS 'Image chain ledger. Old snapshots are kept (state=superseded), never deleted on switch, so the recorded IDs stay resolvable.';

CREATE INDEX IF NOT EXISTS idx_tenant_skill_snapshots_config
    ON tenant_skill_snapshots (sandbox_config_id);
CREATE INDEX IF NOT EXISTS idx_tenant_skill_snapshots_state
    ON tenant_skill_snapshots (state);

-- ---- source: 000087_skill_install_transcript.up.sql ----
-- Description: Locators for the installer agent's transcript. The install runs
-- in a maintenance session whose messages are kept for troubleshooting; without
-- these columns the console has no way to find that conversation.
DO $$ BEGIN RAISE NOTICE '[Migration 000087] Adding install transcript locators to tenant_skills'; END $$;

ALTER TABLE tenant_skills ADD COLUMN IF NOT EXISTS install_session_id VARCHAR(36);
ALTER TABLE tenant_skills ADD COLUMN IF NOT EXISTS install_message_id VARCHAR(36);

COMMENT ON COLUMN tenant_skills.install_session_id IS 'Maintenance session of the most recent install; overwritten on re-install';
COMMENT ON COLUMN tenant_skills.install_message_id IS 'Assistant message the installer transcript was streamed into';

-- ---- source: 000088_skill_snapshot_planned_name.up.sql ----
-- Description: The name a snapshot was going to be committed under, recorded
-- before the provider call. snapshot_id can only be filled in afterwards, so a
-- process that died between the commit and that write left a real provider
-- snapshot the ledger could not name — and therefore could never reclaim.
DO $$ BEGIN RAISE NOTICE '[Migration 000088] Adding planned_name to tenant_skill_snapshots'; END $$;

ALTER TABLE tenant_skill_snapshots ADD COLUMN IF NOT EXISTS planned_name VARCHAR(255);

COMMENT ON COLUMN tenant_skill_snapshots.planned_name IS 'Name passed to CreateSnapshot, written before the provider call so an abandoned build stays identifiable';

-- ---- source: 000089_env_vars.up.sql ----
-- Description: Environment variables for skill execution. Two things live here:
-- the per-skill declaration an installer agent produces (with the optional
-- workspace-wide value an admin supplies), and the values individual members
-- keep for themselves.
--
-- A member value with an empty skill_id belongs to the whole sandbox config and
-- is injected into every execution on it; one with a skill_id is that skill's
-- declared credential, injected only when a tool names the skill. The storage is
-- shared because it is the same kind of thing — only the load timing differs.
DO $$ BEGIN RAISE NOTICE '[Migration 000089] Adding tenant_skills.envs'; END $$;

ALTER TABLE tenant_skills ADD COLUMN IF NOT EXISTS envs JSONB;

COMMENT ON COLUMN tenant_skills.envs IS
    'Installer-agent declaration [{name,description,required,value}]. value is the workspace-wide admin value and is AES-GCM encrypted; the rest is plaintext so the UI can render it without a key.';

DO $$ BEGIN RAISE NOTICE '[Migration 000089] Creating tenant_user_env_vars'; END $$;

CREATE TABLE IF NOT EXISTS tenant_user_env_vars (
    id                VARCHAR(36)  PRIMARY KEY,
    tenant_id         BIGINT       NOT NULL,
    principal_type    VARCHAR(32)  NOT NULL,
    principal_id      VARCHAR(512) NOT NULL,
    sandbox_config_id VARCHAR(36)  NOT NULL,
    skill_id          VARCHAR(36)  NOT NULL DEFAULT '',
    name              VARCHAR(255) NOT NULL,
    value             TEXT,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE tenant_user_env_vars IS
    'One principal''s own environment variable. An empty skill_id applies to every execution on the sandbox config; a skill_id scopes it to that skill''s declaration. Keyed by principal, not user_id: the IM path stores a synthetic tenant account in the user ID, which would make every IM user of a workspace share one value.';
COMMENT ON COLUMN tenant_user_env_vars.value IS 'AES-GCM encrypted. Never returned by any endpoint.';

-- Also the read index for resolution: its leftmost prefix is the exact tuple
-- looked up on every execution. sandbox_config_id is part of the key because
-- without it two configs could not each hold a config-wide variable of the
-- same name (their skill_id is empty in both).
CREATE UNIQUE INDEX IF NOT EXISTS uq_user_env_var
    ON tenant_user_env_vars (tenant_id, principal_type, principal_id, sandbox_config_id, skill_id, name);

-- Cleanup indexes. Skills and sandbox configs are soft-deleted, so a foreign
-- key with ON DELETE CASCADE would never fire; deletion is explicit in the
-- repository and needs these to stay cheap.
CREATE INDEX IF NOT EXISTS idx_user_env_var_skill
    ON tenant_user_env_vars (tenant_id, skill_id);
CREATE INDEX IF NOT EXISTS idx_user_env_var_config
    ON tenant_user_env_vars (tenant_id, sandbox_config_id);

-- ---- source: 000090_skill_catalog.up.sql ----
-- Description: Tenant-level skill catalog. A skill exists independently of any
-- sandbox; tenant_skills rows become installations onto one config's image.
DO $$ BEGIN RAISE NOTICE '[Migration 000090] Creating tenant_skill_catalog'; END $$;

CREATE TABLE IF NOT EXISTS tenant_skill_catalog (
    id            VARCHAR(36)  PRIMARY KEY,
    tenant_id     BIGINT       NOT NULL,
    name          VARCHAR(255) NOT NULL,
    version       VARCHAR(64),
    description   TEXT,
    instructions  TEXT,
    bundle_ref    VARCHAR(1024),
    bundle_sha256 VARCHAR(64),
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ
);

COMMENT ON TABLE tenant_skill_catalog IS
    'Workspace skill definition. Installations onto sandbox configs live in tenant_skills.';

CREATE UNIQUE INDEX IF NOT EXISTS uq_tenant_skill_catalog_name
    ON tenant_skill_catalog (tenant_id, name) WHERE deleted_at IS NULL;

DO $$ BEGIN RAISE NOTICE '[Migration 000090] Linking tenant_skills to catalog'; END $$;

ALTER TABLE tenant_skills ADD COLUMN IF NOT EXISTS catalog_id VARCHAR(36);

CREATE INDEX IF NOT EXISTS idx_tenant_skills_catalog
    ON tenant_skills (catalog_id);

-- One catalog row per (tenant, name). Names are the workspace identity, so
-- same-name installs on different sandboxes collapse here. Prefer a row that
-- still has a stored archive, then the most recently updated one, so the
-- definition matches what operators last wrote rather than the first upload.
INSERT INTO tenant_skill_catalog (
    id, tenant_id, name, version, description, instructions,
    bundle_ref, bundle_sha256, created_at, updated_at
)
SELECT DISTINCT ON (tenant_id, name)
    id, tenant_id, name, version, description, instructions,
    bundle_ref, bundle_sha256, created_at, updated_at
FROM tenant_skills
WHERE deleted_at IS NULL
ORDER BY tenant_id, name,
    CASE WHEN bundle_ref IS NULL OR bundle_ref = '' THEN 1 ELSE 0 END,
    updated_at DESC,
    created_at DESC
ON CONFLICT (id) DO NOTHING;

UPDATE tenant_skills AS s
SET catalog_id = c.id
FROM tenant_skill_catalog AS c
WHERE s.deleted_at IS NULL
  AND c.deleted_at IS NULL
  AND s.tenant_id = c.tenant_id
  AND s.name = c.name
  AND (s.catalog_id IS NULL OR s.catalog_id = '');

-- ---- source: 000091_mcp_tool_enabled.up.sql ----
-- Per-tool MCP enable/disable policy. Existing rows and missing rows remain
-- enabled by default so this migration is backwards compatible.
ALTER TABLE mcp_tool_approvals
    ADD COLUMN IF NOT EXISTS enabled BOOLEAN NOT NULL DEFAULT true;

-- ---- source: 000092_mcp_metadata.up.sql ----
ALTER TABLE mcp_services ADD COLUMN IF NOT EXISTS usage_instructions TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS mcp_metadata (
    tenant_id BIGINT NOT NULL,
    service_id VARCHAR(36) NOT NULL REFERENCES mcp_services(id) ON DELETE CASCADE,
    principal VARCHAR(255) NOT NULL DEFAULT '',
    config_fingerprint VARCHAR(64) NOT NULL,
    tools JSONB NOT NULL,
    instructions TEXT NOT NULL DEFAULT '',
    server_name TEXT NOT NULL DEFAULT '',
    server_version TEXT NOT NULL DEFAULT '',
    server_description TEXT NOT NULL DEFAULT '',
    synced_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (tenant_id, service_id, principal)
);

-- ---- source: 000093_ums_domain_metadata.up.sql ----
-- Migration: 000093_ums_domain_metadata
-- Description: Add UMS (银联商务) domain metadata columns for the YLSW retrieval engine.
-- knowledges: document-level fields extracted by docreader (文号/机构/日期/类型)
-- chunks: chunk-level fields used by metadata search and org-scope filtering
DO $$ BEGIN RAISE NOTICE '[Migration 000093] Adding UMS domain metadata columns'; END $$;

ALTER TABLE knowledges ADD COLUMN IF NOT EXISTS doc_type VARCHAR(50);
ALTER TABLE knowledges ADD COLUMN IF NOT EXISTS org_name VARCHAR(255);
ALTER TABLE knowledges ADD COLUMN IF NOT EXISTS doc_number VARCHAR(100);
ALTER TABLE knowledges ADD COLUMN IF NOT EXISTS doc_date VARCHAR(50);

ALTER TABLE chunks ADD COLUMN IF NOT EXISTS doc_type VARCHAR(50);
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS product_name VARCHAR(100);
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS org_name VARCHAR(255);

COMMENT ON COLUMN knowledges.doc_type IS 'UMS 文档类型: regulation/manual/report/news/table/general';
COMMENT ON COLUMN knowledges.org_name IS 'UMS 机构规范名（如 中国人民银行/银联商务）';
COMMENT ON COLUMN knowledges.doc_number IS 'UMS 文号（如 银发2015325号）';
COMMENT ON COLUMN knowledges.doc_date IS 'UMS 文档日期';
COMMENT ON COLUMN chunks.doc_type IS 'UMS 文档类型（冗余自 knowledges，供元数据检索）';
COMMENT ON COLUMN chunks.product_name IS 'UMS 产品名（如 全民付蓝鲸）';
COMMENT ON COLUMN chunks.org_name IS 'UMS 机构规范名（冗余自 knowledges，供机构范围过滤）';

CREATE INDEX IF NOT EXISTS idx_chunks_product_name ON chunks(product_name);
CREATE INDEX IF NOT EXISTS idx_chunks_doc_type ON chunks(doc_type);
CREATE INDEX IF NOT EXISTS idx_chunks_org_name ON chunks(org_name);
CREATE INDEX IF NOT EXISTS idx_knowledges_doc_number ON knowledges(doc_number);

DO $$ BEGIN RAISE NOTICE '[Migration 000093] UMS domain metadata columns added successfully'; END $$;

-- ---- source: 000095_knowledges_product_name.up.sql ----
-- Migration: 000095_knowledges_product_name
-- Description: Add product_name column to knowledges (UMS 域元数据，chunks 冗余来源)
DO $$ BEGIN RAISE NOTICE '[Migration 000095] Adding product_name column to knowledges'; END $$;

ALTER TABLE knowledges ADD COLUMN IF NOT EXISTS product_name VARCHAR(100);

COMMENT ON COLUMN knowledges.product_name IS 'UMS 产品名（如 全民付蓝鲸）';

DO $$ BEGIN RAISE NOTICE '[Migration 000095] product_name column added successfully'; END $$;

-- ---- source: 000099_embeddings_bytea.up.sql ----
-- Migration: embeddings BYTEA (native vector store retirement, §2.15)
-- Description: embeddings.embedding halfvec -> BYTEA. The app no longer
-- performs vector search in-database; similarity is computed in Go over
-- BYTEA-encoded float32 vectors (see retriever/postgres VectorRetrieve).
-- Only FAQ chunks and ephemeral web-search KBs still write vectors;
-- regular knowledge chunks were migrated to the YLSW engine (§2.2).
--
-- Stored halfvec rows are dropped: they are pure dual-write redundancy for
-- regular chunks (YLSW holds the authoritative copy) and there were no FAQ
-- chunks persisted at migration time. pgvector's halfvec type has no cast
-- to or from bytea, so the column is dropped and re-added instead of
-- ALTERed in place.

DELETE FROM embeddings;

-- HNSW expression indexes must go before the column they reference.
DROP INDEX IF EXISTS embeddings_embedding_idx_3584;
DROP INDEX IF EXISTS embeddings_embedding_idx_798;

ALTER TABLE embeddings DROP COLUMN embedding;
ALTER TABLE embeddings ADD COLUMN embedding BYTEA;

-- ---- source: 000100_gaze_outdated.up.sql ----
-- Migration: gaze page outdated markers (wiki 版本过期同步, §2.15 对齐)
-- Description: gaze_pages gains outdated + superseded_by_slug. Summary pages
-- sharing one doc_number (UMS 文号) form a version chain: the newest page
-- stays current, older pages are flagged outdated and point at the page that
-- supersedes them via superseded_by_slug.
ALTER TABLE gaze_pages ADD COLUMN IF NOT EXISTS outdated BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE gaze_pages ADD COLUMN IF NOT EXISTS superseded_by_slug VARCHAR(255) NOT NULL DEFAULT '';

-- ---- source: 000102_message_rag_stage_timeline.up.sql ----
-- Migration: assistant 消息增加检索过程时间线（YLSW stage 事件结构化日志）
-- Description: 存检索各阶段（召回/融合/重排/上下文/生成）及相对耗时，
-- 前端刷新/重载历史后仍可展开「检索过程」。空数组 = 无检索过程。
ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS rag_stage_timeline JSONB NOT NULL DEFAULT '[]'::jsonb;

-- ---- source: 000103_message_query_rewritten.up.sql ----
-- Migration: user 消息记录指代消解后的实际检索 query（需求五）
-- Description: 空字符串表示本轮未改写或改写失败放行；非空时前端在
-- 用户消息下展示「已改写为」，历史刷新后仍保留。
ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS query_rewritten TEXT NOT NULL DEFAULT '';

-- ---- source: 000104_message_query_rewrite_meta.up.sql ----
-- Migration: user 消息记录指代消解的展开信息（需求五 UI 返工）
-- Description: 保存原文/改写文的实体映射、上轮回答锚点、must_keep 校验与耗时，
-- 供刷新历史后展开「改写已完成」卡片使用。
ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS query_rewrite_meta JSONB;

-- ---- source: 000105_collaboration.up.sql ----
-- Migration 000105: document uploader and engine sync state.
-- Collaboration review tables were removed for the personal edition.

ALTER TABLE knowledges ADD COLUMN IF NOT EXISTS created_by VARCHAR(36);
ALTER TABLE knowledges ADD COLUMN IF NOT EXISTS engine_sync_status VARCHAR(20) NOT NULL DEFAULT 'synced';
ALTER TABLE knowledges ADD COLUMN IF NOT EXISTS engine_sync_requested_at TIMESTAMPTZ;
ALTER TABLE knowledges ADD COLUMN IF NOT EXISTS engine_sync_completed_at TIMESTAMPTZ;
ALTER TABLE knowledges ADD COLUMN IF NOT EXISTS engine_sync_error TEXT NOT NULL DEFAULT '';

-- 存量文档没有独立上传者，按父 KB 创建者近似回填。
UPDATE knowledges k
SET created_by = kb.creator_id
FROM knowledge_bases kb
WHERE k.created_by IS NULL
  AND kb.id = k.knowledge_base_id
  AND kb.creator_id IS NOT NULL
  AND kb.creator_id <> '';

CREATE INDEX IF NOT EXISTS idx_knowledges_created_by ON knowledges(created_by);
CREATE INDEX IF NOT EXISTS idx_knowledges_engine_sync ON knowledges(tenant_id, knowledge_base_id, engine_sync_status);

ALTER TABLE tenants ADD COLUMN IF NOT EXISTS engine_sync_config JSONB;

-- ---- source: 000106_collaboration_v2.up.sql ----
-- Migration 000106: document versions, chunk snapshots and citation counters.

CREATE TABLE IF NOT EXISTS doc_versions (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    knowledge_id VARCHAR(36) NOT NULL,
    version_seq INTEGER NOT NULL,
    version_label VARCHAR(16) NOT NULL,
    source_type VARCHAR(24) NOT NULL,
    source_id VARCHAR(36) NOT NULL DEFAULT '',
    title VARCHAR(255) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    chunk_count INTEGER NOT NULL DEFAULT 0,
    created_by VARCHAR(36) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_doc_versions_knowledge_seq ON doc_versions(knowledge_id, version_seq);
CREATE INDEX IF NOT EXISTS idx_doc_versions_kb ON doc_versions(tenant_id, knowledge_base_id, knowledge_id);

CREATE TABLE IF NOT EXISTS doc_version_chunks (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    version_id VARCHAR(36) NOT NULL,
    chunk_id VARCHAR(36) NOT NULL,
    chunk_index INTEGER NOT NULL DEFAULT 0,
    content TEXT NOT NULL DEFAULT '',
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE
);
CREATE INDEX IF NOT EXISTS idx_doc_version_chunks_version ON doc_version_chunks(version_id, chunk_index);
CREATE INDEX IF NOT EXISTS idx_doc_version_chunks_chunk ON doc_version_chunks(chunk_id);

CREATE TABLE IF NOT EXISTS chunk_citation_daily (
    chunk_id VARCHAR(36) NOT NULL,
    tenant_id BIGINT NOT NULL,
    stat_date DATE NOT NULL,
    citation_count BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (chunk_id, stat_date)
);
CREATE INDEX IF NOT EXISTS idx_chunk_citation_daily_tenant ON chunk_citation_daily(tenant_id, stat_date);

-- ---- source: 000109_knowledge_folders.up.sql ----
-- 000109: real knowledge folders + document folder_id backfill.
-- folder_path stays for one compatibility release; folder_id is authoritative.

CREATE TABLE IF NOT EXISTS knowledge_folders (
    id          VARCHAR(64) PRIMARY KEY,
    tenant_id   BIGINT NOT NULL,
    kb_id       VARCHAR(36) NOT NULL,
    parent_id   VARCHAR(64) NOT NULL DEFAULT '',
    name        VARCHAR(128) NOT NULL,
    path        VARCHAR(1024) NOT NULL DEFAULT '',
    depth       INTEGER NOT NULL DEFAULT 0,
    is_root     BOOLEAN NOT NULL DEFAULT FALSE,
    created_by  VARCHAR(36) NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at  TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_knowledge_folders_kb_path
    ON knowledge_folders (kb_id, path) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_knowledge_folders_sibling
    ON knowledge_folders (kb_id, parent_id, name)
    WHERE deleted_at IS NULL AND is_root = FALSE;
CREATE UNIQUE INDEX IF NOT EXISTS idx_knowledge_folders_root
    ON knowledge_folders (kb_id) WHERE deleted_at IS NULL AND is_root = TRUE;
CREATE INDEX IF NOT EXISTS idx_knowledge_folders_kb
    ON knowledge_folders (kb_id) WHERE deleted_at IS NULL;

-- One root folder per knowledge base.
INSERT INTO knowledge_folders (id, tenant_id, kb_id, parent_id, name, path, depth, is_root, created_by)
SELECT 'folder-root-' || kb.id, kb.tenant_id, kb.id, '', '', '', 0, TRUE, kb.creator_id
FROM knowledge_bases kb
WHERE kb.deleted_at IS NULL
ON CONFLICT DO NOTHING;

-- Materialize every prefix of every existing folder_path.
WITH RECURSIVE paths AS (
    SELECT DISTINCT tenant_id, knowledge_base_id, folder_path
    FROM knowledges
    WHERE deleted_at IS NULL AND folder_path <> ''
), split AS (
    SELECT tenant_id, knowledge_base_id, string_to_array(folder_path, '/') AS all_parts, 1 AS depth
    FROM paths
    UNION ALL
    SELECT tenant_id, knowledge_base_id, all_parts, depth + 1
    FROM split
    WHERE depth < array_length(all_parts, 1)
)
INSERT INTO knowledge_folders (id, tenant_id, kb_id, parent_id, name, path, depth, is_root, created_by)
SELECT
    md5(knowledge_base_id || ':' || array_to_string(all_parts[1:depth], '/')),
    tenant_id,
    knowledge_base_id,
    CASE
        WHEN depth = 1 THEN 'folder-root-' || knowledge_base_id
        ELSE md5(knowledge_base_id || ':' || array_to_string(all_parts[1:depth - 1], '/'))
    END,
    all_parts[depth],
    array_to_string(all_parts[1:depth], '/'),
    depth,
    FALSE,
    ''
FROM split
ON CONFLICT DO NOTHING;

ALTER TABLE knowledges ADD COLUMN IF NOT EXISTS folder_id VARCHAR(64) NOT NULL DEFAULT '';

UPDATE knowledges k
SET folder_id = COALESCE(
    (
        SELECT f.id
        FROM knowledge_folders f
        WHERE f.kb_id = k.knowledge_base_id
          AND f.path = k.folder_path
          AND f.deleted_at IS NULL
        LIMIT 1
    ),
    'folder-root-' || k.knowledge_base_id
)
WHERE k.folder_id = '' OR k.folder_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_knowledges_folder_id
    ON knowledges (tenant_id, knowledge_base_id, folder_id);

-- ---- source: 000112_retrieval_audit.up.sql ----
-- 000112: retrieval audit trail.
-- Best-effort per-request record of the ACL scope the engine applied.
-- Failure to write must never block retrieval.

CREATE TABLE IF NOT EXISTS retrieval_audit_logs (
    id               BIGSERIAL PRIMARY KEY,
    tenant_id        BIGINT NOT NULL,
    user_id          VARCHAR(36) NOT NULL DEFAULT '',
    principal_kind   VARCHAR(32) NOT NULL DEFAULT 'member',
    session_id       VARCHAR(36) NOT NULL DEFAULT '',
    query_digest     VARCHAR(128) NOT NULL DEFAULT '',
    kb_ids           TEXT NOT NULL DEFAULT '',
    scope_mode       VARCHAR(8) NOT NULL DEFAULT '',
    scope_version    BIGINT NOT NULL DEFAULT 0,
    scope_doc_count  INTEGER NOT NULL DEFAULT 0,
    hit_doc_ids      TEXT NOT NULL DEFAULT '',
    decision         VARCHAR(16) NOT NULL DEFAULT '',
    error            TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_retrieval_audit_tenant_time
    ON retrieval_audit_logs (tenant_id, created_at);
CREATE INDEX IF NOT EXISTS idx_retrieval_audit_user_time
    ON retrieval_audit_logs (tenant_id, user_id, created_at);

-- ---- source: 000113_skill_injection.up.sql ----
-- 000113: skill enable flag + per-assistant-message skill metadata.

ALTER TABLE tenant_skill_catalog ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE messages ADD COLUMN skill JSONB;

-- ---- source: 000114_reading_runs.up.sql ----
-- 000114: interactive document reading runs.

ALTER TABLE tenants ADD COLUMN reading_model_id VARCHAR(36) NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS reading_runs (
    id               VARCHAR(36) PRIMARY KEY,
    tenant_id        BIGINT NOT NULL,
    session_id       VARCHAR(36) NOT NULL DEFAULT '',
    user_id          VARCHAR(36) NOT NULL DEFAULT '',
    message_id       VARCHAR(36) NOT NULL DEFAULT '',
    document_ids     JSONB NOT NULL DEFAULT '[]'::jsonb,
    skill_id         VARCHAR(36) NOT NULL DEFAULT '',
    reading_model_id VARCHAR(36) NOT NULL DEFAULT '',
    status           VARCHAR(16) NOT NULL DEFAULT 'planning',
    plan             JSONB NOT NULL DEFAULT '{}'::jsonb,
    notes            JSONB NOT NULL DEFAULT '{}'::jsonb,
    final            JSONB NOT NULL DEFAULT '{}'::jsonb,
    current_section_id VARCHAR(64) NOT NULL DEFAULT '',
    step_count       INTEGER NOT NULL DEFAULT 0,
    token_usage      INTEGER NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_reading_runs_user
    ON reading_runs (tenant_id, user_id, session_id, created_at);
CREATE INDEX IF NOT EXISTS idx_reading_runs_active
    ON reading_runs (tenant_id, user_id, session_id, status);

CREATE TABLE IF NOT EXISTS reading_run_steps (
    id            VARCHAR(36) PRIMARY KEY,
    run_id        VARCHAR(36) NOT NULL,
    seq           INTEGER NOT NULL,
    action        VARCHAR(24) NOT NULL,
    section_id    VARCHAR(64) NOT NULL DEFAULT '',
    question      JSONB NOT NULL DEFAULT '{}'::jsonb,
    user_feedback TEXT NOT NULL DEFAULT '',
    notes_patch   JSONB NOT NULL DEFAULT '{}'::jsonb,
    citations     JSONB NOT NULL DEFAULT '[]'::jsonb,
    status        VARCHAR(16) NOT NULL DEFAULT 'done',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_reading_run_steps_run
    ON reading_run_steps (run_id, seq);

-- ---- source: 000115_message_reading_run.up.sql ----
-- 000115: link chat messages to interactive reading runs.

ALTER TABLE messages ADD COLUMN reading_run_id VARCHAR(36) NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_messages_reading_run
    ON messages (reading_run_id);

-- ---- source: 000117_retrieval_audit_created_at_index.up.sql ----
-- 000117: index for retrieval-audit retention sweeps.
CREATE INDEX IF NOT EXISTS idx_retrieval_audit_created_at
    ON retrieval_audit_logs (created_at);

-- ---- source: 000120_session_reading_run.up.sql ----
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS reading_run_id VARCHAR(36) NOT NULL DEFAULT '';

-- ---- source: 000121_reading_state_tables.up.sql ----
-- 000121: structured state for interactive document reading.

CREATE TABLE IF NOT EXISTS reading_questions (
    id                 VARCHAR(36) PRIMARY KEY,
    run_id             VARCHAR(36) NOT NULL,
    tenant_id          BIGINT NOT NULL,
    parent_id          VARCHAR(36) NOT NULL DEFAULT '',
    section_id         VARCHAR(64) NOT NULL DEFAULT '',
    round              INTEGER NOT NULL DEFAULT 0,
    text               TEXT NOT NULL DEFAULT '',
    recommended_answer TEXT NOT NULL DEFAULT '',
    why                TEXT NOT NULL DEFAULT '',
    search_query       TEXT NOT NULL DEFAULT '',
    evidence           JSONB NOT NULL DEFAULT '[]'::jsonb,
    answer             TEXT NOT NULL DEFAULT '',
    answer_source      VARCHAR(16) NOT NULL DEFAULT '',
    status             VARCHAR(20) NOT NULL DEFAULT 'pending',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_reading_questions_run
    ON reading_questions (run_id, round, created_at);
CREATE INDEX IF NOT EXISTS idx_reading_questions_status
    ON reading_questions (run_id, status);

CREATE TABLE IF NOT EXISTS reading_conclusions (
    id          VARCHAR(36) PRIMARY KEY,
    run_id      VARCHAR(36) NOT NULL,
    tenant_id   BIGINT NOT NULL,
    section_id  VARCHAR(64) NOT NULL DEFAULT '',
    round       INTEGER NOT NULL DEFAULT 0,
    text        TEXT NOT NULL DEFAULT '',
    evidence    JSONB NOT NULL DEFAULT '[]'::jsonb,
    status      VARCHAR(20) NOT NULL DEFAULT 'candidate',
    source      VARCHAR(16) NOT NULL DEFAULT 'model',
    gates       JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_reading_conclusions_run
    ON reading_conclusions (run_id, status, created_at);

CREATE TABLE IF NOT EXISTS reading_terms (
    id          VARCHAR(36) PRIMARY KEY,
    run_id      VARCHAR(36) NOT NULL,
    tenant_id   BIGINT NOT NULL,
    term        VARCHAR(200) NOT NULL DEFAULT '',
    definition  TEXT NOT NULL DEFAULT '',
    source      JSONB NOT NULL DEFAULT '[]'::jsonb,
    canonical   BOOLEAN NOT NULL DEFAULT FALSE,
    status      VARCHAR(20) NOT NULL DEFAULT 'candidate',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_reading_terms_run
    ON reading_terms (run_id, created_at);

CREATE TABLE IF NOT EXISTS reading_contradictions (
    id          VARCHAR(36) PRIMARY KEY,
    run_id      VARCHAR(36) NOT NULL,
    tenant_id   BIGINT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    evidence_a  JSONB NOT NULL DEFAULT '[]'::jsonb,
    evidence_b  JSONB NOT NULL DEFAULT '[]'::jsonb,
    status      VARCHAR(20) NOT NULL DEFAULT 'open',
    resolution  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_reading_contradictions_run
    ON reading_contradictions (run_id, status, created_at);

-- ---- source: 000122_reading_report_drafts.up.sql ----
-- 000122: user-edited reading report drafts.

CREATE TABLE IF NOT EXISTS reading_report_drafts (
    run_id     VARCHAR(36) PRIMARY KEY,
    tenant_id  BIGINT NOT NULL,
    user_id    VARCHAR(36) NOT NULL,
    content_md TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_reading_report_drafts_user
    ON reading_report_drafts (tenant_id, user_id);

-- ---- source: 000123_reading_report_draft_version.up.sql ----
-- 000123: optimistic versioning for edited reading report drafts.

ALTER TABLE reading_report_drafts
    ADD COLUMN version INTEGER NOT NULL DEFAULT 0;

-- ---- source: 000124_reading_question_kind.up.sql ----
-- 000124: question kind and gap type for the reading question quality gate.

ALTER TABLE reading_questions ADD COLUMN kind VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE reading_questions ADD COLUMN gap_type VARCHAR(24) NOT NULL DEFAULT '';
