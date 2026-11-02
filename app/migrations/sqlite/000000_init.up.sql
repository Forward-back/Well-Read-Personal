-- WellRead Personal baseline schema (SQLite).
-- Squashed from the enterprise-era versioned migration chain.
-- Fresh databases only: apply once, then never edit this file in place.

-- ---- source: 000000_init.up.sql ----
-- SQLite schema for Gaze Lite (consolidated from all Postgres migrations)

CREATE TABLE IF NOT EXISTS tenants (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    retriever_engines TEXT NOT NULL DEFAULT '[]',
    status VARCHAR(50) DEFAULT 'active',
    business VARCHAR(255) NOT NULL,
    storage_quota BIGINT NOT NULL DEFAULT 10737418240,
    storage_used BIGINT NOT NULL DEFAULT 0,
    agent_config TEXT DEFAULT NULL,
    context_config TEXT,
    conversation_config TEXT,
    web_search_config TEXT DEFAULT NULL,
    parser_engine_config TEXT DEFAULT NULL,
    storage_engine_config TEXT DEFAULT NULL,
    default_storage_backend_id VARCHAR(36),
    credentials TEXT DEFAULT NULL,
    chat_history_config TEXT,
    retrieval_config TEXT,
    agent_system_prompt TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_tenants_status ON tenants(status);

CREATE TABLE IF NOT EXISTS models (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    type VARCHAR(50) NOT NULL,
    source VARCHAR(50) NOT NULL,
    description TEXT,
    parameters TEXT NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT 0,
    is_builtin BOOLEAN NOT NULL DEFAULT 0,
    managed_by VARCHAR(32) NOT NULL DEFAULT '',
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_models_type ON models(type);
CREATE INDEX IF NOT EXISTS idx_models_source ON models(source);
CREATE INDEX IF NOT EXISTS idx_models_is_builtin ON models(is_builtin);
CREATE INDEX IF NOT EXISTS idx_models_managed_by ON models(managed_by);

CREATE TABLE IF NOT EXISTS knowledge_bases (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    tenant_id INTEGER NOT NULL,
    type VARCHAR(32) NOT NULL DEFAULT 'document',
    chunking_config TEXT NOT NULL DEFAULT '{"chunk_size": 512, "chunk_overlap": 50, "split_markers": ["\n\n", "\n", "。"], "keep_separator": true}',
    image_processing_config TEXT NOT NULL DEFAULT '{"enable_multimodal": false, "model_id": ""}',
    embedding_model_id VARCHAR(64) NOT NULL,
    summary_model_id VARCHAR(64) NOT NULL,
    cos_config TEXT NOT NULL DEFAULT '{}',
    storage_provider_config TEXT DEFAULT NULL,
    vlm_config TEXT NOT NULL DEFAULT '{}',
    extract_config TEXT NULL DEFAULT NULL,
    faq_config TEXT,
    question_generation_config TEXT NULL,
    is_temporary BOOLEAN NOT NULL DEFAULT 0,
    is_pinned INTEGER NOT NULL DEFAULT 0,
    pinned_at DATETIME NULL,
    asr_config TEXT,
    vector_store_id VARCHAR(36),
    storage_backend_id VARCHAR(36),
    creator_id VARCHAR(36),
    gaze_config TEXT,
    indexing_strategy TEXT DEFAULT '{"vector_enabled":true,"keyword_enabled":true,"gaze_enabled":false,"graph_enabled":false}',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_knowledge_bases_tenant_id ON knowledge_bases(tenant_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_bases_tenant_vector_store
    ON knowledge_bases(tenant_id, vector_store_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_bases_storage_backend
    ON knowledge_bases(tenant_id, storage_backend_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_bases_tenant_creator
    ON knowledge_bases(tenant_id, creator_id);

CREATE TABLE IF NOT EXISTS knowledges (
    id VARCHAR(36) PRIMARY KEY,
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
    storage_size BIGINT NOT NULL DEFAULT 0,
    metadata TEXT,
    custom_metadata TEXT NOT NULL DEFAULT '{}',
    tag_id VARCHAR(36),
    summary_status VARCHAR(32) DEFAULT 'none',
    last_faq_import_result TEXT DEFAULT NULL,
    channel VARCHAR(50) NOT NULL DEFAULT 'web',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    processed_at DATETIME,
    error_message TEXT,
    deleted_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_knowledges_tenant_id ON knowledges(tenant_id);
CREATE INDEX IF NOT EXISTS idx_knowledges_base_id ON knowledges(knowledge_base_id);
CREATE INDEX IF NOT EXISTS idx_knowledges_parse_status ON knowledges(parse_status);
CREATE INDEX IF NOT EXISTS idx_knowledges_enable_status ON knowledges(enable_status);
CREATE INDEX IF NOT EXISTS idx_knowledges_tag ON knowledges(tag_id);
CREATE INDEX IF NOT EXISTS idx_knowledges_summary_status ON knowledges(summary_status);

CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    title VARCHAR(255),
    description TEXT,
    knowledge_base_id VARCHAR(36),
    max_rounds INTEGER NOT NULL DEFAULT 5,
    enable_rewrite BOOLEAN NOT NULL DEFAULT 1,
    fallback_strategy VARCHAR(255) NOT NULL DEFAULT 'fixed',
    fallback_response TEXT NOT NULL DEFAULT '很抱歉，我暂时无法回答这个问题。',
    keyword_threshold FLOAT NOT NULL DEFAULT 0.5,
    vector_threshold FLOAT NOT NULL DEFAULT 0.5,
    rerank_model_id VARCHAR(64),
    embedding_top_k INTEGER NOT NULL DEFAULT 10,
    rerank_top_k INTEGER NOT NULL DEFAULT 10,
    rerank_threshold FLOAT NOT NULL DEFAULT 0.65,
    summary_model_id VARCHAR(64),
    summary_parameters TEXT NOT NULL DEFAULT '{}',
    agent_config TEXT DEFAULT NULL,
    context_config TEXT DEFAULT NULL,
    agent_id VARCHAR(36),
    user_id VARCHAR(512),
    is_pinned BOOLEAN NOT NULL DEFAULT 0,
    pinned_at DATETIME,
    sandbox_config_id VARCHAR(36),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_sessions_tenant_id ON sessions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_sessions_agent_id ON sessions(agent_id);
CREATE INDEX IF NOT EXISTS idx_sessions_tenant_user_pin
    ON sessions (tenant_id, user_id, is_pinned, pinned_at, updated_at)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS messages (
    id VARCHAR(36) PRIMARY KEY,
    request_id VARCHAR(36) NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    role VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    rendered_content TEXT NOT NULL DEFAULT '',
    knowledge_references TEXT NOT NULL DEFAULT '[]',
    agent_steps TEXT DEFAULT NULL,
    mentioned_items TEXT DEFAULT '[]',
    images TEXT DEFAULT '[]',
    artifacts TEXT DEFAULT '[]',
    is_completed BOOLEAN NOT NULL DEFAULT 0,
    is_fallback BOOLEAN NOT NULL DEFAULT 0,
    channel VARCHAR(50) NOT NULL DEFAULT '',
    agent_id VARCHAR(36) NOT NULL DEFAULT '',
    agent_tenant_id INTEGER NOT NULL DEFAULT 0,
    model_id VARCHAR(64) NOT NULL DEFAULT '',
    execution_context TEXT NOT NULL DEFAULT '{}',
    agent_duration_ms INTEGER DEFAULT 0,
    knowledge_id VARCHAR(36),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_messages_session_id ON messages(session_id);
CREATE INDEX IF NOT EXISTS idx_messages_knowledge_id ON messages(knowledge_id);
CREATE INDEX IF NOT EXISTS idx_messages_agent_id ON messages(agent_id);

CREATE TABLE IF NOT EXISTS message_suggestion_sets (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    assistant_message_id VARCHAR(36) NOT NULL,
    agent_id VARCHAR(36) NOT NULL DEFAULT '',
    agent_tenant_id INTEGER NOT NULL DEFAULT 0,
    placement VARCHAR(32) NOT NULL,
    config_hash VARCHAR(64) NOT NULL,
    locale VARCHAR(16) NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL,
    allow_regenerate BOOLEAN NOT NULL DEFAULT 0,
    suppression_reason VARCHAR(64) NOT NULL DEFAULT '',
    questions TEXT NOT NULL DEFAULT '[]',
    model_id VARCHAR(64) NOT NULL DEFAULT '',
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    error_code VARCHAR(64) NOT NULL DEFAULT '',
    lease_until DATETIME,
    generated_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_message_suggestion_sets_cache_key
    ON message_suggestion_sets(tenant_id, assistant_message_id, placement, config_hash, locale);
CREATE INDEX IF NOT EXISTS idx_message_suggestion_sets_session
    ON message_suggestion_sets(tenant_id, session_id, created_at);
CREATE INDEX IF NOT EXISTS idx_message_suggestion_sets_status
    ON message_suggestion_sets(status, lease_until);

CREATE TABLE IF NOT EXISTS message_suggestion_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    suggestion_set_id VARCHAR(36) NOT NULL,
    question_id VARCHAR(64) NOT NULL DEFAULT '',
    event_type VARCHAR(32) NOT NULL,
    actor_id VARCHAR(512) NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (suggestion_set_id) REFERENCES message_suggestion_sets(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_message_suggestion_events_set
    ON message_suggestion_events(suggestion_set_id, created_at);
CREATE INDEX IF NOT EXISTS idx_message_suggestion_events_session
    ON message_suggestion_events(tenant_id, session_id, created_at);
CREATE INDEX IF NOT EXISTS idx_message_suggestion_events_type
    ON message_suggestion_events(event_type, created_at);

CREATE TABLE IF NOT EXISTS chunks (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    knowledge_id VARCHAR(36) NOT NULL,
    content TEXT NOT NULL,
    source_content TEXT NOT NULL DEFAULT '',
    content_revision INTEGER NOT NULL DEFAULT 0,
    index_status VARCHAR(16) NOT NULL DEFAULT 'ready',
    last_editor_id VARCHAR(64) NOT NULL DEFAULT '',
    context_header TEXT NOT NULL DEFAULT '',
    chunk_index INTEGER NOT NULL,
    is_enabled BOOLEAN NOT NULL DEFAULT 1,
    start_at INTEGER NOT NULL,
    end_at INTEGER NOT NULL,
    pre_chunk_id VARCHAR(36),
    next_chunk_id VARCHAR(36),
    chunk_type VARCHAR(20) NOT NULL DEFAULT 'text',
    parent_chunk_id VARCHAR(36),
    image_info TEXT,
    video_info TEXT,
    relation_chunks TEXT,
    indirect_relation_chunks TEXT,
    metadata TEXT,
    tag_id VARCHAR(36),
    status INTEGER NOT NULL DEFAULT 0,
    content_hash VARCHAR(64),
    flags INTEGER NOT NULL DEFAULT 1,
    seq_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_chunks_tenant_kg ON chunks(tenant_id, knowledge_id);
CREATE INDEX IF NOT EXISTS idx_chunks_parent_id ON chunks(parent_chunk_id);
CREATE INDEX IF NOT EXISTS idx_chunks_chunk_type ON chunks(chunk_type);
CREATE INDEX IF NOT EXISTS idx_chunks_tag ON chunks(tag_id);
CREATE INDEX IF NOT EXISTS idx_chunks_content_hash ON chunks(content_hash);
CREATE UNIQUE INDEX IF NOT EXISTS idx_chunks_seq_id ON chunks(seq_id);
CREATE INDEX IF NOT EXISTS idx_chunks_kb_tenant ON chunks(knowledge_base_id, tenant_id);
CREATE INDEX IF NOT EXISTS idx_chunks_knowledge_enabled ON chunks(knowledge_id, is_enabled, deleted_at);

CREATE TABLE IF NOT EXISTS chunk_revisions (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    knowledge_id VARCHAR(36) NOT NULL,
    chunk_id VARCHAR(36) NOT NULL,
    revision INTEGER NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    is_enabled BOOLEAN NOT NULL DEFAULT 1,
    editor_id VARCHAR(64) NOT NULL DEFAULT '',
    edit_source VARCHAR(16) NOT NULL DEFAULT 'user',
    edited_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(chunk_id, revision)
);
CREATE INDEX IF NOT EXISTS idx_chunk_revisions_tenant_chunk ON chunk_revisions(tenant_id, chunk_id);

CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(36) PRIMARY KEY,
    username VARCHAR(100) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    avatar VARCHAR(500),
    tenant_id INTEGER,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    can_access_all_tenants BOOLEAN NOT NULL DEFAULT 0,
    -- Per-user JSON preferences (memory toggle, future UI knobs).
    -- SQLite has no JSONB; store as TEXT and let GORM (de)serialise via
    -- the driver.Valuer / sql.Scanner methods on types.UserPreferences.
    preferences TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_tenant_id ON users(tenant_id);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);

CREATE TABLE IF NOT EXISTS auth_tokens (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    token TEXT NOT NULL,
    token_type VARCHAR(50) NOT NULL,
    expires_at DATETIME NOT NULL,
    is_revoked BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_auth_tokens_user_id ON auth_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_auth_tokens_token ON auth_tokens(token);
CREATE INDEX IF NOT EXISTS idx_auth_tokens_token_type ON auth_tokens(token_type);
CREATE INDEX IF NOT EXISTS idx_auth_tokens_expires_at ON auth_tokens(expires_at);

-- user_kb_pins — sqlite mirror of migration 000050. Per-(user, tenant)
-- pinned knowledge bases; replaces the tenant-wide knowledge_bases.is_pinned
-- column for ordering purposes. The legacy column on knowledge_bases is
-- still defined above for back-compat with existing rows but is no longer
-- written by the application.
CREATE TABLE IF NOT EXISTS user_kb_pins (
    tenant_id INTEGER NOT NULL,
    user_id VARCHAR(36) NOT NULL,
    kb_id VARCHAR(36) NOT NULL,
    pinned_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, user_id, kb_id)
);
CREATE INDEX IF NOT EXISTS idx_user_kb_pins_user_tenant_pinned_at
    ON user_kb_pins(tenant_id, user_id, pinned_at DESC);

CREATE TABLE IF NOT EXISTS knowledge_tags (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    name VARCHAR(128) NOT NULL,
    color VARCHAR(32),
    sort_order INTEGER NOT NULL DEFAULT 0,
    seq_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_knowledge_tags_kb_name ON knowledge_tags(tenant_id, knowledge_base_id, name);
CREATE INDEX IF NOT EXISTS idx_knowledge_tags_kb ON knowledge_tags(tenant_id, knowledge_base_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_knowledge_tags_seq_id ON knowledge_tags(seq_id);

CREATE TABLE IF NOT EXISTS mcp_services (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    enabled BOOLEAN DEFAULT 1,
    transport_type VARCHAR(50) NOT NULL,
    url VARCHAR(512),
    headers TEXT,
    auth_config TEXT,
    advanced_config TEXT,
    stdio_config TEXT,
    env_vars TEXT,
    is_builtin BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_mcp_services_tenant_id ON mcp_services(tenant_id);
CREATE INDEX IF NOT EXISTS idx_mcp_services_enabled ON mcp_services(enabled);
CREATE INDEX IF NOT EXISTS idx_mcp_services_is_builtin ON mcp_services(is_builtin);
CREATE INDEX IF NOT EXISTS idx_mcp_services_deleted_at ON mcp_services(deleted_at);

CREATE TABLE IF NOT EXISTS mcp_tool_approvals (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    service_id VARCHAR(36) NOT NULL,
    tool_name VARCHAR(512) NOT NULL,
    require_approval BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (service_id) REFERENCES mcp_services(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_mcp_tool_approvals_tenant_svc_tool ON mcp_tool_approvals(tenant_id, service_id, tool_name);
CREATE INDEX IF NOT EXISTS idx_mcp_tool_approvals_service_id ON mcp_tool_approvals(service_id);

CREATE TABLE IF NOT EXISTS mcp_oauth_clients (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    service_id VARCHAR(36) NOT NULL,
    client_id VARCHAR(512) NOT NULL,
    client_secret TEXT,
    redirect_uri VARCHAR(1024),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (service_id) REFERENCES mcp_services(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_mcp_oauth_clients_tenant_svc ON mcp_oauth_clients(tenant_id, service_id);
CREATE INDEX IF NOT EXISTS idx_mcp_oauth_clients_service_id ON mcp_oauth_clients(service_id);

CREATE TABLE IF NOT EXISTS mcp_oauth_tokens (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    user_id VARCHAR(64) NOT NULL,
    service_id VARCHAR(36) NOT NULL,
    access_token TEXT,
    refresh_token TEXT,
    token_type VARCHAR(32),
    expires_at DATETIME,
    refresh_lease_id VARCHAR(36),
    refresh_lease_until DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (service_id) REFERENCES mcp_services(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_mcp_oauth_tokens_tenant_user_svc ON mcp_oauth_tokens(tenant_id, user_id, service_id);
CREATE INDEX IF NOT EXISTS idx_mcp_oauth_tokens_service_id ON mcp_oauth_tokens(service_id);
CREATE INDEX IF NOT EXISTS idx_mcp_oauth_tokens_user_id ON mcp_oauth_tokens(user_id);

CREATE TABLE IF NOT EXISTS custom_agents (
    id VARCHAR(36) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    avatar VARCHAR(64),
    is_builtin BOOLEAN NOT NULL DEFAULT 0,
    tenant_id INTEGER NOT NULL,
    created_by VARCHAR(36),
    runnable_by_viewer BOOLEAN NOT NULL DEFAULT 1,
    config TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    PRIMARY KEY (id, tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_custom_agents_tenant_id ON custom_agents(tenant_id);
CREATE INDEX IF NOT EXISTS idx_custom_agents_is_builtin ON custom_agents(is_builtin);
CREATE INDEX IF NOT EXISTS idx_custom_agents_deleted_at ON custom_agents(deleted_at);

CREATE TABLE IF NOT EXISTS tenant_sandbox_configs (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    sandbox_type VARCHAR(32) NOT NULL,
    config TEXT NOT NULL DEFAULT '{}',
    cordoned_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_tenant_sandbox_configs_tenant_id ON tenant_sandbox_configs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenant_sandbox_configs_deleted_at ON tenant_sandbox_configs(deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS uq_tenant_sandbox_configs_tenant_name
    ON tenant_sandbox_configs(tenant_id, name) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS im_channel_sessions (
    id VARCHAR(36) PRIMARY KEY,
    platform VARCHAR(20) NOT NULL,
    user_id VARCHAR(128) NOT NULL,
    chat_id VARCHAR(128) NOT NULL DEFAULT '',
    session_id VARCHAR(36) NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    tenant_id INTEGER NOT NULL,
    agent_id VARCHAR(36) DEFAULT '',
    im_channel_id VARCHAR(36) DEFAULT '',
    thread_id VARCHAR(128) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    metadata TEXT DEFAULT '{}',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_channel_lookup
    ON im_channel_sessions (platform, user_id, chat_id, tenant_id)
    WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_channel_thread_lookup
    ON im_channel_sessions (platform, chat_id, thread_id, tenant_id)
    WHERE deleted_at IS NULL AND thread_id != '';
CREATE INDEX IF NOT EXISTS idx_im_channel_tenant ON im_channel_sessions (tenant_id);
CREATE INDEX IF NOT EXISTS idx_im_channel_session ON im_channel_sessions (session_id);
CREATE INDEX IF NOT EXISTS idx_im_channel_sessions_channel ON im_channel_sessions (im_channel_id)
    WHERE im_channel_id != '';

CREATE TABLE IF NOT EXISTS im_channels (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    agent_id VARCHAR(36) NOT NULL,
    platform VARCHAR(20) NOT NULL,
    name VARCHAR(255) NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    mode VARCHAR(20) NOT NULL DEFAULT 'websocket',
    output_mode VARCHAR(20) NOT NULL DEFAULT 'stream',
    credentials TEXT NOT NULL DEFAULT '{}',
    knowledge_base_id VARCHAR(36) DEFAULT '',
    bot_identity VARCHAR(255) NOT NULL DEFAULT '',
    session_mode VARCHAR(20) NOT NULL DEFAULT 'user',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_im_channels_tenant ON im_channels (tenant_id);
CREATE INDEX IF NOT EXISTS idx_im_channels_agent ON im_channels (agent_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_im_channels_bot_identity
    ON im_channels (bot_identity)
    WHERE deleted_at IS NULL AND bot_identity != '';

CREATE TABLE IF NOT EXISTS embed_channels (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    agent_id VARCHAR(36) NOT NULL DEFAULT 'builtin-quick-answer',
    name VARCHAR(255) NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    publish_token VARCHAR(64) NOT NULL DEFAULT '',
    allowed_origins TEXT NOT NULL DEFAULT '[]',
    welcome_message TEXT NOT NULL DEFAULT '',
    rate_limit_per_minute INTEGER NOT NULL DEFAULT 30,
    rate_limit_per_day INTEGER NOT NULL DEFAULT 10000,
    primary_color VARCHAR(32) NOT NULL DEFAULT '',
    page_title VARCHAR(255) NOT NULL DEFAULT '',
    header_title_mode VARCHAR(32) NOT NULL DEFAULT 'channel',
    show_suggested_questions INTEGER NOT NULL DEFAULT 1,
    widget_position VARCHAR(32) NOT NULL DEFAULT 'bottom-right',
    allow_web_search INTEGER NOT NULL DEFAULT 0,
    allow_file_upload INTEGER NOT NULL DEFAULT 0,
    default_locale VARCHAR(16) NOT NULL DEFAULT '',
    webhook_url VARCHAR(512) NOT NULL DEFAULT '',
    webhook_secret VARCHAR(128) NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_embed_channels_tenant ON embed_channels (tenant_id);
CREATE INDEX IF NOT EXISTS idx_embed_channels_agent ON embed_channels (agent_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_embed_channels_publish_token
    ON embed_channels (publish_token)
    WHERE publish_token != '' AND deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS data_sources (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    config TEXT,
    sync_schedule VARCHAR(100),
    sync_mode VARCHAR(20) DEFAULT 'incremental',
    status VARCHAR(32) DEFAULT 'active',
    conflict_strategy VARCHAR(32) DEFAULT 'overwrite',
    sync_deletions INTEGER DEFAULT 1,
    last_sync_at DATETIME NULL,
    last_sync_cursor TEXT,
    last_sync_result TEXT,
    error_message TEXT,
    sync_log_retention_days INTEGER DEFAULT 30,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME NULL
);

CREATE INDEX IF NOT EXISTS idx_data_sources_tenant_id ON data_sources (tenant_id);
CREATE INDEX IF NOT EXISTS idx_data_sources_knowledge_base_id ON data_sources (knowledge_base_id);
CREATE INDEX IF NOT EXISTS idx_data_sources_type ON data_sources (type);
CREATE INDEX IF NOT EXISTS idx_data_sources_status ON data_sources (status);
CREATE INDEX IF NOT EXISTS idx_data_sources_deleted_at ON data_sources (deleted_at);

CREATE TABLE IF NOT EXISTS sync_logs (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    data_source_id VARCHAR(36) NOT NULL REFERENCES data_sources(id) ON DELETE CASCADE,
    tenant_id INTEGER NOT NULL,
    status VARCHAR(32) NOT NULL,
    started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME NULL,
    items_total INTEGER DEFAULT 0,
    items_created INTEGER DEFAULT 0,
    items_updated INTEGER DEFAULT 0,
    items_deleted INTEGER DEFAULT 0,
    items_skipped INTEGER DEFAULT 0,
    items_failed INTEGER DEFAULT 0,
    error_message TEXT,
    result TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sync_logs_data_source_id ON sync_logs (data_source_id);
CREATE INDEX IF NOT EXISTS idx_sync_logs_tenant_id ON sync_logs (tenant_id);
CREATE INDEX IF NOT EXISTS idx_sync_logs_status ON sync_logs (status);
CREATE INDEX IF NOT EXISTS idx_sync_logs_started_at ON sync_logs (started_at);

CREATE TABLE IF NOT EXISTS web_search_providers (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    description TEXT,
    parameters TEXT,
    is_default INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME NULL
);

CREATE INDEX IF NOT EXISTS idx_web_search_providers_tenant_id ON web_search_providers (tenant_id);
CREATE INDEX IF NOT EXISTS idx_web_search_providers_provider ON web_search_providers (provider);
CREATE INDEX IF NOT EXISTS idx_web_search_providers_deleted_at ON web_search_providers (deleted_at);

CREATE TABLE IF NOT EXISTS vector_stores (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    engine_type VARCHAR(50) NOT NULL,
    connection_config TEXT NOT NULL DEFAULT '{}',
    index_config TEXT NOT NULL DEFAULT '{}',
    tenant_id INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_vector_stores_name_tenant
    ON vector_stores(name, tenant_id)
    WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_vector_stores_tenant_id ON vector_stores(tenant_id);
CREATE INDEX IF NOT EXISTS idx_vector_stores_engine_type ON vector_stores(engine_type);
CREATE INDEX IF NOT EXISTS idx_vector_stores_deleted_at ON vector_stores(deleted_at);

CREATE TABLE IF NOT EXISTS storage_backends (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    provider VARCHAR(32) NOT NULL,
    config TEXT NOT NULL DEFAULT '{}',
    source VARCHAR(16) NOT NULL DEFAULT 'user',
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    legacy_alias BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_storage_backends_name_tenant
    ON storage_backends(tenant_id, name) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_storage_backends_legacy_alias
    ON storage_backends(tenant_id, provider) WHERE deleted_at IS NULL AND legacy_alias = 1;
CREATE INDEX IF NOT EXISTS idx_storage_backends_tenant ON storage_backends(tenant_id);

CREATE TABLE IF NOT EXISTS resources (
    id VARCHAR(36) PRIMARY KEY,
    handle VARCHAR(22) NOT NULL UNIQUE,
    tenant_id INTEGER NOT NULL,
    storage_backend_id VARCHAR(36),
    provider VARCHAR(32) NOT NULL,
    physical_path TEXT NOT NULL,
    location_hash VARCHAR(64) NOT NULL,
    kind VARCHAR(32) NOT NULL DEFAULT 'file',
    mime_type VARCHAR(255) NOT NULL DEFAULT '',
    original_name VARCHAR(1024) NOT NULL DEFAULT '',
    size INTEGER NOT NULL DEFAULT 0,
    content_hash VARCHAR(64) NOT NULL DEFAULT '',
    lifecycle VARCHAR(16) NOT NULL DEFAULT 'persistent',
    expires_at DATETIME,
    state VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_resources_tenant_location
    ON resources(tenant_id, location_hash) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_resources_tenant ON resources(tenant_id);
CREATE INDEX IF NOT EXISTS idx_resources_backend ON resources(storage_backend_id);

CREATE TABLE IF NOT EXISTS resource_bindings (
    id VARCHAR(36) PRIMARY KEY,
    resource_id VARCHAR(36) NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
    tenant_id INTEGER NOT NULL,
    owner_type VARCHAR(32) NOT NULL,
    owner_id VARCHAR(64) NOT NULL,
    relation VARCHAR(32) NOT NULL DEFAULT 'attachment',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_resource_bindings_unique
    ON resource_bindings(resource_id, owner_type, owner_id, relation);
CREATE INDEX IF NOT EXISTS idx_resource_bindings_owner
    ON resource_bindings(tenant_id, owner_type, owner_id);

CREATE TABLE IF NOT EXISTS tenant_api_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER,
    scope_type TEXT NOT NULL DEFAULT 'tenant'
        CHECK (scope_type IN ('tenant', 'platform')),
    name TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    api_key TEXT NOT NULL DEFAULT '',
    full_access BOOLEAN NOT NULL DEFAULT 0,
    knowledge_base_ids TEXT NOT NULL DEFAULT '[]',
    capabilities TEXT NOT NULL DEFAULT '[]',
    last_used_at DATETIME,
    expires_at DATETIME,
    revoked_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    CHECK (
        (scope_type = 'tenant' AND tenant_id IS NOT NULL)
        OR (scope_type = 'platform' AND tenant_id IS NULL AND full_access = 0)
    )
);

CREATE INDEX IF NOT EXISTS idx_tenant_api_keys_tenant ON tenant_api_keys(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenant_api_keys_revoked_at ON tenant_api_keys(revoked_at);
CREATE INDEX IF NOT EXISTS idx_tenant_api_keys_scope_type ON tenant_api_keys(scope_type);

CREATE TABLE IF NOT EXISTS temporary_documents (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    resource_ref TEXT NOT NULL,
    file_name VARCHAR(1024) NOT NULL,
    file_type VARCHAR(32) NOT NULL,
    mime_type VARCHAR(255) NOT NULL DEFAULT '',
    file_size INTEGER NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'uploaded',
    content TEXT NOT NULL DEFAULT '',
    chunks TEXT NOT NULL DEFAULT '[]',
    image_refs TEXT NOT NULL DEFAULT '[]',
    metadata TEXT NOT NULL DEFAULT '{}',
    processing_options TEXT NOT NULL DEFAULT '{}',
    token_count INTEGER NOT NULL DEFAULT 0,
    chunk_count INTEGER NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    expires_at DATETIME NOT NULL,
    started_at DATETIME,
    ready_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_temporary_documents_scope ON temporary_documents(tenant_id, session_id);
CREATE INDEX IF NOT EXISTS idx_temporary_documents_status ON temporary_documents(status);
CREATE INDEX IF NOT EXISTS idx_temporary_documents_expires ON temporary_documents(expires_at);

-- ---------------------------------------------------------------------------
-- Gaze (consolidated from Postgres migrations 000037, 000040, 000061, 000075)
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS gaze_pages (
    id                VARCHAR(36) PRIMARY KEY,
    tenant_id         INTEGER NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    slug              VARCHAR(255) NOT NULL,
    title             VARCHAR(512) NOT NULL DEFAULT '',
    page_type         VARCHAR(32) NOT NULL DEFAULT 'summary',
    status            VARCHAR(32) NOT NULL DEFAULT 'published',
    content           TEXT NOT NULL DEFAULT '',
    summary           TEXT NOT NULL DEFAULT '',
    parent_slug       VARCHAR(255) NOT NULL DEFAULT '',
    folder_id         VARCHAR(36) NOT NULL DEFAULT '',
    category_path     TEXT DEFAULT '[]',
    gaze_path         VARCHAR(1024) NOT NULL DEFAULT '',
    depth             INTEGER NOT NULL DEFAULT 0,
    sort_order        INTEGER NOT NULL DEFAULT 0,
    source_refs       TEXT DEFAULT '[]',
    chunk_refs        TEXT DEFAULT '[]',
    in_links          TEXT DEFAULT '[]',
    out_links         TEXT DEFAULT '[]',
    page_metadata     TEXT DEFAULT '{}',
    aliases           TEXT DEFAULT '[]',
    version           INTEGER NOT NULL DEFAULT 1,
    last_edit_source  VARCHAR(16) NOT NULL DEFAULT '',
    last_editor_id    VARCHAR(64) NOT NULL DEFAULT '',
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at        DATETIME
);

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

CREATE TABLE IF NOT EXISTS gaze_folders (
    id                VARCHAR(36) PRIMARY KEY,
    tenant_id         INTEGER NOT NULL DEFAULT 0,
    knowledge_base_id VARCHAR(36) NOT NULL,
    parent_id         VARCHAR(36) NOT NULL DEFAULT '',
    name              VARCHAR(255) NOT NULL,
    path              VARCHAR(1024) NOT NULL DEFAULT '',
    depth             INTEGER NOT NULL DEFAULT 0,
    sort_order        INTEGER NOT NULL DEFAULT 0,
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at        DATETIME
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_gaze_folders_parent_name
    ON gaze_folders (knowledge_base_id, parent_id, name)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_gaze_folders_parent
    ON gaze_folders (knowledge_base_id, parent_id);

CREATE INDEX IF NOT EXISTS idx_gaze_folders_deleted_at
    ON gaze_folders (deleted_at);

CREATE TABLE IF NOT EXISTS gaze_page_issues (
    id                    VARCHAR(36) PRIMARY KEY,
    tenant_id             INTEGER NOT NULL,
    knowledge_base_id     VARCHAR(36) NOT NULL,
    slug                  VARCHAR(255) NOT NULL,
    issue_type            VARCHAR(50) NOT NULL,
    description           TEXT NOT NULL,
    suspected_knowledge_ids TEXT,
    status                VARCHAR(20) NOT NULL DEFAULT 'pending',
    reported_by           VARCHAR(100) NOT NULL,
    created_at            DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at            DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at            DATETIME
);

CREATE INDEX IF NOT EXISTS idx_gaze_page_issues_tenant_id
    ON gaze_page_issues(tenant_id);

CREATE INDEX IF NOT EXISTS idx_gaze_page_issues_knowledge_base_id
    ON gaze_page_issues(knowledge_base_id);

CREATE INDEX IF NOT EXISTS idx_gaze_page_issues_slug
    ON gaze_page_issues(slug);

CREATE INDEX IF NOT EXISTS idx_gaze_page_issues_status
    ON gaze_page_issues(status);

CREATE TABLE IF NOT EXISTS gaze_page_revisions (
    id                VARCHAR(36) PRIMARY KEY,
    tenant_id         INTEGER NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL,
    page_id           VARCHAR(36) NOT NULL,
    slug              VARCHAR(255) NOT NULL,
    version           INTEGER NOT NULL,
    title             VARCHAR(512) NOT NULL DEFAULT '',
    page_type         VARCHAR(32) NOT NULL DEFAULT 'summary',
    status            VARCHAR(32) NOT NULL DEFAULT 'published',
    content           TEXT NOT NULL DEFAULT '',
    summary           TEXT NOT NULL DEFAULT '',
    aliases           TEXT DEFAULT '[]',
    edit_source       VARCHAR(16) NOT NULL DEFAULT '',
    editor_id         VARCHAR(64) NOT NULL DEFAULT '',
    edited_at         DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_gaze_page_revisions_page_version
    ON gaze_page_revisions (page_id, version);

CREATE INDEX IF NOT EXISTS idx_gaze_page_revisions_kb_slug
    ON gaze_page_revisions (knowledge_base_id, slug);

-- ---- source: 000001_remove_gaze_log.up.sql ----
DROP TABLE IF EXISTS gaze_log_entries;

DELETE FROM gaze_pages WHERE page_type = 'log';

-- ---- source: 000002_knowledge_folder_path.up.sql ----
-- Folder tree support for knowledge bases (SQLite / lite mode).
--
-- Mirrors migrations/versioned/000079_knowledge_folder_path.up.sql.
ALTER TABLE knowledges ADD COLUMN folder_path VARCHAR(1024) NOT NULL DEFAULT '';

-- Backfill legacy folder uploads, which encoded their relative directory in
-- file_name. SQLite has no regex, so the directory prefix is isolated with the
-- rtrim trick: trimming every non-slash character off the right leaves exactly
-- the prefix up to and including the last '/'.
UPDATE knowledges
SET folder_path = substr(
        rtrim(file_name, replace(file_name, '/', '')),
        1,
        length(rtrim(file_name, replace(file_name, '/', ''))) - 1
    ),
    file_name = substr(
        file_name,
        length(rtrim(file_name, replace(file_name, '/', ''))) + 1
    )
WHERE file_name LIKE '%/%';

CREATE INDEX IF NOT EXISTS idx_knowledges_folder_path
    ON knowledges (tenant_id, knowledge_base_id, folder_path);

-- ---- source: 000003_knowledge_base_auto_tag_config.up.sql ----
ALTER TABLE knowledge_bases ADD COLUMN auto_tag_config TEXT;

-- ---- source: 000004_memory.up.sql ----
-- Cross-session long-term memory (Lite). Mirrors migrations/versioned/000084.
-- Row ids are generated in Go, so there is no server-side default here.

CREATE TABLE IF NOT EXISTS memory_subjects (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    subject_id VARCHAR(512) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    block_text TEXT NOT NULL DEFAULT '',
    block_updated_at DATETIME,
    item_count INTEGER NOT NULL DEFAULT 0,
    last_extracted_at DATETIME,
    extract_cursor DATETIME,
    pending_sessions TEXT,
    extract_scheduled_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_memory_subjects_scope
    ON memory_subjects (tenant_id, subject_id);

CREATE TABLE IF NOT EXISTS memory_items (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    subject_id VARCHAR(512) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    content TEXT NOT NULL,
    topic VARCHAR(255) NOT NULL DEFAULT '',
    normalized_key VARCHAR(255) NOT NULL DEFAULT '',
    importance INTEGER NOT NULL DEFAULT 3,
    origin VARCHAR(16) NOT NULL DEFAULT 'extracted',
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    source_session_id VARCHAR(36),
    source_message_id VARCHAR(36),
    valid_from DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    invalid_at DATETIME,
    expires_at DATETIME,
    superseded_by VARCHAR(36),
    last_used_at DATETIME,
    use_count INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_memory_items_scope
    ON memory_items (tenant_id, subject_id, status);
CREATE INDEX IF NOT EXISTS idx_memory_items_key
    ON memory_items (tenant_id, subject_id, normalized_key);

CREATE TABLE IF NOT EXISTS memory_tombstones (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    subject_id VARCHAR(512) NOT NULL,
    topic VARCHAR(255) NOT NULL DEFAULT '',
    fingerprint VARCHAR(64) NOT NULL,
    source_message_id VARCHAR(36),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_memory_tombstones_scope
    ON memory_tombstones (tenant_id, subject_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_mem_tomb_fp
    ON memory_tombstones (tenant_id, subject_id, fingerprint);

CREATE TABLE IF NOT EXISTS memory_topic_stats (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    subject_id VARCHAR(512) NOT NULL,
    normalized_key VARCHAR(255) NOT NULL,
    topic VARCHAR(255) NOT NULL DEFAULT '',
    hits INTEGER NOT NULL DEFAULT 0,
    last_seen_at DATETIME,
    promoted_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_mem_topic_scope
    ON memory_topic_stats (tenant_id, subject_id, normalized_key);

CREATE TABLE IF NOT EXISTS memory_doc_affinity (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    subject_id VARCHAR(512) NOT NULL,
    knowledge_id VARCHAR(36) NOT NULL,
    knowledge_base_id VARCHAR(36) NOT NULL DEFAULT '',
    title VARCHAR(512) NOT NULL DEFAULT '',
    hits INTEGER NOT NULL DEFAULT 0,
    last_used_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_mem_affinity_scope
    ON memory_doc_affinity (tenant_id, subject_id, knowledge_id);

ALTER TABLE tenants ADD COLUMN memory_config TEXT;
ALTER TABLE messages ADD COLUMN used_memories TEXT;

ALTER TABLE memory_subjects ADD COLUMN consolidated_at DATETIME;
ALTER TABLE memory_subjects ADD COLUMN forced_consolidated_at DATETIME;

ALTER TABLE memory_topic_stats ADD COLUMN aliases TEXT NOT NULL DEFAULT '[]';

CREATE TABLE IF NOT EXISTS memory_item_embeddings (
    item_id VARCHAR(36) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    subject_id VARCHAR(512) NOT NULL,
    model_id VARCHAR(64) NOT NULL DEFAULT '',
    dims INTEGER NOT NULL DEFAULT 0,
    vector BLOB,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_mem_emb_scope
    ON memory_item_embeddings (tenant_id, subject_id);

-- ---- source: 000005_messages_attachments.up.sql ----
-- Mirrors versioned migration 000034_add_attachments.

ALTER TABLE messages ADD COLUMN attachments TEXT NOT NULL DEFAULT '[]';

-- ---- source: 000006_task_queue_and_dead_letters.up.sql ----
-- Mirrors versioned migration 000041_task_queue_and_gaze_indexes:
-- durable pending-op queue and dead-letter archive for SQLite.

CREATE TABLE IF NOT EXISTS task_pending_ops (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id   INTEGER NOT NULL,
    task_type   VARCHAR(64) NOT NULL,
    scope       VARCHAR(32) NOT NULL,
    scope_id    VARCHAR(64) NOT NULL,
    op          VARCHAR(32) NOT NULL,
    dedup_key   VARCHAR(128) NOT NULL DEFAULT '',
    payload     TEXT NOT NULL DEFAULT '{}',
    fail_count  INTEGER NOT NULL DEFAULT 0,
    enqueued_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    claimed_at  DATETIME
);

CREATE INDEX IF NOT EXISTS idx_task_pending_ops_scope
    ON task_pending_ops (task_type, scope, scope_id, id);

CREATE INDEX IF NOT EXISTS idx_task_pending_ops_tenant
    ON task_pending_ops (tenant_id);

CREATE TABLE IF NOT EXISTS task_dead_letters (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id   INTEGER NOT NULL,
    task_type   VARCHAR(64) NOT NULL,
    scope       VARCHAR(32) NOT NULL,
    scope_id    VARCHAR(64) NOT NULL,
    related_id  VARCHAR(64) NOT NULL DEFAULT '',
    payload     TEXT NOT NULL,
    last_error  TEXT NOT NULL DEFAULT '',
    fail_count  INTEGER NOT NULL,
    failed_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_task_dead_letters_scope
    ON task_dead_letters (scope, scope_id, failed_at DESC);

CREATE INDEX IF NOT EXISTS idx_task_dead_letters_tenant
    ON task_dead_letters (tenant_id, failed_at DESC);

CREATE INDEX IF NOT EXISTS idx_task_dead_letters_task_type
    ON task_dead_letters (task_type, failed_at DESC);

-- ---- source: 000007_system_admin_and_settings.up.sql ----
-- Mirrors versioned migration 000053_system_admin_and_settings:
-- users.is_system_admin + system_settings table.

ALTER TABLE users ADD COLUMN is_system_admin BOOLEAN NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_users_is_system_admin ON users (is_system_admin);

CREATE TABLE IF NOT EXISTS system_settings (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    key              VARCHAR(128) NOT NULL UNIQUE,
    value            TEXT NOT NULL,
    value_type       VARCHAR(16)  NOT NULL,
    category         VARCHAR(32)  NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    is_secret        BOOLEAN NOT NULL DEFAULT 0,
    requires_restart BOOLEAN NOT NULL DEFAULT 0,
    last_modified_by VARCHAR(36) NOT NULL DEFAULT '',
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_system_settings_category
    ON system_settings (category);

-- ---- source: 000008_processing_spans_and_pending_subtasks.up.sql ----
-- Mirrors versioned migrations:
--   000055_knowledge_processing_spans   knowledge_processing_spans table
--   000056_knowledge_pending_subtasks   knowledges.pending_subtasks_count
--   000066_expand_knowledge_span_name   name VARCHAR(255)

CREATE TABLE IF NOT EXISTS knowledge_processing_spans (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    knowledge_id    VARCHAR(64) NOT NULL,
    attempt         INTEGER NOT NULL DEFAULT 1,
    span_id         VARCHAR(64) NOT NULL,
    parent_span_id  VARCHAR(64),
    name            VARCHAR(255) NOT NULL,
    kind            VARCHAR(16) NOT NULL,
    status          VARCHAR(16) NOT NULL,
    input           TEXT,
    output          TEXT,
    metadata        TEXT,
    error_code      VARCHAR(64),
    error_message   TEXT,
    error_detail    TEXT,
    started_at      DATETIME,
    finished_at     DATETIME,
    duration_ms     INTEGER,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_kpspan_attempt_span UNIQUE (knowledge_id, attempt, span_id)
);

CREATE INDEX IF NOT EXISTS idx_kpspan_knowledge_attempt
    ON knowledge_processing_spans (knowledge_id, attempt);

CREATE INDEX IF NOT EXISTS idx_kpspan_status_started
    ON knowledge_processing_spans (status, started_at);

CREATE INDEX IF NOT EXISTS idx_kpspan_parent
    ON knowledge_processing_spans (parent_span_id)
    WHERE parent_span_id IS NOT NULL;

ALTER TABLE knowledges ADD COLUMN pending_subtasks_count INTEGER NOT NULL DEFAULT 0;

-- ---- source: 000009_embed_channel_memory_flag.up.sql ----
-- Mirrors versioned migration 000060_embed_channels: embed_channels.allow_memory.

ALTER TABLE embed_channels ADD COLUMN allow_memory BOOLEAN NOT NULL DEFAULT 0;

-- ---- source: 000010_knowledge_multi_tags.up.sql ----
-- Mirrors versioned migration 000063_knowledge_multi_tags:
-- many-to-many join table between knowledges and knowledge_tags.

CREATE TABLE IF NOT EXISTS knowledge_tag_relations (
    knowledge_id VARCHAR(36) NOT NULL,
    tag_id       VARCHAR(36) NOT NULL,
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (knowledge_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_ktr_knowledge
    ON knowledge_tag_relations (knowledge_id);

CREATE INDEX IF NOT EXISTS idx_ktr_tag
    ON knowledge_tag_relations (tag_id);

-- Migrate existing single-tag data into the join table.
INSERT INTO knowledge_tag_relations (knowledge_id, tag_id, created_at)
SELECT id, tag_id, updated_at
FROM knowledges
WHERE tag_id IS NOT NULL AND tag_id != ''
  AND deleted_at IS NULL;

DROP INDEX IF EXISTS idx_knowledges_tag;
ALTER TABLE knowledges DROP COLUMN tag_id;

-- ---- source: 000011_principal_model.up.sql ----
-- Mirrors versioned migration 000064_principal_model:
-- principal identity for MCP OAuth tokens and tenant API config.

ALTER TABLE tenants ADD COLUMN api_principal_config TEXT;

ALTER TABLE mcp_oauth_tokens ADD COLUMN principal_type VARCHAR(32);
ALTER TABLE mcp_oauth_tokens ADD COLUMN principal_id VARCHAR(512);

UPDATE mcp_oauth_tokens
SET principal_type = 'web_user',
    principal_id = user_id
WHERE (principal_type IS NULL OR principal_type = '')
  AND user_id IS NOT NULL
  AND user_id <> '';

DROP INDEX IF EXISTS idx_mcp_oauth_tokens_tenant_user_svc;

CREATE UNIQUE INDEX IF NOT EXISTS idx_mcp_oauth_tokens_tenant_principal_svc
    ON mcp_oauth_tokens(tenant_id, principal_type, principal_id, service_id);

CREATE INDEX IF NOT EXISTS idx_mcp_oauth_tokens_principal
    ON mcp_oauth_tokens(principal_type, principal_id);

-- ---- source: 000012_message_usage.up.sql ----
-- Mirrors versioned migration 000085_message_usage:
-- per-turn LLM token usage persisted with the assistant message.

ALTER TABLE messages ADD COLUMN usage TEXT;

-- ---- source: 000013_mcp_tool_enabled.up.sql ----
-- Per-tool MCP enable/disable policy. Missing rows are treated as enabled.
ALTER TABLE mcp_tool_approvals ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT 1;

-- ---- source: 000014_embeddings_bytea.up.sql ----
-- Migration: lite_embeddings BYTEA column (native vector store retirement, §2.15)
-- SQLite Lite stores vectors as float32 little-endian bytes in the embedding BLOB.
-- The table is also auto-migrated by the runtime retriever; create it if this
-- migration runs against a fresh database before that runtime path exists.
CREATE TABLE IF NOT EXISTS lite_embeddings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at DATETIME,
    updated_at DATETIME,
    source_id TEXT NOT NULL,
    source_type INTEGER NOT NULL,
    chunk_id TEXT,
    knowledge_id TEXT,
    knowledge_base_id TEXT,
    tag_id TEXT,
    content TEXT NOT NULL,
    dimension INTEGER NOT NULL,
    embedding BLOB,
    is_enabled BOOLEAN DEFAULT TRUE
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sqlite_emb_source
    ON lite_embeddings (source_id, source_type);
CREATE INDEX IF NOT EXISTS idx_lite_embeddings_chunk_id ON lite_embeddings (chunk_id);
CREATE INDEX IF NOT EXISTS idx_lite_embeddings_knowledge_id ON lite_embeddings (knowledge_id);
CREATE INDEX IF NOT EXISTS idx_lite_embeddings_knowledge_base_id ON lite_embeddings (knowledge_base_id);
CREATE INDEX IF NOT EXISTS idx_lite_embeddings_tag_id ON lite_embeddings (tag_id);
CREATE INDEX IF NOT EXISTS idx_lite_embeddings_is_enabled ON lite_embeddings (is_enabled);

-- ---- source: 000015_message_query_rewritten.up.sql ----
-- Migration: user 消息记录指代消解后的实际检索 query（需求五）
ALTER TABLE messages ADD COLUMN query_rewritten TEXT NOT NULL DEFAULT '';

-- ---- source: 000016_message_query_rewrite_meta.up.sql ----
-- Migration: user 消息记录指代消解的展开信息（需求五 UI 返工）
ALTER TABLE messages ADD COLUMN query_rewrite_meta TEXT;

-- ---- source: 000017_collaboration.up.sql ----
-- Migration 000017: document uploader and engine sync state (SQLite mirror).

ALTER TABLE knowledges ADD COLUMN created_by VARCHAR(36);
ALTER TABLE knowledges ADD COLUMN engine_sync_status VARCHAR(20) NOT NULL DEFAULT 'synced';
ALTER TABLE knowledges ADD COLUMN engine_sync_requested_at DATETIME;
ALTER TABLE knowledges ADD COLUMN engine_sync_completed_at DATETIME;
ALTER TABLE knowledges ADD COLUMN engine_sync_error TEXT NOT NULL DEFAULT '';

UPDATE knowledges
SET created_by = (
    SELECT kb.creator_id FROM knowledge_bases kb
    WHERE kb.id = knowledges.knowledge_base_id
)
WHERE created_by IS NULL;

CREATE INDEX IF NOT EXISTS idx_knowledges_created_by ON knowledges(created_by);
CREATE INDEX IF NOT EXISTS idx_knowledges_engine_sync ON knowledges(tenant_id, knowledge_base_id, engine_sync_status);

ALTER TABLE tenants ADD COLUMN engine_sync_config TEXT;

-- ---- source: 000018_collaboration_v2.up.sql ----
-- Migration 000018: document versions, chunk snapshots and citation counters.

CREATE TABLE IF NOT EXISTS doc_versions (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    knowledge_base_id TEXT NOT NULL,
    knowledge_id TEXT NOT NULL,
    version_seq INTEGER NOT NULL,
    version_label TEXT NOT NULL,
    source_type TEXT NOT NULL,
    source_id TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    chunk_count INTEGER NOT NULL DEFAULT 0,
    created_by TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_doc_versions_knowledge_seq ON doc_versions(knowledge_id, version_seq);
CREATE INDEX IF NOT EXISTS idx_doc_versions_kb ON doc_versions(tenant_id, knowledge_base_id, knowledge_id);

CREATE TABLE IF NOT EXISTS doc_version_chunks (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    version_id TEXT NOT NULL,
    chunk_id TEXT NOT NULL,
    chunk_index INTEGER NOT NULL DEFAULT 0,
    content TEXT NOT NULL DEFAULT '',
    is_enabled INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_doc_version_chunks_version ON doc_version_chunks(version_id, chunk_index);
CREATE INDEX IF NOT EXISTS idx_doc_version_chunks_chunk ON doc_version_chunks(chunk_id);

CREATE TABLE IF NOT EXISTS chunk_citation_daily (
    chunk_id TEXT NOT NULL,
    tenant_id INTEGER NOT NULL,
    stat_date TEXT NOT NULL,
    citation_count INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (chunk_id, stat_date)
);
CREATE INDEX IF NOT EXISTS idx_chunk_citation_daily_tenant ON chunk_citation_daily(tenant_id, stat_date);

-- ---- source: 000021_knowledge_folders.up.sql ----
-- SQLite mirror of 000109_knowledge_folders.

CREATE TABLE IF NOT EXISTS knowledge_folders (
    id          TEXT PRIMARY KEY,
    tenant_id   INTEGER NOT NULL,
    kb_id       TEXT NOT NULL,
    parent_id   TEXT NOT NULL DEFAULT '',
    name        TEXT NOT NULL,
    path        TEXT NOT NULL DEFAULT '',
    depth       INTEGER NOT NULL DEFAULT 0,
    is_root     INTEGER NOT NULL DEFAULT 0,
    created_by  TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at  DATETIME
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_knowledge_folders_kb_path
    ON knowledge_folders (kb_id, path) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_knowledge_folders_sibling
    ON knowledge_folders (kb_id, parent_id, name)
    WHERE deleted_at IS NULL AND is_root = 0;
CREATE UNIQUE INDEX IF NOT EXISTS idx_knowledge_folders_root
    ON knowledge_folders (kb_id) WHERE deleted_at IS NULL AND is_root = 1;
CREATE INDEX IF NOT EXISTS idx_knowledge_folders_kb
    ON knowledge_folders (kb_id) WHERE deleted_at IS NULL;

INSERT OR IGNORE INTO knowledge_folders (id, tenant_id, kb_id, parent_id, name, path, depth, is_root, created_by)
SELECT 'folder-root-' || kb.id, kb.tenant_id, kb.id, '', '', '', 0, 1, kb.creator_id
FROM knowledge_bases kb
WHERE kb.deleted_at IS NULL;

-- Materialize every prefix of every existing folder_path.
WITH RECURSIVE
paths(tenant_id, kb_id, folder_path) AS (
    SELECT DISTINCT tenant_id, knowledge_base_id, folder_path
    FROM knowledges
    WHERE deleted_at IS NULL AND folder_path <> ''
),
prefixes(tenant_id, kb_id, path, prefix, parent_prefix, rest, depth) AS (
    SELECT tenant_id, kb_id, folder_path, '', '', folder_path, 0
    FROM paths
    UNION ALL
    SELECT
        tenant_id,
        kb_id,
        path,
        CASE
            WHEN prefix = '' THEN
                CASE WHEN instr(rest, '/') > 0 THEN substr(rest, 1, instr(rest, '/') - 1) ELSE rest END
            ELSE
                prefix || '/' || CASE WHEN instr(rest, '/') > 0 THEN substr(rest, 1, instr(rest, '/') - 1) ELSE rest END
        END,
        prefix,
        CASE WHEN instr(rest, '/') > 0 THEN substr(rest, instr(rest, '/') + 1) ELSE '' END,
        depth + 1
    FROM prefixes
    WHERE rest <> ''
)
INSERT OR IGNORE INTO knowledge_folders (id, tenant_id, kb_id, parent_id, name, path, depth, is_root, created_by)
SELECT
    'fd-' || lower(hex(kb_id || ':' || prefix)),
    tenant_id,
    kb_id,
    CASE
        WHEN parent_prefix = '' THEN 'folder-root-' || kb_id
        ELSE 'fd-' || lower(hex(kb_id || ':' || parent_prefix))
    END,
    CASE
        WHEN parent_prefix = '' THEN prefix
        ELSE substr(prefix, length(parent_prefix) + 2)
    END,
    prefix,
    depth,
    0,
    ''
FROM prefixes
WHERE prefix <> '';

ALTER TABLE knowledges ADD COLUMN folder_id TEXT NOT NULL DEFAULT '';

UPDATE knowledges
SET folder_id = COALESCE(
    (
        SELECT f.id
        FROM knowledge_folders f
        WHERE f.kb_id = knowledges.knowledge_base_id
          AND f.path = knowledges.folder_path
          AND f.deleted_at IS NULL
        LIMIT 1
    ),
    'folder-root-' || knowledge_base_id
);

CREATE INDEX IF NOT EXISTS idx_knowledges_folder_id
    ON knowledges (tenant_id, knowledge_base_id, folder_id);

-- ---- source: 000024_retrieval_audit.up.sql ----
-- SQLite mirror of 000112_retrieval_audit.

CREATE TABLE IF NOT EXISTS retrieval_audit_logs (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id        INTEGER NOT NULL,
    user_id          TEXT NOT NULL DEFAULT '',
    principal_kind   TEXT NOT NULL DEFAULT 'member',
    session_id       TEXT NOT NULL DEFAULT '',
    query_digest     TEXT NOT NULL DEFAULT '',
    kb_ids           TEXT NOT NULL DEFAULT '',
    scope_mode       TEXT NOT NULL DEFAULT '',
    scope_version    INTEGER NOT NULL DEFAULT 0,
    scope_doc_count  INTEGER NOT NULL DEFAULT 0,
    hit_doc_ids      TEXT NOT NULL DEFAULT '',
    decision         TEXT NOT NULL DEFAULT '',
    error            TEXT NOT NULL DEFAULT '',
    created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_retrieval_audit_tenant_time
    ON retrieval_audit_logs (tenant_id, created_at);
CREATE INDEX IF NOT EXISTS idx_retrieval_audit_user_time
    ON retrieval_audit_logs (tenant_id, user_id, created_at);

-- ---- source: 000025_skill_injection.up.sql ----
-- SQLite mirror of 000113_skill_injection.

CREATE TABLE IF NOT EXISTS tenant_skill_catalog (
    id            TEXT PRIMARY KEY,
    tenant_id     INTEGER NOT NULL,
    name          TEXT NOT NULL,
    version       TEXT,
    description   TEXT,
    instructions  TEXT,
    bundle_ref    TEXT,
    bundle_sha256 TEXT,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at    DATETIME
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_tenant_skill_catalog_name
    ON tenant_skill_catalog (tenant_id, name) WHERE deleted_at IS NULL;

ALTER TABLE tenant_skill_catalog ADD COLUMN enabled INTEGER NOT NULL DEFAULT 1;
ALTER TABLE messages ADD COLUMN skill TEXT;

-- ---- source: 000026_reading_runs.up.sql ----
-- SQLite mirror of 000114_reading_runs.

ALTER TABLE tenants ADD COLUMN reading_model_id TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS reading_runs (
    id                 TEXT PRIMARY KEY,
    tenant_id          INTEGER NOT NULL,
    session_id         TEXT NOT NULL DEFAULT '',
    user_id            TEXT NOT NULL DEFAULT '',
    message_id         TEXT NOT NULL DEFAULT '',
    document_ids       TEXT NOT NULL DEFAULT '[]',
    skill_id           TEXT NOT NULL DEFAULT '',
    reading_model_id   TEXT NOT NULL DEFAULT '',
    status             TEXT NOT NULL DEFAULT 'planning',
    plan               TEXT NOT NULL DEFAULT '{}',
    notes              TEXT NOT NULL DEFAULT '{}',
    final              TEXT NOT NULL DEFAULT '{}',
    current_section_id TEXT NOT NULL DEFAULT '',
    step_count         INTEGER NOT NULL DEFAULT 0,
    token_usage        INTEGER NOT NULL DEFAULT 0,
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_reading_runs_user
    ON reading_runs (tenant_id, user_id, session_id, created_at);
CREATE INDEX IF NOT EXISTS idx_reading_runs_active
    ON reading_runs (tenant_id, user_id, session_id, status);

CREATE TABLE IF NOT EXISTS reading_run_steps (
    id            TEXT PRIMARY KEY,
    run_id        TEXT NOT NULL,
    seq           INTEGER NOT NULL,
    action        TEXT NOT NULL,
    section_id    TEXT NOT NULL DEFAULT '',
    question      TEXT NOT NULL DEFAULT '{}',
    user_feedback TEXT NOT NULL DEFAULT '',
    notes_patch   TEXT NOT NULL DEFAULT '{}',
    citations     TEXT NOT NULL DEFAULT '[]',
    status        TEXT NOT NULL DEFAULT 'done',
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_reading_run_steps_run
    ON reading_run_steps (run_id, seq);

-- ---- source: 000027_message_reading_run.up.sql ----
-- SQLite mirror of 000115_message_reading_run.

ALTER TABLE messages ADD COLUMN reading_run_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_messages_reading_run
    ON messages (reading_run_id);

-- ---- source: 000029_retrieval_audit_created_at_index.up.sql ----
CREATE INDEX IF NOT EXISTS idx_retrieval_audit_created_at
    ON retrieval_audit_logs (created_at);

-- ---- source: 000032_session_reading_run.up.sql ----
ALTER TABLE sessions ADD COLUMN reading_run_id VARCHAR(36) NOT NULL DEFAULT '';

-- ---- source: 000033_reading_state_tables.up.sql ----
-- SQLite mirror of 000121_reading_state_tables.

CREATE TABLE IF NOT EXISTS reading_questions (
    id                 TEXT PRIMARY KEY,
    run_id             TEXT NOT NULL,
    tenant_id          INTEGER NOT NULL,
    parent_id          TEXT NOT NULL DEFAULT '',
    section_id         TEXT NOT NULL DEFAULT '',
    round              INTEGER NOT NULL DEFAULT 0,
    text               TEXT NOT NULL DEFAULT '',
    recommended_answer TEXT NOT NULL DEFAULT '',
    why                TEXT NOT NULL DEFAULT '',
    search_query       TEXT NOT NULL DEFAULT '',
    evidence           TEXT NOT NULL DEFAULT '[]',
    answer             TEXT NOT NULL DEFAULT '',
    answer_source      TEXT NOT NULL DEFAULT '',
    status             TEXT NOT NULL DEFAULT 'pending',
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_reading_questions_run
    ON reading_questions (run_id, round, created_at);
CREATE INDEX IF NOT EXISTS idx_reading_questions_status
    ON reading_questions (run_id, status);

CREATE TABLE IF NOT EXISTS reading_conclusions (
    id          TEXT PRIMARY KEY,
    run_id      TEXT NOT NULL,
    tenant_id   INTEGER NOT NULL,
    section_id  TEXT NOT NULL DEFAULT '',
    round       INTEGER NOT NULL DEFAULT 0,
    text        TEXT NOT NULL DEFAULT '',
    evidence    TEXT NOT NULL DEFAULT '[]',
    status      TEXT NOT NULL DEFAULT 'candidate',
    source      TEXT NOT NULL DEFAULT 'model',
    gates       TEXT NOT NULL DEFAULT '{}',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_reading_conclusions_run
    ON reading_conclusions (run_id, status, created_at);

CREATE TABLE IF NOT EXISTS reading_terms (
    id          TEXT PRIMARY KEY,
    run_id      TEXT NOT NULL,
    tenant_id   INTEGER NOT NULL,
    term        TEXT NOT NULL DEFAULT '',
    definition  TEXT NOT NULL DEFAULT '',
    source      TEXT NOT NULL DEFAULT '[]',
    canonical   INTEGER NOT NULL DEFAULT 0,
    status      TEXT NOT NULL DEFAULT 'candidate',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_reading_terms_run
    ON reading_terms (run_id, created_at);

CREATE TABLE IF NOT EXISTS reading_contradictions (
    id          TEXT PRIMARY KEY,
    run_id      TEXT NOT NULL,
    tenant_id   INTEGER NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    evidence_a  TEXT NOT NULL DEFAULT '[]',
    evidence_b  TEXT NOT NULL DEFAULT '[]',
    status      TEXT NOT NULL DEFAULT 'open',
    resolution  TEXT NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_reading_contradictions_run
    ON reading_contradictions (run_id, status, created_at);

-- ---- source: 000034_reading_report_drafts.up.sql ----
-- SQLite mirror of 000122_reading_report_drafts.

CREATE TABLE IF NOT EXISTS reading_report_drafts (
    run_id     TEXT PRIMARY KEY,
    tenant_id  INTEGER NOT NULL,
    user_id    TEXT NOT NULL,
    content_md TEXT NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_reading_report_drafts_user
    ON reading_report_drafts (tenant_id, user_id);

-- ---- source: 000035_reading_report_draft_version.up.sql ----
-- SQLite mirror of 000123_reading_report_draft_version.

ALTER TABLE reading_report_drafts
    ADD COLUMN version INTEGER NOT NULL DEFAULT 0;

-- ---- source: 000036_reading_question_kind.up.sql ----
-- SQLite mirror of 000124_reading_question_kind.

ALTER TABLE reading_questions ADD COLUMN kind TEXT NOT NULL DEFAULT '';
ALTER TABLE reading_questions ADD COLUMN gap_type TEXT NOT NULL DEFAULT '';
