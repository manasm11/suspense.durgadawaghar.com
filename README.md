# Suspense Account Manager

A web application for managing suspense account transactions from receipt books. Parses transaction data, extracts payment identifiers, and automatically matches transactions to parties.

## Features

- **Receipt Book Parsing**: Import text from receipt books and automatically parse transactions
- **Identifier Extraction**: Automatically extracts:
  - UPI VPAs (e.g., `user@ybl`, `name@hdfc`)
  - Phone numbers (Indian 10-digit mobile numbers)
  - Bank account numbers (9-18 digit account numbers)
  - IFSC codes (bank branch identifiers)
- **Payment Mode Detection**: Identifies transaction types:
  - UPI, IMPS, NEFT, RTGS, CLG (clearing/cheque), INF (internal fund transfer), TRF (transfer), CHEQUE, POS, CASH
- **Party Matching**: Automatically links transactions to parties based on extracted identifiers
- **Search**: Search parties by name, location, or any identifier

## Prerequisites

- Go 1.24+
- [templ](https://templ.guide/) - HTML templating
- SQLite (via modernc.org/sqlite, no CGO required)

## Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/suspense.durgadawaghar.com.git
cd suspense.durgadawaghar.com

# Install dependencies and tools
go mod download
go install github.com/a-h/templ/cmd/templ@latest
```

## Usage

### Build and Run

```bash
# Using Makefile (recommended)
make build    # Generate templ files and build
make run      # Generate and run server

# Or manually
go generate ./...
go build -o bin/server ./cmd/server
./bin/server
```

### Command Line Options

```
-port int    HTTP server port (default 8005)
-db string   SQLite database path (default "suspense.db")
```

### Development

```bash
# Run tests
make test     # or: go test ./...

# Format code
make fmt      # or: go fmt ./... && templ fmt .

# Regenerate sqlc code after schema changes
make sqlc
```

## Project Structure

```
.
├── cmd/server/          # Main application entry point
├── internal/
│   ├── db/              # Database schema and sqlc config
│   ├── extractor/       # Identifier extraction from narrations
│   ├── handler/         # HTTP handlers
│   ├── matcher/         # Party matching logic
│   ├── parser/          # Receipt book text parsing
│   └── views/           # Templ templates
├── static/              # Static assets (CSS)
├── Caddyfile            # Caddy server config (production)
├── Makefile             # Build commands
└── sqlc.yaml            # SQLC configuration
```

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /` | Home page with search |
| `POST /search` | Search parties by narration |
| `GET /parties` | List all parties |
| `GET /party/{id}` | Party details and transactions |
| `GET /import` | Import form |
| `POST /import/preview` | Preview parsed transactions |
| `POST /import/confirm` | Confirm and save import |

## License

MIT
