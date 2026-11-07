package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// baselineSQLiteTables is the set of tables the personal-edition baseline
// (migrations/sqlite/000000_init.up.sql) must create on a fresh database. The
// enterprise versioned chain was collapsed into this single migration; this
// list guards the consolidated schema against accidental drift.
var baselineSQLiteTables = []string{
	"tenants",
	"models",
	"knowledge_bases",
	"knowledges",
	"sessions",
	"messages",
	"message_suggestion_sets",
	"message_suggestion_events",
	"chunks",
	"chunk_revisions",
	"users",
	"auth_tokens",
	"user_kb_pins",
	"knowledge_tags",
	"mcp_services",
	"mcp_tool_approvals",
	"mcp_oauth_clients",
	"mcp_oauth_tokens",
	"custom_agents",
	"tenant_sandbox_configs",
	"im_channel_sessions",
	"im_channels",
	"embed_channels",
	"data_sources",
	"sync_logs",
	"web_search_providers",
	"vector_stores",
	"storage_backends",
	"resources",
	"resource_bindings",
	"tenant_api_keys",
	"temporary_documents",
	"gaze_pages",
	"gaze_folders",
	"gaze_page_issues",
	"gaze_page_revisions",
	"memory_subjects",
	"memory_items",
	"memory_tombstones",
	"memory_topic_stats",
	"memory_doc_affinity",
	"memory_item_embeddings",
	"task_pending_ops",
	"task_dead_letters",
	"system_settings",
	"knowledge_processing_spans",
	"knowledge_tag_relations",
	"lite_embeddings",
	"doc_versions",
	"doc_version_chunks",
	"chunk_citation_daily",
	"knowledge_folders",
	"retrieval_audit_logs",
	"tenant_skill_catalog",
	"reading_runs",
	"reading_run_steps",
	"reading_questions",
	"reading_conclusions",
	"reading_terms",
	"reading_contradictions",
	"reading_report_drafts",
}

// baselineSQLiteColumns maps tables to columns the baseline must define
// because the application reads or writes them.
var baselineSQLiteColumns = map[string][]string{
	"tenants":               {"api_principal_config", "memory_config", "reading_model_id"},
	"users":                 {"is_system_admin"},
	"knowledges":            {"pending_subtasks_count", "folder_path", "folder_id", "created_by", "engine_sync_status"},
	"messages":              {"attachments", "usage", "query_rewritten", "query_rewrite_meta", "skill", "reading_run_id"},
	"embed_channels":        {"allow_memory"},
	"mcp_oauth_tokens":      {"principal_type", "principal_id"},
	"mcp_tool_approvals":    {"enabled"},
	"sessions":              {"reading_run_id"},
	"reading_report_drafts": {"version"},
	"reading_questions":     {"kind", "gap_type"},
	"knowledge_bases":       {"auto_tag_config"},
}

func TestSQLiteMigrationsCreateBaselineSchema(t *testing.T) {
	repoRoot := sqliteRepoRoot(t)
	chdirAndRestore(t, repoRoot)

	dbPath := filepath.Join(t.TempDir(), "fresh.db")
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: dbPath}))

	db := openSQLiteDB(t, dbPath)
	version, dirty := sqliteMigrationState(t, db)
	require.Equal(t, 0, version, "the personal-edition baseline is the single migration 000000_init")
	require.False(t, dirty)

	for _, table := range baselineSQLiteTables {
		require.Truef(t, sqliteTableExists(t, db, table), "SQLite migrations must create table %s", table)
	}
	for table, columns := range baselineSQLiteColumns {
		for _, column := range columns {
			require.Truef(
				t,
				sqliteColumnExists(t, db, table, column),
				"SQLite migrations must create column %s.%s",
				table,
				column,
			)
		}
	}

	assertSQLiteMCPOAuthPrincipalUpsertWorks(t, db)
	require.False(t, sqliteColumnExists(t, db, "knowledges", "tag_id"),
		"SQLite migrations must drop legacy knowledges.tag_id after multi-tag migration")
	require.False(t, sqliteTableExists(t, db, "gaze_log_entries"),
		"SQLite migrations must drop the legacy gaze_log_entries table")
}

func sqliteRepoRoot(t *testing.T) string {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	require.NoError(t, err)
	return repoRoot
}

func chdirAndRestore(t *testing.T, dir string) {
	t.Helper()
	previousDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(previousDir) })
}

func openSQLiteDB(t *testing.T, dbPath string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func sqliteMigrationState(t *testing.T, db *sql.DB) (version int, dirty bool) {
	t.Helper()
	require.NoError(t, db.QueryRow("SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty))
	return version, dirty
}

func sqliteTableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
		table,
	).Scan(&n))
	return n == 1
}

func sqliteColumnExists(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(
		"SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?",
		table,
		column,
	).Scan(&n))
	return n == 1
}

func assertSQLiteMCPOAuthPrincipalUpsertWorks(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(
		"INSERT INTO mcp_services (id, tenant_id, name, transport_type) VALUES (?, 1, 'svc', 'http')",
		"svc-migration-1",
	)
	require.NoError(t, err)

	tokenInsertPrefix := "INSERT INTO mcp_oauth_tokens " +
		"(id, tenant_id, user_id, service_id, principal_type, principal_id, access_token) "
	_, err = db.Exec(
		tokenInsertPrefix +
			"VALUES ('tok-1', 1, 'u1', 'svc-migration-1', 'web_user', 'u1', 'token-1')",
	)
	require.NoError(t, err)

	_, err = db.Exec(
		tokenInsertPrefix +
			"VALUES ('tok-2', 1, 'u1', 'svc-migration-1', 'web_user', 'u1', 'token-2') " +
			"ON CONFLICT(tenant_id, principal_type, principal_id, service_id) " +
			"DO UPDATE SET access_token = excluded.access_token",
	)
	require.NoError(t, err)

	var accessToken string
	require.NoError(t, db.QueryRow(
		"SELECT access_token FROM mcp_oauth_tokens "+
			"WHERE tenant_id = 1 AND principal_type = 'web_user' "+
			"AND principal_id = 'u1' AND service_id = 'svc-migration-1'",
	).Scan(&accessToken))
	require.Equal(t, "token-2", accessToken)

	var rowCount int
	require.NoError(t, db.QueryRow(
		"SELECT COUNT(*) FROM mcp_oauth_tokens WHERE tenant_id = 1 AND service_id = 'svc-migration-1'",
	).Scan(&rowCount))
	require.Equal(t, 1, rowCount)
}
