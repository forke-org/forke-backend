-- PostgreSQL schema for Forke Backend
-- Mirrored 1:1 from Drizzle Schema

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Enums
DO $$ BEGIN
    CREATE TYPE user_role AS ENUM ('developer', 'owner');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE task_status AS ENUM ('processing', 'open', 'claimed', 'submitted', 'approved', 'disputed');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE submission_status AS ENUM ('pending', 'approved', 'rejected');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE escrow_status AS ENUM ('held', 'released', 'refunded');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE blog_status AS ENUM ('draft', 'published');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- 1. Users Table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username TEXT UNIQUE,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    email_verified TIMESTAMP WITH TIME ZONE,
    image TEXT,
    github_avatar_url TEXT,
    google_avatar_url TEXT,
    password_hash TEXT,
    role user_role NOT NULL DEFAULT 'developer',
    level INTEGER NOT NULL DEFAULT 1,
    xp INTEGER NOT NULL DEFAULT 0,
    github_url TEXT,
    bio TEXT,
    headline TEXT,
    location TEXT,
    website_url TEXT,
    linkedin_url TEXT,
    github_stats JSONB,
    last_login_at TIMESTAMP WITH TIME ZONE,
    current_streak INTEGER NOT NULL DEFAULT 0,
    is_approved BOOLEAN NOT NULL DEFAULT false,
    is_banned BOOLEAN NOT NULL DEFAULT false,
    email_alerts BOOLEAN DEFAULT true,
    slack_webhooks BOOLEAN DEFAULT false,
    college TEXT,
    deletion_scheduled_at TIMESTAMP WITH TIME ZONE,
    attribution JSONB,
    last_active_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 2. Owners Table
CREATE TABLE IF NOT EXISTS owners (
    id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    contact_number TEXT NOT NULL,
    contact_email TEXT NOT NULL,
    company_name TEXT NOT NULL,
    company_website TEXT,
    personal_linkedin TEXT NOT NULL,
    company_linkedin TEXT NOT NULL,
    designation TEXT NOT NULL,
    other_links TEXT,
    message TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 3. Admins Table
CREATE TABLE IF NOT EXISTS admins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    username TEXT UNIQUE,
    password_hash TEXT,
    role TEXT NOT NULL DEFAULT 'admin',
    alternative_email TEXT,
    invite_token TEXT UNIQUE,
    invite_expires_at TIMESTAMP WITH TIME ZONE,
    is_disabled BOOLEAN NOT NULL DEFAULT false,
    last_login_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 4. Auth Accounts Table
CREATE TABLE IF NOT EXISTS account (
    "userId" UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    provider TEXT NOT NULL,
    "providerAccountId" TEXT NOT NULL,
    refresh_token TEXT,
    access_token TEXT,
    expires_at INTEGER,
    token_type TEXT,
    scope TEXT,
    id_token TEXT,
    session_state TEXT,
    PRIMARY KEY (provider, "providerAccountId")
);

-- 5. Sessions Table
CREATE TABLE IF NOT EXISTS session (
    "sessionToken" TEXT PRIMARY KEY,
    "userId" UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires TIMESTAMP WITH TIME ZONE NOT NULL
);

-- 6. Sandbox Users
CREATE TABLE IF NOT EXISTS sandbox_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    github_id INTEGER NOT NULL UNIQUE,
    username TEXT NOT NULL,
    access_token TEXT NOT NULL,
    role TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 7. Sandbox Repos
CREATE TABLE IF NOT EXISTS sandbox_repos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES sandbox_users(id) ON DELETE CASCADE,
    source_repo TEXT NOT NULL,
    sandbox_repo TEXT NOT NULL,
    task_title TEXT,
    task_description TEXT,
    frontend_stack TEXT,
    backend_stack TEXT,
    allowed_paths TEXT,
    restricted_paths TEXT,
    acceptance_criteria TEXT,
    verification_status TEXT NOT NULL DEFAULT 'verifying',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 8. Tasks Table
CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    budget INTEGER NOT NULL,
    currency TEXT NOT NULL DEFAULT 'INR',
    status task_status NOT NULL DEFAULT 'open',
    skill_tags TEXT[],
    client_id UUID NOT NULL REFERENCES users(id),
    claimant_id UUID REFERENCES users(id),
    claimed_at TIMESTAMP WITH TIME ZONE,
    deadline TIMESTAMP WITH TIME ZONE,
    sandbox_repo_id UUID REFERENCES sandbox_repos(id) ON DELETE SET NULL,
    source_repo TEXT,
    codespace_name TEXT,
    codespace_url TEXT,
    codespace_status TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 9. Submissions Table
CREATE TABLE IF NOT EXISTS submissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id),
    developer_id UUID NOT NULL REFERENCES users(id),
    github_link TEXT NOT NULL,
    note TEXT,
    status submission_status NOT NULL DEFAULT 'pending',
    rating INTEGER,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 10. Escrow Table
CREATE TABLE IF NOT EXISTS escrow (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL UNIQUE REFERENCES tasks(id),
    amount INTEGER NOT NULL,
    status escrow_status NOT NULL DEFAULT 'held',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 11. Revision Requests Table
CREATE TABLE IF NOT EXISTS revision_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id),
    client_note TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 12. Support Enquiries Table
CREATE TABLE IF NOT EXISTS support_enquiries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    contact_number TEXT NOT NULL,
    contact_email TEXT NOT NULL,
    message TEXT NOT NULL,
    relevant_links TEXT,
    error_type TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 13. Subscribers Table
CREATE TABLE IF NOT EXISTS subscribers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    source TEXT NOT NULL DEFAULT 'direct',
    attribution JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 14. System Settings Table
CREATE TABLE IF NOT EXISTS system_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 15. Admin Audit Log Table
CREATE TABLE IF NOT EXISTS admin_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id UUID,
    actor_name TEXT,
    category TEXT NOT NULL DEFAULT 'admin',
    action TEXT NOT NULL,
    target TEXT,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 16. Backup Runs Table
CREATE TABLE IF NOT EXISTS backup_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMP WITH TIME ZONE,
    status TEXT NOT NULL,
    tier TEXT NOT NULL,
    size_bytes BIGINT,
    r2_key TEXT,
    triggered_by TEXT,
    error_message TEXT
);

-- 17. Developers Profile Table
CREATE TABLE IF NOT EXISTS developers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    github_id TEXT UNIQUE,
    username TEXT,
    access_token TEXT,
    avatar_url TEXT,
    profile_url TEXT,
    repos JSONB,
    languages JSONB,
    raw_profile JSONB,
    is_github_connected BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 18. Messages Table
CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    receiver_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    is_received BOOLEAN NOT NULL DEFAULT false,
    is_seen BOOLEAN NOT NULL DEFAULT false,
    file_url TEXT,
    file_name TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 19. Notifications Table
CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    link TEXT,
    is_read BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 20. Blogs Table
CREATE TABLE IF NOT EXISTS blogs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id UUID REFERENCES admins(id) ON DELETE SET NULL,
    author_name TEXT,
    title TEXT NOT NULL DEFAULT 'Untitled',
    slug TEXT NOT NULL UNIQUE,
    excerpt TEXT,
    cover_image TEXT,
    content JSONB,
    content_html TEXT,
    status blog_status NOT NULL DEFAULT 'draft',
    reading_minutes INTEGER NOT NULL DEFAULT 1,
    views INTEGER NOT NULL DEFAULT 0,
    published_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 21. Page Visits (In-House First-Party Analytics)
CREATE TABLE IF NOT EXISTS page_visits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id TEXT,
    source TEXT NOT NULL DEFAULT 'direct',
    medium TEXT,
    campaign TEXT,
    referrer TEXT,
    landing_path TEXT,
    country TEXT,
    is_bot BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 22. Auth Security Events
CREATE TABLE IF NOT EXISTS auth_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    email TEXT,
    event TEXT NOT NULL,
    provider TEXT,
    ip_hash TEXT,
    country TEXT,
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 23. Changelogs Table (Linear / Supermemory style)
CREATE TABLE IF NOT EXISTS changelogs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    tag TEXT NOT NULL DEFAULT 'CORE',
    description TEXT NOT NULL,
    improvements JSONB DEFAULT '[]'::jsonb,
    fixes JSONB DEFAULT '[]'::jsonb,
    media_type TEXT NOT NULL DEFAULT 'none',
    media_url TEXT,
    is_published BOOLEAN NOT NULL DEFAULT true,
    published_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

