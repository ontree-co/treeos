Of course. Here is the complete and unabbreviated set of tickets, updated to include the new conversational UI and its backend requirements. This plan is structured for clarity and individual testability.

---

## Ticket 1: Database and Backend Models for Conversational UI**

**Title:** `feature(agent-ui): Implement Database Schema and Backend Models for Agent Chat History`

**Description:**
This foundational ticket establishes the persistence layer for the new conversational UI. Before any agent logic is built, we need a way to store and retrieve the chat messages that the agent will generate. This involves creating the necessary database table and the corresponding Go data model.

**Tasks:**
1.  **Define Database Schema:** Create a new database migration for a `chat_messages` table. The table should include the following columns:
    *   `id` (Primary Key, e.g., UUID or auto-incrementing integer)
    *   `app_id` (String, Foreign Key to an applications table if one exists, otherwise an indexed string)
    *   `timestamp` (Timestamp with timezone)
    *   `status_level` (String, e.g., 'OK', 'WARNING', 'CRITICAL')
    *   `message_summary` (Text, for the main chat message)
    *   `message_details` (Text or JSONB, for the expandable details from the LLM's `analysis` section)
2.  **Create Go Data Model:** In the backend, create a Go struct `ChatMessage` that directly maps to this database table.
3.  **Implement Data Access Layer:** Create a "repository" or "store" service with functions for interacting with the `chat_messages` table:
    *   `CreateChatMessage(message ChatMessage) error`
    *   `GetChatMessagesForApp(appID string, limit int, offset int) ([]ChatMessage, error)`
4.  **Create API Endpoint:** Expose a new backend API endpoint, for example `GET /api/apps/{appID}/chat`, which uses the `GetChatMessagesForApp` function to return the message history for a specific application as JSON.

**Acceptance Criteria:**
*   [ ] The database migration for the `chat_messages` table is written and applies successfully.
*   [ ] The `ChatMessage` Go struct is defined.
*   [ ] The Data Access Layer functions for creating and retrieving messages are implemented.
*   [ ] The new API endpoint `GET /api/apps/{appID}/chat` is functional and returns a list of chat messages for a given application ID.
*   [ ] **Unit tests for the Data Access Layer are passing.** These tests should verify the create and retrieve logic, preferably against a test database.

---

## Ticket 2: Frontend UI for Application Chat Log**

**Title:** `feature(agent-ui): Implement Frontend Chat Log UI for Displaying Agent Messages`

**Description:**
This ticket focuses exclusively on building the visual component for the conversational UI. It will consume the API endpoint created in the previous ticket to display a read-only chat history for each application. The goal is to create an intuitive and visually clear interface for the user.

**Tasks:**
1.  **Create Chat View Component:** Build a new frontend view/component (e.g., in React, Vue, Svelte) that will serve as the chat interface. This view will be displayed when a user selects an application.
2.  **API Integration:** The component should make an API call to `GET /api/apps/{appID}/chat` when it loads to fetch the message history for the selected application.
3.  **Render Messages:** Implement the logic to iterate over the fetched messages and render them.
    *   Each message should display the agent's avatar, the timestamp, and the message content.
    *   Implement conditional styling based on the `status_level` of each message (e.g., green background for 'OK', yellow for 'WARNING', red for 'CRITICAL').
4.  **Layout and Styling:** Style the component to look like a modern chat application. Ensure it is readable and provides a clear historical log. For now, this is a read-only view.

**Acceptance Criteria:**
*   [ ] A new chat log view is present in the UI for each application.
*   [ ] The UI correctly fetches and displays the chat history from the backend API.
*   [ ] Messages are correctly color-coded based on their status level.
*   [ ] The UI gracefully handles the case where there are no messages yet for an application.

---

## Ticket 3: Foundational Models & Configuration Provider for Agent Logic**

**Title:** `feature(agent): Implement Core Data Models and Configuration Provider for AI Agent`

**Description:**
This ticket lays the entire foundation for the AI agent's logic. We need to define the data structures that will be used to represent the state of the system and create a reliable way to load the on-disk configuration that defines what the agent should monitor. This work is purely backend and does not involve any external API calls yet.

**Tasks:**
1.  **Define Go Structs:** In a new `agent/` package, create the Go structs that will represent the entire system snapshot. This includes `SystemSnapshot`, `ServerHealth`, `AppStatus`, `DesiredState`, `ActualState`, `ServiceStatus`, and `LogSummary` as detailed in the tech spec. Add appropriate `json` and `yaml` tags for serialization.
2.  **Implement `AppConfig` Provider:** Create a `config` sub-package or service.
    *   Define the `AppConfig` struct based on the `app.homeserver.yaml` schema (`id`, `name`, `primary_service`, etc.).
    *   Implement a `FilesystemProvider` service that has a `GetAll()` method.
    *   This method will scan a given root directory (e.g., `/opt/homeserver-config/apps/`) for all `app.homeserver.yaml` files, parse them using a YAML library, and return a slice of `AppConfig` structs.
    *   Include robust error handling for missing directories, unreadable files, or malformed YAML.

**Acceptance Criteria:**
*   [ ] All required structs are defined in the `agent/` package.
*   [ ] The `FilesystemProvider.GetAll()` method successfully discovers and parses all valid `app.homeserver.yaml` files in a test directory structure.
*   [ ] The provider returns clear errors for I/O issues or YAML parsing failures.
*   [ ] **Unit tests for the `FilesystemProvider` are passing.** Tests should cover success cases, malformed YAML files, and missing files.

---

## Ticket 4: Multi-Source Data Collection Service**

**Title:** `feature(agent): Implement Multi-Source Data Collection Service`

**Description:**
This ticket involves creating a cohesive service that populates the `SystemSnapshot` struct defined in Ticket 3. This service will be responsible for gathering all the "actual state" data from various sources: Docker, Uptime Kuma, and Node Exporter. It will use the `AppConfig` structs (from Ticket 3) as input to know what to look for.

**Tasks:**
1.  **Create Collector Service:** In the `agent/` package, create a `Collector` service. Its primary method could be `CollectSystemSnapshot(configs []AppConfig) (*SystemSnapshot, error)`.
2.  **Docker Collector:**
    *   Integrate with the existing Docker Go SDK client.
    *   For each app, list its containers and match them against the `expected_services`.
    *   Gather status (`running`, `restarting`), restart counts, etc.
    *   Implement the log pre-processing logic: fetch recent logs and perform a keyword search to generate the `LogSummary` without storing the raw logs.
3.  **Uptime Kuma Collector:**
    *   Implement a simple HTTP client to make a GET request to the Uptime Kuma API.
    *   Parse the JSON response and extract the status for the monitor name specified in the `AppConfig`.
4.  **Node Exporter Collector:**
    *   Implement an HTTP client to scrape the `/metrics` endpoint of Node Exporter.
    *   Use a Prometheus client library to parse the text format and extract key CPU, Memory, and Disk metrics.

**Acceptance Criteria:**
*   [ ] The `Collector` service can successfully populate all fields of the `SystemSnapshot` struct.
*   [ ] The log pre-processing logic correctly identifies error keywords and creates an accurate `LogSummary`.
*   [ ] **Unit tests for each collector function are passing.** These tests will heavily use mocking:
    *   Mock the Docker SDK interface to return predefined container states and logs.
    *   Mock the HTTP client to return sample JSON from Uptime Kuma and sample text from Node Exporter.

---

## Ticket 5: LLM Reasoning and Action Generation Service**

**Title:** `feature(agent): Implement LLM Reasoning Service for Analysis and Action Generation`

**Description:**
This ticket creates the "brain" of the agent. It will take the fully populated `SystemSnapshot` from the Collector Service, serialize it, wrap it in a carefully constructed prompt, send it to an LLM API, and parse the structured JSON response back into actionable Go objects.

**Tasks:**
1.  **Define LLM Response Structs:** Create Go structs that mirror the JSON output format expected from the LLM (e.g., `LLMResponse`, `AnalysisItem`, `RecommendedAction`).
2.  **Implement Prompt Templating:** Create a function that takes the `SystemSnapshot` object and injects its JSON representation into the master prompt template defined in the tech spec.
3.  **Create LLM Client:**
    *   Implement a client service to handle the API call to your chosen LLM (e.g., OpenAI).
    *   This client will send the generated prompt and handle the HTTP request/response cycle.
    *   Make the API key and endpoint URL configurable.
4.  **Implement Response Parser:** Create a function that takes the raw response body from the LLM and unmarshals it into the `LLMResponse` struct. It must be resilient to potential LLM failures, such as returning non-JSON text or malformed JSON.

**Acceptance Criteria:**
*   [ ] The prompt templating function correctly generates the full prompt string.
*   [ ] The LLM client can successfully communicate with the LLM API.
*   [ ] The response parser correctly decodes valid JSON and gracefully handles errors (API errors, invalid JSON).
*   [ ] **Unit tests are passing.**
    *   Test the prompt generation logic.
    *   Test the response parser with mock valid JSON strings, malformed JSON, and empty responses.
    *   The LLM API call itself should be mocked to avoid real API calls during tests.

---

## Ticket 6: Orchestrator, Action Executor, and Cron Job Integration**

**Title:** `feature(agent): Implement Main Orchestration Cron Job and Action Executor`

**Description:**
This is the final integration ticket that brings all the previous components together. It involves creating the main cron job that will run periodically. This job will orchestrate the flow of data between the services and execute the final, LLM-recommended actions, including persisting the results to the chat UI.

**Tasks:**
1.  **Create Orchestrator Service:** Create a top-level `agent.RunCheck()` function.
2.  **Orchestration Logic:** This function will:
    *   Call the `ConfigProvider` to get all app configs.
    *   Pass the configs to the `DataCollector` to get the system snapshot.
    *   Pass the snapshot to the `LLMReasoningService` to get an action plan.
    *   Pass the action plan to the `ActionExecutor`.
3.  **Implement Action Executor:**
    *   Create a function that takes the `RecommendedAction` slice from the LLM response.
    *   Use a `switch` statement on the `action_key`.
    *   For `PERSIST_CHAT_MESSAGE`, call the `CreateChatMessage` function from the Data Access Layer (Ticket 1).
    *   For `RESTART_CONTAINER`, call the existing Docker restart function from your app.
    *   For `NO_ACTION`, do nothing.
4.  **Integrate Cron Job:** Hook the `agent.RunCheck()` function into a cron scheduler (e.g., `robfig/cron`) within your main application startup.

**Acceptance Criteria:**
*   [ ] The cron job triggers the `RunCheck` function at the configured interval.
*   [ ] The data flows correctly through all the services in sequence.
*   [ ] After a check runs, a new message appears in the database `chat_messages` table.
*   [ ] The `ActionExecutor`'s switch statement correctly routes action keys to the appropriate backend functions.
*   [ ] **Unit tests are passing.** Using dependency injection and interfaces is critical here. The tests should pass mocked-up structs between mocked services to verify that the orchestration logic and `ActionExecutor` switch work as intended without making real API or Docker calls.

---

## Ticket 7: Manual Quality Assurance and End-to-End Validation**

**Title:** `quality-assurance(agent): Perform Manual End-to-End Walkthrough and Validation of AI Agent and Chat UI`

**Description:**
This is a manual testing ticket to verify the entire feature works as a whole in a live, staging environment. The goal is to simulate various server states and confirm the agent behaves as expected and that the UI reflects these states correctly.

**Test Plan:**
1.  **Setup:** Configure a staging environment with 2-3 sample applications, Node Exporter, and Uptime Kuma.
2.  **Test Case 1 (All OK):**
    *   Ensure all containers are running and healthy.
    *   Trigger the agent check.
    *   **Expected:** A new green message appears in the chat UI for each app, stating that everything is nominal.
3.  **Test Case 2 (Container Down):**
    *   Manually stop a container (e.g., `docker stop nextcloud-db-1`).
    *   Trigger the agent check.
    *   **Expected:** A new red, critical message appears in the app's chat log, correctly identifying the stopped container. The log should show that a restart action was attempted.
4.  **Test Case 3 (Log Errors):**
    *   Inject error messages into a container's log (`docker exec my-container logger "FATAL: Something went wrong"`).
    *   Trigger the agent check.
    *   **Expected:** A new yellow, warning message appears in the app's chat log, mentioning the log errors.
5.  **Test Case 4 (LLM Malformed Response):**
    *   (Requires temporarily modifying the code to simulate a bad response from the LLM).
    *   **Expected:** The system does not crash. It logs an error about being unable to parse the LLM response. No new message appears in the chat UI, or a system-level error message is posted.

**Acceptance Criteria:**
*   [ ] All test cases in the plan have been executed and their actual results match the expected results.
*   [ ] A sign-off from the project owner confirms the feature is behaving as intended.

---

## Ticket 8: End-to-End Test Automation with Playwright**

**Title:** `test(agent): Implement End-to-End Playwright Tests for AI Agent Chat UI`

**Description:**
To ensure long-term stability and prevent regressions, we need to automate the end-to-end testing. This ticket involves writing Playwright tests that interact with the application's web UI to verify that the backend agent's findings are correctly posted and displayed in the chat interface.

**Tasks:**
1.  **Setup Playwright Environment:** Configure the Playwright test runner within the project's frontend directory or a separate test directory.
2.  **Write Test Scenarios:** Create test files that mirror the manual Quality Assurance scenarios.
    *   **Scenario 1 (All OK):** The test should trigger an agent run via a test-only API hook, navigate to an app's chat view, and assert that the latest message is present and has the correct 'green' styling and content.
    *   **Scenario 2 (Container Down):** The test should use a backend hook to stop a container, trigger the agent run, reload the chat UI, and assert that the latest message is a 'red' critical alert.
    *   **Scenario 3 (History Check):** The test should verify that older messages are still present and that the chat log scrolls correctly.

**Acceptance Criteria:**
*   [ ] Playwright tests are written for the key success and failure scenarios of the chat UI.
*   [ ] The tests run successfully against a staging environment where the agent's behavior can be controlled or mocked.
*   [ ] The test suite is integrated into the Continuous Integration / Continuous Deployment pipeline to run on new commits.