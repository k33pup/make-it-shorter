# Authentication System

## Overview

The URL shortener uses secure authentication with username and password credentials stored in PostgreSQL database. Users can register new accounts and login to create and manage their short URLs.

## Features

- User registration (username + password only)
- Duplicate username prevention
- Secure password storage with bcrypt hashing
- JWT token-based authentication
- Input validation and sanitization
- PostgreSQL database for persistent storage

## Backend Implementation

### Database Layer (`pkg/database/users.go`)

1. **User Model**
   - ID, Username, PasswordHash, Email, CreatedAt, UpdatedAt
   - Indexed username for fast lookups

2. **User Management Functions**
   - `CreateUser()` - Register new user with bcrypt password hashing
   - `ValidateUser()` - Verify username and password
   - `UserExists()` - Check if username is taken
   - `GetUser()` - Retrieve user information

### API Endpoints (`services/gateway/main.go`)

1. **Registration Endpoint** (`/api/register`)
   - Validates username (3-50 chars), password (min 6 chars)
   - Checks for duplicate usernames
   - Hashes password with bcrypt
   - Creates user and returns JWT token for auto-login

2. **Login Endpoint** (`/api/login`)
   - Validates credentials against database
   - Returns JWT token on success
   - Generic error message to prevent user enumeration

## Frontend Implementation

### UI Components (`web/static/index.html`)

1. **Login Form**
   - Username and password inputs
   - Link to switch to registration form

2. **Registration Form**
   - Username, password, and password confirmation inputs
   - Client-side validation
   - Link to switch back to login
   - Styled to match existing design with gradient buttons

### JavaScript (`web/static/app.js`)

1. **Registration Function**
   - Validates form inputs (username length, password length, password match)
   - Sends POST request to `/api/register`
   - Auto-login on successful registration
   - Clear error messages

2. **Login Function**
   - Sends POST request to `/api/login`
   - Stores JWT token and username in localStorage
   - Redirects to main application

3. **Form Switching**
   - Toggle between login and registration views
   - Clear forms on logout

## Test Users

The system includes two test users:

- **admin** / admin123
- **user** / user123

## Security Features

- Passwords are hashed using bcrypt (cost factor: 10)
- JWT tokens expire after 24 hours
- Input sanitization on all user inputs
- Failed login attempts return generic error message to prevent user enumeration

## API Examples

### Registration Request

```bash
curl -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "newuser",
    "password": "securepass123"
  }'
```

**Successful Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user_id": "newuser",
  "message": "User created successfully"
}
```

**Error Responses:**
- `Username must be between 3 and 50 characters`
- `Password must be at least 6 characters`
- `Username already taken`

### Login Request

```bash
curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "admin123"
  }'
```

**Successful Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user_id": "admin"
}
```

**Error Response:**
```
Invalid username or password
```

## Database Schema

```sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    email VARCHAR(255),  -- Optional, can be empty
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_username ON users(username);
```

## Production Recommendations

1. ✅ ~~Replace in-memory user storage with a database~~ - **Done with PostgreSQL**
2. ✅ ~~Implement user registration endpoint~~ - **Done**
3. Add password complexity requirements (uppercase, lowercase, numbers, special chars)
4. Implement account lockout after failed login attempts
5. Add password reset functionality via email
6. Use environment variables for JWT secret (currently hardcoded)
7. Consider adding 2FA support
8. Add email verification on registration
9. Implement rate limiting on registration endpoint
10. Add CAPTCHA for bot protection
11. Add session management and token refresh
12. Implement password change functionality
