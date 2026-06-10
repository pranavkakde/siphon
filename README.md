# Siphon — Real-time Open-Source Test Analytics Platform

Siphon is a high-throughput, real-time open-source test analytics platform designed to capture automated test suite execution metrics, manage screenshots of test failures via object storage, and stream live execution feeds straight to an interactive, premium engineering dashboard.

---

## Architecture Overview

Siphon is built on a decoupled, microservice-oriented event pipeline:

```
[Test Framework Client Hooks]
            │
            ├─► Upload Failure Screenshots ────► [MinIO Object Storage (9000)]
            │
            └─► Stream Test Case Metadata ─────► [siphon-ingress API (50051)]
                                                          │
                                                (JSON Serialization)
                                                          │
                                                          ▼
                                            [RabbitMQ Buffer Queue (5672)]
                                                          │
                                                (Go Routine Consumer Pool)
                                                          │
                                                          ▼
                                            [siphon-sink Worker Pool]
                                                          │
                                                (Idempotent Upsert Writes)
                                                          │
                                                          ▼
                                              [MongoDB database (27017)]
                                                          │
                                                (Change Stream/Poll Tailing)
                                                          │
                                                          ▼
                                            [siphon-stream-api (8080)]
                                                          │
                                              (Gorilla WebSocket Hub)
                                                          │
                                                          ▼
                                            [siphon-glass React Web UI (5173)]
```

---

## Repository Structure

The project is managed inside a Go workspace (`go.work` multi-module layout) alongside a React web application:

```
siphon/
├── apps/
│   └── siphon-glass/            # Vite + React + TS dashboard client
├── services/
│   ├── siphon-ingress/          # Go gRPC stream ingestion edge service
│   ├── siphon-sink/             # Go consumer worker pool inserting to MongoDB
│   └── siphon-stream-api/       # Go Gorilla WebSocket server tailing Mongo
├── shared/
│   ├── proto/                   # Protobuf schema and compiled Go bindings
│   └── telemetry/               # Shared OpenTelemetry configuration utilities
├── clients/
│   └── go-hook/                 # Mock client framework simulation
├── docker/                      # MongoDB init indexing script
└── docker-compose.yml           # Core database, broker, and LGPL observability configuration
```

---

## Tech Stack & Port Configuration

| Service | Port | Technology | Purpose |
| :--- | :--- | :--- | :--- |
| **`siphon-glass`** | `5173` | React, Recharts, Lucide | Premium, dark-mode real-time visual dashboard |
| **`siphon-ingress`** | `50051` | Go, gRPC, RabbitMQ | High-performance metrics edge stream ingestion |
| **`siphon-sink`** | *Daemon* | Go, MongoDB | Concurrent worker pool executing idempotent data writes |
| **`siphon-stream-api`** | `8080` | Go, Gin, WebSockets | WebSockets and REST endpoint data stream tailer |
| **MongoDB** | `27017` | MongoDB Community | Document database storing flat test metadata |
| **Mongo Express** | `8081` | Web Console UI | Visual interface for exploring Mongo database collections |
| **RabbitMQ** | `5672` / `15672` | RabbitMQ + Management | Event queue buffering ingestion spikes |
| **MinIO** | `9000` / `9001` | MinIO Storage Engine | Object storage bucket storing failed test screenshots |
| **Grafana** | `3000` | Grafana Dashboard | LGPL telemetry dashboard for metrics, traces, and Loki logs |

---

## Database Indempotency Rule

Siphon ensures zero-duplication data processing by enforcing a composite unique indexing constraint inside MongoDB:
* Unique compound key: `execution_id + test_case_id`
* The `siphon-sink` worker pool consumes messages and performs updates using `UpdateOne(..., options.Update().SetUpsert(true))`. Duplicate keys resulting from racing or retried gRPC connections are intercepted safely and acknowledged out of the broker queue without panicking or creating duplicated rows.

---

## Setup & Running the Project

### Prerequisites
* Go v1.22+
* Node.js v18+
* Docker Desktop

### 1. Launch local Infrastructure Services
Spin up MongoDB, RabbitMQ, MinIO, and the telemetry stack (OTel Collector, Prometheus, Tempo, Loki, Grafana) via Docker Compose:
```bash
docker compose up -d
```

### 2. Run Go microservices (Concurrently in background)

Start the edge ingestion endpoint:
```bash
go run services/siphon-ingress/main.go
```

Start the sink worker pool consumer:
```bash
go run services/siphon-sink/main.go
```

Start the real-time API socket server:
```bash
go run services/siphon-stream-api/main.go
```

### 3. Start the Web Dashboard Client
Run the Vite development server for the React UI:
```bash
cd apps/siphon-glass
npm run dev
```
Open `http://localhost:5173/` in your browser.

### 4. Trigger Integration Simulation
To test the pipeline end-to-end, execute the simulated mock test suite runner. It generates a test execution run, uploads a dummy screenshot, and streams results over gRPC:
```bash
go run clients/go-hook/client.go
```
The dashboard charts and feeds will update automatically in real-time.
