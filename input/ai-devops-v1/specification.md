Of course. Integrating a chat-based UI is a brilliant way to personify the agent and make its 
## **Tech Spec: AI SysAdmin Agent v1.0 (with Chat UI)**

### **1. Project Goal & Philosophy**

The primary goal of this project is to create a "self-managing" component for a Go-based homeserver application. This component, an "AI SysAdmin Agent," will periodically monitor the health and performance of the server and its managed applications, report its findings, and suggest or execute corrective actions.

The user will interact with the agent's findings through a **chat-like interface**, where the agent posts regular status updates for each application, creating an ongoing, human-readable log of events and health checks.

**Core Philosophies for v1.0:**

*   **Conversational UI:** All agent activities and findings should be presented as messages in a chat log, making the system's state easy to understand at a glance.
*   **Modular & Self-Contained Apps:** Each managed application (defined by a `docker-compose.yml` file) should have its configuration and metadata co-located within a single directory. This makes apps portable and easy to manage.
*   **Declarative & GitOps-Ready:** The desired state of the server will be defined in version-controllable text files. While a full GitOps workflow is a v2 goal, the structure will be designed for it from day one.
*   **Safety Through Bounded Actions:** The AI (LLM) will act as a reasoning engine, not an executor. It can only recommend actions from a small, predefined set of safe functions built into the Go application.
*   **Iterative Development:** This v1 spec intentionally omits complex features like automated backups and encrypted secrets management, but the architecture is explicitly designed to accommodate them seamlessly in the future.

### **2. User Interface (UI): The Application Chat Log**

The primary user interface for interacting with the agent's findings will be a dedicated "chat log" view for each managed application.

**A. UI Mockup / Description:**
Imagine a view similar to a messaging app (like Discord or Slack).
*   On the left, a list of managed applications (e.g., "Nextcloud", "Plex").
*   Selecting an app opens its dedicated chat log on the right.
*   The agent is personified as a "System Agent" or "Homeserver Bot" with an avatar.
*   Each time the agent runs its check, it posts a new message into the chat for that specific app.

**B. Message Types & Appearance:**
Messages will be color-coded to convey status instantly:
*   **Green Message (OK):** A simple, concise message for successful checks.
    *   *Example:* "✅ **All systems nominal.** All services are running and responding as expected. (Checked at 10:05 AM)"
*   **Yellow Message (Warning):** A more detailed message for non-critical issues. It should be expandable to show more detail.
    *   *Example:* "⚠️ **Warning: High restart count detected.** The 'plex' service has restarted 5 times in the last hour. The application is still online, but you should investigate the logs for potential instability. (Checked at 10:10 AM)"
*   **Red Message (Critical):** A prominent, detailed message for critical failures.
    *   *Example:* "🚨 **CRITICAL: Application is down!** The 'db' service for Nextcloud is not running. The application is offline. I have attempted to restart the service. Please check the system immediately. (Checked at 10:15 AM)"

**C. Data Persistence:**
*   The chat messages must be persisted in the application's database. This allows users to scroll back and see the history of an application's health over time.
*   Each message in the database will be associated with an `app_id` and will store its content, severity level (`OK`, `WARNING`, `CRITICAL`), and a timestamp.

### **3. Filesystem & Repository Structure**

The entire configuration will live in a single Git repository. For v1, this repository will manage one server. The structure is designed to scale to multiple servers later.

```
/opt/homeserver-config/
├── .git/
├── .gitignore
└── apps/
    ├── nextcloud/
    │   ├── docker-compose.yml   # The app's Docker Compose definition
    │   ├── app.homeserver.yaml  # The app's metadata for our Go agent
    │   └── .env                 # (On server ONLY, NOT in Git) App secrets
    │
    └── plex/
        ├── docker-compose.yml
        ├── app.homeserver.yaml
        └── .env                 # (On server ONLY, NOT in Git)
```

**`.gitignore` File (Crucial for Security):**
This file MUST be in the root of the repository to prevent accidentally committing secrets.

```gitignore
# Ignore all .env files, in any subdirectory.
**/.env

# Ignore any potential future secrets files
**/*.secrets

# Ignore the data directories that will be added for v2 backups
**/data/
```

### **4. Configuration Schema: `app.homeserver.yaml`**

This file is the "source of truth" for each application. It provides the necessary metadata for the Go agent to perform its checks.

```yaml
# The unique identifier for the application. Should match the parent directory name.
id: "nextcloud"

# A human-friendly name for the application.
name: "Nextcloud Suite"

# The primary service container whose health is most indicative of the app's overall status.
primary_service: "app"

# The name or tag of the corresponding monitor in your Uptime Kuma instance.
uptime_kuma_monitor: "nextcloud-web"

# A list of all services that are expected to be running for this app to be considered healthy.
# These names must match the service names in the docker-compose.yml file.
expected_services:
  - "app"
  - "db"
  - "redis"
```

### **5. The Go Orchestration Loop (Core Logic)**

The Go application will run a scheduled job (e.g., every 5 minutes) using a cron library like `github.com/robfig/cron`. Each job execution will perform the following steps:

1.  **Discover Apps:** Scan the `/opt/homeserver-config/apps/` directory by searching for all `app.homeserver.yaml` files. This creates the list of apps to check.
2.  **Collect General Server Data:** Gather high-level server metrics from Node Exporter.
3.  **Collect Per-App Data:** For each discovered app:
    *   Read its `app.homeserver.yaml`.
    *   Query the Docker daemon for the status of its `expected_services`.
    *   Query the Uptime Kuma API for its external health.
    *   Fetch and pre-process recent logs from each service container.
4.  **Build System Snapshot:** Aggregate all collected data into a single, large JSON object (the "System Snapshot").
5.  **Call LLM:** Send the System Snapshot JSON to the LLM API within a carefully crafted prompt.
6.  **Parse LLM Response:** Receive and parse the JSON response from the LLM, which contains its analysis and recommended actions.
7.  **Persist & Execute Actions:** Based on the parsed response:
    *   **Persist Chat Message:** Generate the appropriate chat message based on the LLM's `summary` and `overall_status`. Save this message to the database, linked to the specific `app_id`.
    *   **Execute Actions:** Execute the recommended safe actions (e.g., restart a container).

### **6. Data Collection Layer**

The Go application will need functions to gather data from these sources:

*   **Node Exporter (Server Health):**
    *   **Method:** HTTP GET request to `http://localhost:9100/metrics`.
    *   **Library:** Use a Go Prometheus client library (e.g., `github.com/prometheus/client_golang`) to parse the text response.
    *   **Key Metrics:** `node_cpu_seconds_total`, `node_memory_MemTotal_bytes`, `node_memory_MemAvailable_bytes`, `node_filesystem_avail_bytes`.

*   **Uptime Kuma (External Health):**
    *   **Method:** HTTP GET request to the Uptime Kuma API status endpoint.
    *   **Data:** Parse the JSON response to find the status of the monitor specified in `uptime_kuma_monitor`.

*   **Docker Daemon (Container Status & Logs):**
    *   **Library:** The official Docker Go SDK (`github.com/docker/docker/client`).
    *   **Status:** Use `client.ContainerList` with a filter for the Docker Compose project name to find all containers for an app. Check their `State` and `Status`.
    *   **Logs:** Use `client.ContainerLogs`. Set the `Since` option to `5m` (or your check interval) to get only recent logs.
    *   **Log Pre-processing:** **Do not send raw logs to the LLM.** Scan the log strings in Go for keywords (`ERROR`, `FATAL`, `Exception`, `failed`). Create a summary count and collect a few sample error lines.

### **7. Reasoning Layer (LLM Interaction)**

This is the interface between your Go app and the AI.

**A. Data Structure Sent to LLM (`SystemSnapshot`)**

```go
// The full snapshot sent to the LLM
type SystemSnapshot struct {
    Timestamp      time.Time    `json:"timestamp"`
    ServerHealth   ServerHealth `json:"server_health"`
    AppStatuses    []AppStatus  `json:"app_statuses"`
}
type ServerHealth struct {
    CPUUsagePercent    float64 `json:"cpu_usage_percent"`
    MemoryUsagePercent float64 `json:"memory_usage_percent"`
    DiskUsagePercent   float64 `json:"disk_usage_percent"`
}
type AppStatus struct {
    AppName         string        `json:"app_name"`
    DesiredState    DesiredState  `json:"desired_state"`
    ActualState     ActualState   `json:"actual_state"`
}
type DesiredState struct {
    ExpectedServices  []string `json:"expected_services"`
}
type ActualState struct {
    UptimeKumaStatus  string             `json:"uptime_kuma_status"` // "UP", "DOWN"
    Services          []ServiceStatus    `json:"services"`
}
type ServiceStatus struct {
    Name           string      `json:"name"`
    Status         string      `json:"status"` // "running", "exited", "restarting"
    RestartCount   int         `json:"restart_count"`
    LogSummary     LogSummary  `json:"log_summary"`
}
type LogSummary struct {
    ErrorsFound   int      `json:"errors_found"`
    SampleErrorLines []string `json:"sample_error_lines"`
}
```

**B. LLM Prompt Template**

```text
You are an expert, helpful, and cautious Site Reliability Engineer AI for a homeserver. Your task is to analyze the following system snapshot by comparing each app's desired state with its actual state. Identify any deviations or problems. Your output will be used to generate a short status message for a user in a chat interface.

The current time is: {{current_time}}.

Here is the system data in JSON format:
{{system_snapshot_json}}

Analyze the data and respond ONLY in the following JSON format. Do not add any explanation before or after the JSON block.

{
  "overall_status": "ALL_OK | WARNING | CRITICAL",
  "summary": "A one-sentence, human-readable summary of the server's state. This will be the main text in the chat message. Make it concise and clear.",
  "analysis": [
    {
      "component": "Component Name (e.g., 'Server Health', 'App: Nextcloud')",
      "status": "OK | WARN | FAIL",
      "finding": "A brief description of the finding for this component. This can be used for an expandable 'details' section in the UI."
    }
  ],
  "recommended_actions": [
    {
      "action_key": "A predefined action key from the allowed list.",
      "parameters": { "key": "value" },
      "justification": "Why this action is recommended."
    }
  ]
}

The ONLY allowed values for 'action_key' are:
- "PERSIST_CHAT_MESSAGE" (parameters: {"app_id": "string", "status": "string", "message": "string"})
- "RESTART_CONTAINER" (parameters: {"container_name": "string"})
- "NO_ACTION" (parameters: {})

A CRITICAL issue exists if a service is missing, exited, or Uptime Kuma reports 'DOWN'. A WARNING exists if logs show errors or restart counts are high. If everything is fine, return 'ALL_OK'. For every check, a PERSIST_CHAT_MESSAGE action must be recommended.
```
*(Note: The `SEND_NOTIFICATION` action has been replaced by `PERSIST_CHAT_MESSAGE` to align with the new UI-first approach. External notifications can be added back later as a separate action.)*

### **8. Action & Communication Layer**

The Go application will parse the LLM's response and use a `switch` statement on the `action_key`.

*   **`NO_ACTION`:** Log that a successful check was performed.
*   **`PERSIST_CHAT_MESSAGE`:** This is now the primary communication method.
    *   **Action:** Take the parameters (`app_id`, `status`, `message`) from the LLM recommendation.
    *   **Logic:** Write a new entry to the `chat_messages` table in the application's database.
    *   **Real-time Update (Optional v1.1):** Use WebSockets to push the new message to any connected web clients in real-time. For v1, a simple page refresh will suffice.
*   **`RESTART_CONTAINER`:** Use the Docker Go SDK `client.ContainerRestart` function to restart the specified container. Log this action clearly. After executing, a follow-up chat message should be posted (e.g., "I have attempted to restart the 'plex' container.").

### **9. Secrets Management (v1 Strategy)**

For v1, secrets will be managed manually on the server to prioritize security over automation.

*   **`.env` Files:** Each application that needs secrets will have a `.env` file in its directory (e.g., `apps/nextcloud/.env`).
*   **`.gitignore`:** The `**/.env` entry in `.gitignore` is **mandatory** to prevent these files from ever entering the Git repository.
*   **`docker-compose.yml`:** Use the `env_file: [ './.env' ]` directive in your compose files to instruct Docker to load secrets from this file.
*   **Disaster Recovery:** The contents of all `.env` files must be manually backed up by the user in a secure location, such as a password manager. This is a manual v1 process.

### **10. Path to v2 (Future-Proofing)**

This design paves the way for future enhancements with minimal refactoring:

*   **Backup Readiness:** Create a `data/` subdirectory within each app's folder (e.g., `apps/nextcloud/data/`) and add it to `.gitignore`. Use relative bind mounts in your `docker-compose.yml` to point to it (e.g., `volumes: ['./data/db:/var/lib/postgresql/data']`). This co-locates app data, making it trivial for a future backup script (like Restic) to find and back up.
*   **Secrets Management Readiness:** The manual creation of `.env` files will be replaced by a single command in a deployment script: `sops --decrypt secrets.sops.yaml > .env`. The rest of the application logic and Docker Compose files do not need to change.