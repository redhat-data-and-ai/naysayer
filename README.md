# GitLab Merge Request Bot Backend

This application is a backend service for handling GitLab merge request (MR) events. It is built using the [Fiber](https://gofiber.io/) web framework and provides a webhook endpoint to process GitLab events, such as merge request creation. The service is designed to be extensible, allowing custom handlers to process specific events.

---

## Features

- **GitLab Webhook Integration**: Processes GitLab merge request events via a `/gitlab-webhook` endpoint.
- **Customizable Handlers**: Supports a registry of handlers to process specific event types.
- **GitLab API Integration**: Includes a lightweight GitLab API client for interacting with GitLab (e.g., assigning reviewers to merge requests).
- **Dual Licensing**: Licensed under both Apache 2.0 and MIT licenses for flexibility.

---

## How It Works

1. **Webhook Endpoint**: The `/gitlab-webhook` endpoint listens for GitLab events (e.g., `merge_request` events).
2. **Event Parsing**: The payload is parsed into a `MergeRequestEvent` struct.
3. **Handler Registry**: Handlers registered for specific event types (e.g., `merge_request`) are executed sequentially.
4. **GitLab API Client**: The `GitLabClient` is used to interact with GitLab, such as assigning reviewers to merge requests.

---

## Setup and Usage

### Prerequisites

- Go 1.23 or later
- GitLab instance with API access
- GitLab personal access token

### How to use

This project works as backend for the bot. It implements the web server required to handle the webhook events. So that the implementations can focus on the business logic.

Create a new go project. `main.go` would look like this -

```go
package main

import (
	"log"

	backendApp "github.com/redhat-data-and-ai/gitlab-bot-backend/app"
	"<your-go-module>/handlers"
)

func main() {
	// Create a new Fiber app
	handlerRegistry := map[string][]backendApp.Handler{
		"merge_request": {
			&handlers.MRCreatedHandler{},
		},
	}

	app := backendApp.SetupApp(handlerRegistry)

	// Start the server
	log.Fatal(app.Listen(":3000"))
}
```

And create a folder called `handlers` and create as many handlers you need. E.g. `mr_created_handler.go` can look like this -

```go
package handlers

import (
	"log"

	backendApp "github.com/redhat-data-and-ai/gitlab-bot-backend/app"
	"github.com/redhat-data-and-ai/gitlab-bot-backend/library"
)

type MRCreatedHandler struct {
	backendApp.Handler
}

func (h *MRCreatedHandler) Handle(event backendApp.MergeRequestEvent, gitLabClient library.GitLabClient) error {
	// Handle the merge request created event
	// You can access the event data and perform actions here
	// For example, you can log the event or send a notification

	// Example: Log the event
	log.Printf("Merge request created by %s with action %s", event.User.Name, event.ObjectAttributes["action"])

	// Perform any other actions you need to do for this event

	return nil
}
```

And run the server -

```bash
go run main.go
```

## License

This project is dual-licensed under the Apache 2.0 and MIT licenses. You may choose either license to use this software. See [LICENSE](LICENSE) and [LICENSE-MIT](LICENSE-MIT) for details.
