# Fin-Line: Financial Market Data Pipeline

A high-performance, real-time financial market data processing pipeline built with Go. This system ingests, normalizes, analyzes, and serves financial market data with comprehensive validation, persistent storage, and secure authentication.

## 🚀 Features

- **Real-time Data Ingestion**: WebSocket and HTTP endpoints for market data feeds
- **Data Validation & Sanomalization**: Comprehensive input validation and data cleaning
- **PostgreSQL Integration**: Persistent storage with connection pooling and migrations
- **JWT Authentication**: Secure role-based access control
- **Anomaly Detection**: Real-time detection of market anomalies using statistical analysis
- **Redis Streams**: High-performance message queuing and caching
- **GraphQL API**: Flexible data querying with real-time subscriptions
- **Prometheus Metrics**: Comprehensive monitoring and observability
- **Docker Support**: Containerized deployment
- **Health Checks**: Built-in health monitoring and readiness probes

## 🧰 Tech Stack

- **Language:** Go 1.21+
- **API:** GraphQL (gqlgen), REST (net/http, gorilla/mux)
- **Database:** PostgreSQL (lib/pq)
- **Cache & Messaging:** Redis Streams (go-redis)
- **Authentication:** JWT (golang-jwt), RSA keys
- **WebSockets:** gorilla/websocket
- **Validation:** go-playground/validator
- **Monitoring:** Prometheus (client_golang)
- **Logging:** Uber Zap
- **Containerization:** Docker, Docker Compose
- **Other:** Make, OpenSSL (for key generation)

## 🏗️ Architecture

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Ingest    │───▶│ Normalize   │───▶│ Cache/Pub   │───▶│    API      │
│  Service    │    │  Service    │    │  Service    │    │  Service    │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
       │                   │                   │                   │
       ▼                   ▼                   ▼                   ▼
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Redis     │    │ PostgreSQL  │    │   Redis     │    │  GraphQL    │
│  Streams    │    │  Database   │    │  Pub/Sub    │    │  Endpoint   │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
```

### Service Components

- **Ingest Service**: Receives raw market data from various sources
- **Normalize Service**: Cleans and standardizes data format
- **Cache/Pub Service**: Manages Redis caching and pub/sub messaging
- **Anomaly Detection**: Identifies statistical anomalies in price movements
- **API Service**: Provides REST and GraphQL endpoints
- **Archival Service**: Long-term data storage and backup

## 📋 Prerequisites

Before running this project, ensure you have the following installed:

- **Go 1.21+**: [Download Go](https://golang.org/dl/)
- **PostgreSQL 13+**: [Download PostgreSQL](https://www.postgresql.org/download/)
- **Redis 6+**: [Download Redis](https://redis.io/download)
- **Docker** (optional): [Download Docker](https://www.docker.com/products/docker-desktop)

## 🛠️ Installation & Setup

### 1. Clone the Repository

```bash
git clone https://github.com/yourusername/fin_line.git
cd fin_line
```

### 2. Database Setup

#### PostgreSQL Installation

**macOS (using Homebrew):**
```bash
brew install postgresql
brew services start postgresql
```

**Ubuntu/Debian:**
```bash
sudo apt update
sudo apt install postgresql postgresql-contrib
sudo systemctl start postgresql
sudo systemctl enable postgresql
```

**Windows:**
Download and install from [PostgreSQL official website](https://www.postgresql.org/download/windows/)

#### Create Database and User

```sql
-- Connect to PostgreSQL as superuser
sudo -u postgres psql

-- Create database and user
CREATE DATABASE fin_line;
CREATE USER fin_line_user WITH PASSWORD 'your_secure_password';
GRANT ALL PRIVILEGES ON DATABASE fin_line TO fin_line_user;
ALTER USER fin_line_user CREATEDB;
\q
```

### 3. Redis Setup

**macOS (using Homebrew):**
```bash
brew install redis
brew services start redis
```

**Ubuntu/Debian:**
```bash
sudo apt install redis-server
sudo systemctl start redis-server
sudo systemctl enable redis-server
```

**Windows:**
Download and install from [Redis official website](https://redis.io/download)

### 4. JWT Key Generation

Generate RSA key pair for JWT authentication:

```bash
# Create keys directory
mkdir -p keys

# Generate private key
openssl genrsa -out keys/private.pem 2048

# Generate public key
openssl rsa -in keys/private.pem -pubout -out keys/public.pem

# Set proper permissions
chmod 600 keys/private.pem
chmod 644 keys/public.pem
```

### 5. Environment Configuration

Create a `.env` file in the project root:

```bash
# Database Configuration
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=fin_line_user
export DB_PASSWORD=your_secure_password
export DB_NAME=fin_line
export DB_SSLMODE=disable
export DB_MAX_OPEN_CONNS=25
export DB_MAX_IDLE_CONNS=5
export DB_CONN_MAX_LIFETIME=5m
export DB_CONN_MAX_IDLE_TIME=5m

# Redis Configuration
export REDIS_URL=redis://localhost:6379

# JWT Configuration
export JWT_PRIVATE_KEY_PATH=keys/private.pem
export JWT_PUBLIC_KEY_PATH=keys/public.pem
export JWT_ISSUER=fin-line
export JWT_AUDIENCE=fin-line-api
export JWT_EXPIRATION=24h

# Application Configuration
export ENVIRONMENT=development
export LOG_LEVEL=info
export API_PORT=8080

# Service Configuration
export INGEST_PORT=8081
export NORMALIZE_PORT=8082
export CACHEPUB_PORT=8083
export ANOMALY_PORT=8084
export ARCHIVAL_PORT=8085
```

### 6. Install Dependencies

```bash
# Download Go modules
go mod download

# Verify dependencies
go mod verify
```

### 7. Run Database Migrations

The database migrations will run automatically when you start the API service, but you can also run them manually:

```bash
# Build the application
go build -o bin/api cmd/api/main.go

# Run migrations manually (optional)
./bin/api --migrate-only
```

## 🚀 Running the Application

### Development Mode

#### Option 1: Run All Services (Recommended)

```bash
# Start all services in development mode
make dev
```

#### Option 2: Run Services Individually

```bash
# Terminal 1: Start API service
go run cmd/api/main.go

# Terminal 2: Start Ingest service
go run cmd/ingest/main.go

# Terminal 3: Start Normalize service
go run cmd/normalize/main.go

# Terminal 4: Start Cache/Pub service
go run cmd/cachepub/main.go

# Terminal 5: Start Anomaly Detection service
go run cmd/anomaly/main.go

# Terminal 6: Start Archival service
go run cmd/archival/main.go
```

### Production Mode

```bash
# Build all services
make build

# Run with production configuration
ENVIRONMENT=production ./bin/api
ENVIRONMENT=production ./bin/ingest
ENVIRONMENT=production ./bin/normalize
ENVIRONMENT=production ./bin/cachepub
ENVIRONMENT=production ./bin/anomaly
ENVIRONMENT=production ./bin/archival
```

### Docker Deployment

```bash
# Build Docker images
docker-compose build

# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

## 📊 API Endpoints

### Health Checks
- `GET /health` - Health check endpoint
- `GET /ready` - Readiness check endpoint
- `GET /metrics` - Prometheus metrics

### Public Endpoints (No Authentication Required)
- `GET /api/v1/quotes/latest` - Get latest quotes for all tickers
- `GET /api/v1/quotes/{ticker}` - Get quotes for specific ticker
- `GET /api/v1/stats` - Get system statistics

### Protected Endpoints (Authentication Required)
- `GET /api/v1/quotes/sector/{sector}` - Get quotes by sector
- `GET /api/v1/quotes/{ticker}/history` - Get quote history
- `GET /api/v1/anomalies` - Get detected anomalies
- `GET /api/v1/anomalies/{ticker}` - Get anomalies for specific ticker

### Admin Endpoints (Admin Role Required)
- `GET /api/v1/admin/raw-events` - Get raw events
- `GET /api/v1/admin/raw-events/source/{source}` - Get raw events by source
- `GET /api/v1/admin/migrations/status` - Get migration status

### GraphQL Endpoint
- `POST /graphql` - GraphQL endpoint with subscriptions

## 🔐 Authentication

### Getting a JWT Token

```bash
# Example: Generate a token (you'll need to implement a login endpoint)
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "user", "password": "password"}'
```

### Using JWT Token

```bash
# Include token in Authorization header
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  http://localhost:8080/api/v1/quotes/sector/technology
```

## 🧪 Testing

### Run All Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with verbose output
go test -v ./...
```

### Run Specific Test Suites

```bash
# Test validation package
go test ./pkg/validation

# Test database package
go test ./pkg/database

# Test authentication package
go test ./pkg/auth
```

### Integration Tests

```bash
# Run integration tests (requires running services)
make test-integration
```

## 📈 Monitoring

### Prometheus Metrics

Access metrics at `http://localhost:8080/metrics`

Key metrics include:
- Request duration and count
- Database operation performance
- Redis operation performance
- Authentication metrics
- System resource usage

### Health Checks

```bash
# Check API health
curl http://localhost:8080/health

# Check readiness
curl http://localhost:8080/ready
```

### Logs

Logs are written to stdout/stderr and can be configured via environment variables:

```bash
export LOG_LEVEL=debug  # debug, info, warn, error
export LOG_FORMAT=json  # json, console
```

## 🔧 Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `ENVIRONMENT` | Application environment | `development` |
| `LOG_LEVEL` | Logging level | `info` |
| `API_PORT` | API service port | `8080` |
| `DB_HOST` | Database host | `localhost` |
| `DB_PORT` | Database port | `5432` |
| `REDIS_URL` | Redis connection URL | `redis://localhost:6379` |
| `JWT_EXPIRATION` | JWT token expiration | `24h` |

### Configuration Files

The application supports configuration via:
- Environment variables (highest priority)
- Configuration files (YAML/JSON)
- Default values (lowest priority)

## 🐛 Troubleshooting

### Common Issues

#### Database Connection Issues
```bash
# Check PostgreSQL status
sudo systemctl status postgresql

# Check database connectivity
psql -h localhost -U fin_line_user -d fin_line

# Verify environment variables
echo $DB_HOST $DB_PORT $DB_USER $DB_NAME
```

#### Redis Connection Issues
```bash
# Check Redis status
sudo systemctl status redis-server

# Test Redis connectivity
redis-cli ping

# Check Redis logs
sudo journalctl -u redis-server
```

#### JWT Key Issues
```bash
# Verify key files exist
ls -la keys/

# Check key permissions
chmod 600 keys/private.pem
chmod 644 keys/public.pem

# Test key generation
openssl rsa -in keys/private.pem -check
```

#### Port Conflicts
```bash
# Check if ports are in use
netstat -tulpn | grep :8080
lsof -i :8080

# Kill process using port
sudo kill -9 <PID>
```

## 📚 API Documentation

### GraphQL Playground

Access the GraphQL playground at `http://localhost:8080/graphql/playground`

### Example Queries

```graphql
# Get latest quotes
query {
  latestQuotes {
    ticker
    price
    timestamp
    sector
  }
}

# Get quotes for specific ticker
query {
  quotes(ticker: "AAPL", limit: 10) {
    ticker
    price
    timestamp
  }
}

# Get anomalies
query {
  anomalies(minZScore: 2.0, limit: 10) {
    ticker
    price
    zScore
    timestamp
  }
}
```

## 🔮 Future Development

### Phase 1: Enhanced Data Processing (Next 3 months)

#### Machine Learning Integration
- **Anomaly Detection Improvements**: Implement more sophisticated ML models (LSTM, Isolation Forest)
- **Price Prediction**: Add price forecasting capabilities using time series analysis
- **Sentiment Analysis**: Integrate news and social media sentiment analysis
- **Pattern Recognition**: Identify technical analysis patterns automatically

#### Data Enrichment
- **Multiple Data Sources**: Integrate additional market data providers (Alpha Vantage, IEX Cloud, Polygon)
- **Fundamental Data**: Add company fundamentals, earnings, and financial ratios
- **News Integration**: Real-time news feed integration with sentiment scoring
- **Social Media**: Twitter and Reddit sentiment analysis for crypto assets

### Phase 2: Advanced Analytics (3-6 months)

#### Real-time Analytics
- **Streaming Analytics**: Apache Kafka integration for real-time data processing
- **Complex Event Processing**: Detect complex market events and patterns
- **Risk Management**: Real-time risk assessment and alerting
- **Portfolio Analytics**: Portfolio performance tracking and optimization

#### Advanced Features
- **Backtesting Engine**: Historical strategy backtesting capabilities
- **Algorithmic Trading**: Basic algorithmic trading signal generation
- **Market Microstructure**: Order book analysis and market depth
- **Cross-Asset Correlation**: Multi-asset correlation analysis

### Phase 3: Enterprise Features (6-12 months)

#### Scalability & Performance
- **Horizontal Scaling**: Kubernetes deployment with auto-scaling
- **Database Sharding**: Implement database sharding for high-volume data
- **Caching Layer**: Redis Cluster for distributed caching
- **Load Balancing**: Advanced load balancing and failover

#### Security & Compliance
- **Multi-factor Authentication**: TOTP and hardware key support
- **Audit Logging**: Comprehensive audit trail for compliance
- **Data Encryption**: End-to-end encryption for sensitive data
- **SOC 2 Compliance**: Security and compliance certifications

#### Advanced APIs
- **WebSocket API**: Real-time streaming API for live data
- **REST API v2**: Enhanced REST API with pagination and filtering
- **GraphQL Subscriptions**: Real-time GraphQL subscriptions
- **Webhook Support**: Configurable webhooks for events

### Phase 4: Platform Features (12+ months)

#### User Management
- **Multi-tenancy**: Support for multiple organizations
- **Role-based Access Control**: Advanced permission system
- **API Key Management**: Self-service API key generation
- **Usage Analytics**: API usage tracking and billing

#### Integration & Ecosystem
- **Third-party Integrations**: Trading platforms, CRM systems
- **Plugin System**: Extensible plugin architecture
- **API Marketplace**: Public API marketplace for data consumers
- **Developer Portal**: Comprehensive developer documentation

#### Advanced Analytics
- **Custom Indicators**: User-defined technical indicators
- **Strategy Builder**: Visual strategy building interface
- **Risk Analytics**: Advanced risk modeling and stress testing
- **Performance Attribution**: Detailed performance analysis

### Technology Stack Evolution

#### Current Stack
- **Backend**: Go, PostgreSQL, Redis
- **API**: REST, GraphQL
- **Authentication**: JWT
- **Monitoring**: Prometheus, Grafana

#### Future Additions
- **Message Queue**: Apache Kafka
- **Stream Processing**: Apache Flink
- **Machine Learning**: Python ML services
- **Frontend**: React/TypeScript dashboard
- **Mobile**: React Native mobile app

### Development Priorities

1. **Immediate (Next Sprint)**
   - Fix any critical bugs
   - Add comprehensive test coverage
   - Implement proper error handling
   - Add API rate limiting

2. **Short-term (Next Month)**
   - Add more data sources
   - Implement caching strategies
   - Add comprehensive logging
   - Create deployment automation

3. **Medium-term (Next Quarter)**
   - Machine learning integration
   - Advanced analytics features
   - Performance optimization
   - Security hardening

4. **Long-term (Next Year)**
   - Enterprise features
   - Platform capabilities
   - Advanced integrations
   - Global expansion

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### Development Setup

```bash
# Fork and clone the repository
git clone https://github.com/yourusername/fin_line.git
cd fin_line

# Create a feature branch
git checkout -b feature/your-feature-name

# Make your changes
# Add tests for new functionality
# Update documentation

# Commit your changes
git commit -m "Add your feature description"

# Push to your fork
git push origin feature/your-feature-name

# Create a pull request
```

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🆘 Support

- **Documentation**: [Implementation Guide](IMPLEMENTATION_GUIDE.md)
- **Issues**: [GitHub Issues](https://github.com/yourusername/fin_line/issues)
- **Discussions**: [GitHub Discussions](https://github.com/yourusername/fin_line/discussions)
- **Email**: support@fin-line.com

## 🙏 Acknowledgments

- Built with [Go](https://golang.org/)
- Database powered by [PostgreSQL](https://www.postgresql.org/)
- Caching with [Redis](https://redis.io/)
- Monitoring with [Prometheus](https://prometheus.io/)
- Authentication with [JWT](https://jwt.io/)

---

**Fin-Line** - Empowering financial data analysis with real-time insights and advanced analytics.