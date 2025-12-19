# Analytics API

URL shortener analytics system with global statistics, referer tracking, and hourly distribution.

## Endpoints

### 1. Global Top URLs

Get the most popular short URLs globally.

```bash
GET /api/analytics/top?period={period}&limit={limit}
```

**Parameters:**
- `period` (optional): Time period - `all`, `week`, `month`. Default: `all`
- `limit` (optional): Number of results (1-100). Default: 100

**Example:**
```bash
curl "http://localhost:8080/api/analytics/top?period=all&limit=10"
```

**Response:**
```json
{
  "urls": [
    {
      "short_code": "abc123",
      "clicks": 150
    },
    {
      "short_code": "xyz789",
      "clicks": 120
    }
  ]
}
```

### 2. Top Referers

Get top referers (traffic sources) for a specific short URL.

```bash
GET /api/analytics/referers?code={shortCode}&limit={limit}
```

**Parameters:**
- `code` (required): Short code
- `limit` (optional): Number of results (1-100). Default: 100

**Example:**
```bash
curl "http://localhost:8080/api/analytics/referers?code=abc123&limit=10"
```

**Response:**
```json
{
  "referers": [
    {
      "referer": "https://google.com",
      "count": 45
    },
    {
      "referer": "https://twitter.com",
      "count": 30
    }
  ]
}
```

### 3. Hourly Distribution

Get click distribution by hour for a specific short URL.

```bash
GET /api/analytics/hourly?code={shortCode}&date={date}
```

**Parameters:**
- `code` (required): Short code
- `date` (optional): Date in YYYY-MM-DD format. Default: today

**Example:**
```bash
curl "http://localhost:8080/api/analytics/hourly?code=abc123&date=2025-12-19"
```

**Response:**
```json
{
  "hours": [
    {
      "hour": 0,
      "count": 5
    },
    {
      "hour": 1,
      "count": 3
    },
    {
      "hour": 9,
      "count": 25
    }
  ]
}
```

### 4. URL Stats

Get detailed statistics for a specific short URL.

```bash
GET /api/stats?code={shortCode}
```

**Parameters:**
- `code` (required): Short code

**Example:**
```bash
curl "http://localhost:8080/api/stats?code=abc123"
```

**Response:**
```json
{
  "stats": {
    "total_clicks": 150,
    "unique_clicks": 87,
    "daily_clicks": [
      {
        "date": "2025-12-19",
        "count": 25
      },
      {
        "date": "2025-12-18",
        "count": 30
      }
    ]
  }
}
```

## Data Collection

Analytics are collected automatically on each redirect:
- Total clicks (global and per-URL)
- Unique clicks (by IP address)
- Referer information
- Hourly distribution
- Daily statistics

## Data Retention

- Daily stats: 30 days
- Hourly stats: 30 days
- Referer data: 30 days
- Weekly aggregates: 90 days
- Monthly aggregates: 180 days
- Global all-time stats: permanent

## Use Cases

**Monitor trending links:**
```bash
curl "http://localhost:8080/api/analytics/top?period=week&limit=20"
```

**Analyze traffic sources:**
```bash
curl "http://localhost:8080/api/analytics/referers?code=mylink&limit=50"
```

**Find peak hours:**
```bash
curl "http://localhost:8080/api/analytics/hourly?code=mylink"
```

**Track daily performance:**
```bash
curl "http://localhost:8080/api/stats?code=mylink"
```
