-- Git repositories the system deploys from.
CREATE TABLE IF NOT EXISTS repositories (
    id blob PRIMARY KEY,
    name varchar(255) not null default '',
    url text not null UNIQUE,
    username text not null default '',
    -- base64 AES-GCM sealed secrets, '' when unset. Text, not blob: sqlc maps every blob to a UUID.
    password text not null default '',
    ssh_private_key text not null default '',
    insecure boolean not null default false,
    created_at timestamp not null default CURRENT_TIMESTAMP,
    updated_at timestamp not null default CURRENT_TIMESTAMP
);
