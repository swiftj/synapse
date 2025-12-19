# Synapse Visualization Server

Web-based DAG (Directed Acyclic Graph) visualization for Synapse task dependencies.

## Features

- Real-time DAG visualization using Mermaid.js
- Color-coded task status (open, in-progress, blocked, review, done)
- Auto-refresh every 5 seconds
- Shows both BlockedBy and ParentID relationships
- Clean, minimal design with embedded HTML templates

## Usage

### Basic Server

```go
package main

import (
    "log"
    "github.com/johnswift/synapse/internal/storage"
    "github.com/johnswift/synapse/internal/view"
)

func main() {
    // Initialize storage
    store := storage.NewJSONLStore(".synapse")
    store.Init()
    store.Load()

    // Start visualization server on port 8080
    server := view.NewServer(store, 8080)
    log.Fatal(server.Run())
}
```

### Run Example

```bash
go run examples/viz_server.go
```

Then open http://localhost:8080 in your browser.

## API Endpoints

- `GET /` - Serves the visualization HTML page
- `GET /api/synapses` - Returns all synapses as JSON
- `GET /api/ready` - Returns ready synapses as JSON

## Graph Visualization

### Node Colors

- White: Open tasks
- Yellow: In-progress tasks
- Gray: Blocked tasks
- Sky Blue: Tasks in review
- Light Green: Completed tasks

### Edge Types

- Solid arrows (`-->`) represent BlockedBy dependencies (blocker → blocked task)
- Dotted arrows (`-.->`) represent ParentID relationships (parent → child task)

## Implementation Details

The server uses:
- `embed.FS` to embed HTML templates directly in the binary
- Mermaid.js CDN for client-side graph rendering
- Standard library `net/http` for the web server
- JSON API for data exchange

The Mermaid graph is generated client-side from the JSON API for better interactivity and to reduce server load.
