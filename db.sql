-- Welcome to N0CTURNALBBS.
-- Table generation is manual here, it isn't built into the code. 


CREATE TABLE boards (
    id SERIAL PRIMARY KEY,
    slug VARCHAR(50) NOT NULL UNIQUE,  
    name VARCHAR(100) NOT NULL,       
    description TEXT NOT NULL,
    locked BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    active BOOLEAN DEFAULT TRUE
);

CREATE INDEX idx_boards_locked ON boards(locked) WHERE locked = TRUE;

CREATE TABLE threads (
    id SERIAL PRIMARY KEY,
    board_id INTEGER NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    title VARCHAR(200) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    is_pinned BOOLEAN DEFAULT FALSE,
    is_locked BOOLEAN DEFAULT FALSE,
    last_post_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    post_count INTEGER DEFAULT 1
);

CREATE TABLE posts (
    id SERIAL PRIMARY KEY,
    thread_id INTEGER NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    author VARCHAR(100),             
    body TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    ip_hash VARCHAR(64),            
    tripcode VARCHAR(20)
);

CREATE INDEX idx_threads_board_id ON threads(board_id);
CREATE INDEX idx_threads_last_post_at ON threads(last_post_at);
CREATE INDEX idx_posts_thread_id ON posts(thread_id);

CREATE TABLE moderators (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL, 
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_login TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN DEFAULT TRUE
);

CREATE TABLE mod_sessions (
    id VARCHAR(64) PRIMARY KEY,   
    moderator_id INTEGER NOT NULL REFERENCES moderators(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    ip_address VARCHAR(45),      
    user_agent TEXT
);

CREATE TABLE mod_actions (
    id SERIAL PRIMARY KEY,
    moderator_id INTEGER NOT NULL REFERENCES moderators(id),
    action_type VARCHAR(50) NOT NULL, 
    target_id INTEGER NOT NULL,      
    target_type VARCHAR(20) NOT NULL,
    executed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    ip_address VARCHAR(45),
    reason TEXT
);

CREATE TABLE banned_words (
    id SERIAL PRIMARY KEY,
    word VARCHAR(255) NOT NULL UNIQUE, 
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_banned_words_word ON banned_words USING gin(word gin_trgm_ops);

CREATE EXTENSION IF NOT EXISTS pg_trgm;