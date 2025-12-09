-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    email VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create index on username for faster lookups
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);

-- Insert default test users (admin and user)
-- admin:admin123, user:user123
INSERT INTO users (username, password_hash) VALUES
    ('admin', '$2a$10$ptanDVQHNgfOoHjLHMmpi.MkoHqru/HPEcmV.j14okPx8QKVlwue2'),
    ('user', '$2a$10$E0Ljq24iBKdLMb8BLR9IeOZNfQd..2BfR0pL.j1fGaLUJP8MrJTE.')
ON CONFLICT (username) DO NOTHING;
