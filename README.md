# Pottery Barn Backend API

Backend API and scraper for fetching Pottery Barn open-box deals products.

## Setup

1. Install Node.js dependencies (for API server):

```bash
npm install
```

2. Install Go dependencies (for scraper):

```bash
go mod download
```

3. Initialize the database schema:

```bash
node db/init.js
```

3. Create a `.env` file (optional, defaults are set in code):

```env
DB_HOST=dpg-d4ficjqli9vc73ag07qg-a.ohio-postgres.render.com
DB_PORT=5432
DB_NAME=potterybarn
DB_USER=potterybarn_user
DB_PASSWORD=07qYYLJ81qN8uumJIj8p0fGxZ4P59a1w
PORT=3001
```

## Usage

### Start the API server:

```bash
npm start
# or for development with auto-reload:
npm run dev
```

### Run the scraper (Go):

```bash
npm run scrape
# or
go run scraper.go
# or build and run:
go build -o scraper scraper.go
./scraper
```

### Trigger scraping via API:

```bash
POST http://localhost:3001/api/products/scrape
```

## API Endpoints

- `GET /` - API information
- `GET /health` - Health check (checks database connection)
- `GET /api/products` - Get all products (supports pagination: `?page=1&limit=50&sort=created_at&order=DESC`)
- `GET /api/products/:id` - Get product by ID
- `GET /api/products/grade/:grade` - Get products by grade (A, B, C, Open Box)
- `POST /api/products/scrape` - Trigger product scraping

## Database Schema

The `products` table stores:

- `id` - Primary key
- `name` - Product name
- `price` - Product price (decimal)
- `grade` - Product grade (A, B, C, Open Box, or null)
- `image_url` - Product image URL
- `product_url` - Product page URL (unique)
- `created_at` - Timestamp when product was first added
- `updated_at` - Timestamp when product was last updated

## Notes

- The scraper is written in Go using chromedp (headless browser) to handle JavaScript-rendered content and infinite scroll
- Products are loaded via infinite scroll - the scraper automatically scrolls to load all products
- Products are deduplicated by `product_url`
- The scraper will update existing products if they're found again
- The API server is written in Node.js/Express
