-- owner: the first user, manages users; admin: read/write; user: read only.
ALTER TABLE users ADD COLUMN role varchar(255) not null default 'user' CHECK (role IN ('owner', 'admin', 'user'));

-- Existing installs: the oldest active user becomes the owner.
UPDATE users SET role = 'owner'
WHERE id = (SELECT id FROM users WHERE disabled_at IS NULL ORDER BY created_at, name LIMIT 1);

-- At most one owner.
CREATE UNIQUE INDEX IF NOT EXISTS users_single_owner ON users(role) WHERE role = 'owner';
