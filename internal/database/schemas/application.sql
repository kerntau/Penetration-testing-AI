-- Latest SQLite structure. Update these definitions when adding tables or columns.

CREATE TABLE conversations (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    role_name TEXT NOT NULL DEFAULT '默认',
    agent_mode TEXT NOT NULL DEFAULT 'eino_single',
    last_react_input TEXT,
    last_react_output TEXT,
    pinned INTEGER DEFAULT 0,
    project_id TEXT,
    owner_user_id TEXT
);

CREATE TABLE messages (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    mcp_execution_ids TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    reasoning_content TEXT,
    FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
);

CREATE TABLE model_token_usage (
    id TEXT PRIMARY KEY,
    process_detail_id TEXT NOT NULL UNIQUE,
    message_id TEXT NOT NULL,
    conversation_id TEXT NOT NULL,
    project_id TEXT,
    source TEXT NOT NULL DEFAULT '',
    orchestration TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    model_calls INTEGER NOT NULL DEFAULT 0,
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    cached_tokens INTEGER NOT NULL DEFAULT 0,
    cache_write_tokens INTEGER NOT NULL DEFAULT 0,
    reasoning_tokens INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    FOREIGN KEY (process_detail_id) REFERENCES process_details(id) ON DELETE CASCADE,
    FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE,
    FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
);

CREATE TABLE process_details (
    id TEXT PRIMARY KEY,
    message_id TEXT NOT NULL,
    conversation_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    message TEXT,
    data TEXT,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE,
    FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
);

CREATE TABLE skill_stats (
    skill_name TEXT PRIMARY KEY,
    total_calls INTEGER NOT NULL DEFAULT 0,
    success_calls INTEGER NOT NULL DEFAULT 0,
    failed_calls INTEGER NOT NULL DEFAULT 0,
    last_call_time DATETIME,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE tool_executions (
    id TEXT PRIMARY KEY,
    tool_name TEXT NOT NULL,
    arguments TEXT NOT NULL,
    status TEXT NOT NULL,
    result TEXT,
    error TEXT,
    start_time DATETIME NOT NULL,
    end_time DATETIME,
    duration_ms INTEGER,
    partial_output TEXT,
    partial_output_bytes INTEGER NOT NULL DEFAULT 0,
    partial_output_truncated INTEGER NOT NULL DEFAULT 0,
    partial_output_updated_at DATETIME,
    owner_user_id TEXT,
    conversation_id TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE tool_stats (
    tool_name TEXT PRIMARY KEY,
    total_calls INTEGER NOT NULL DEFAULT 0,
    success_calls INTEGER NOT NULL DEFAULT 0,
    failed_calls INTEGER NOT NULL DEFAULT 0,
    last_call_time DATETIME,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE vulnerabilities (
    id TEXT PRIMARY KEY,
    conversation_id TEXT,
    conversation_tag TEXT,
    task_tag TEXT,
    title TEXT NOT NULL,
    description TEXT,
    severity TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',
    vulnerability_type TEXT,
    target TEXT,
    preconditions TEXT,
    reproduction_steps TEXT,
    evidence TEXT,
    impact TEXT,
    recommendation TEXT,
    retest_notes TEXT,
    retest_log TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    project_id TEXT,
    owner_user_id TEXT,
    FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE SET NULL
);

CREATE TABLE vulnerability_retests (
    conversation_id TEXT PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE,
    source_conversation_id TEXT NOT NULL,
    vulnerability_id TEXT NOT NULL,
    source_project_id TEXT NOT NULL DEFAULT '',
    pending_prompt TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_conversations_pinned ON conversations(pinned);
CREATE INDEX idx_conversations_updated_at ON conversations(updated_at);
CREATE INDEX idx_messages_conversation_id ON messages(conversation_id);
CREATE INDEX idx_model_token_usage_conversation ON model_token_usage(conversation_id);
CREATE INDEX idx_model_token_usage_created_at ON model_token_usage(created_at);
CREATE INDEX idx_model_token_usage_model ON model_token_usage(model);
CREATE INDEX idx_process_details_conversation_id ON process_details(conversation_id);
CREATE INDEX idx_process_details_message_id ON process_details(message_id);
CREATE INDEX idx_tool_executions_conversation ON tool_executions(conversation_id);
CREATE INDEX idx_tool_executions_owner ON tool_executions(owner_user_id);
CREATE INDEX idx_tool_executions_start_time ON tool_executions(start_time);
CREATE INDEX idx_tool_executions_status ON tool_executions(status);
CREATE INDEX idx_tool_executions_tool_name ON tool_executions(tool_name);
CREATE INDEX idx_vulnerabilities_conversation_id ON vulnerabilities(conversation_id);
CREATE INDEX idx_vulnerabilities_conversation_tag ON vulnerabilities(conversation_tag);
CREATE INDEX idx_vulnerabilities_created_at ON vulnerabilities(created_at);
CREATE INDEX idx_vulnerabilities_severity ON vulnerabilities(severity);
CREATE INDEX idx_vulnerabilities_status ON vulnerabilities(status);
CREATE INDEX idx_vulnerabilities_task_tag ON vulnerabilities(task_tag);
