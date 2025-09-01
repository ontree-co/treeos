Of course. Here are the development tickets for implementing the specification. They are designed to be self-contained, meaningful chunks of work, each with clear acceptance criteria for testing and verification.

---

## Ticket 1: Implement Backend Support for GPU Monitoring**

**Description:**
The application needs to monitor GPU utilization. This ticket covers all backend and database work required to collect, store, and expose GPU load data. The frontend will remain unchanged for now.

**Tasks:**
1.  Modify the database schema to support storing GPU load.
2.  Implement logic in the data collection service to read the system's GPU utilization.
3.  Ensure the collected data is written to the database periodically.
4.  Update the API to include the new GPU load metric in its responses.

**Acceptance Criteria:**
*   [ ] The `monitoring_stats` database table has a new column named `gpu_load` (e.g., `FLOAT` or `INTEGER`).
*   [ ] The data collection service successfully reads the GPU utilization percentage (e.g., using `nvidia-smi` for NVIDIA or another appropriate tool).
*   [ ] When inspecting the database, new rows in `monitoring_stats` are populated with a plausible integer/float value in the `gpu_load` column.
*   [ ] The API endpoint `/api/v1/status/latest` now includes a `gpu_load` field in its JSON response (e.g., `"gpu_load": 15`).
*   [ ] The API endpoint `/api/v1/status/history` now includes the `gpu_load` field for each historical data point.

---

## Ticket 2: Fix Network Monitoring Backend & Split into Upload/Download**

**Description:**
The current network monitoring is flawed. It shows constant activity because it's reading a cumulative counter instead of calculating a rate. This ticket will fix the measurement logic and split the single network metric into separate upload and download rates, including all necessary backend, database, and API changes.

**Tasks:**
1.  Modify the database schema to replace the old `network_load` column with `upload_rate` and `download_rate`.
2.  Refactor the data collection service to calculate the network rate (bytes/second) by sampling `net_io_counters` over a time interval.
3.  The service must calculate upload and download rates independently.
4.  Update the relevant API endpoints to serve the new, separate metrics.

**Acceptance Criteria:**
*   [ ] The `monitoring_stats` table no longer has the `network_load` column.
*   [ ] The `monitoring_stats` table has two new columns: `upload_rate` and `download_rate` (e.g., `BIGINT` to store bytes/second).
*   [ ] The data collection service correctly calculates and stores the upload/download rates.
*   [ ] When the server is idle, the values written to the database for `upload_rate` and `download_rate` are at or very near `0`.
*   [ ] When running a network-intensive task (like a file download), the `download_rate` shows a significant, non-cumulative value.
*   [ ] The API endpoint `/api/v1/status/latest` returns `upload_rate` and `download_rate` fields in its response, and no longer returns `network_load`.

---

## Ticket 3: Refactor Dashboard UI for New Metrics and Instant Loading**

**Description:**
This ticket covers the frontend work to display the new metrics and improve performance. The dashboard will be updated to show GPU load and split network rates. The legacy asynchronous loading mechanism will be removed in favor of a single, fast API call on page load.

**Dependencies:**
*   Ticket 1: Implement Backend Support for GPU Monitoring
*   Ticket 2: Fix Network Monitoring Backend & Split into Upload/Download

**Tasks:**
1.  Remove the old JavaScript code that performs asynchronous probing on page load.
2.  Implement a single API call to `/api/v1/status/latest` when the dashboard page loads.
3.  Add a new UI component to display the GPU load from the API response.
4.  Replace the single "Network" UI component with two separate components for "Upload" and "Download".
5.  Add formatting logic to display network rates in a human-readable format (e.g., KB/s, MB/s).

**Acceptance Criteria:**
*   [ ] The dashboard page loads and displays all stats (CPU, Memory, Disk, etc.) almost instantly, without a noticeable delay or "loading..." animations for the data.
*   [ ] A new gauge/chart/text element is present on the dashboard, correctly displaying the GPU load percentage.
*   [ ] The old network display is gone. Two new displays for "Upload" and "Download" are present.
*   [ ] The upload and download values are correctly formatted (e.g., `560 KB/s` instead of `573440`).
*   [ ] When the server is idle, the network displays show `0 B/s` or a similar near-zero value.

---

## Ticket 4: Repair and Verify the History Monitoring Feature**

**Description:**
The history page is currently blank and non-functional. This ticket covers the full-stack investigation and repair of the history feature. The goal is to make the history charts correctly display data for all monitored metrics over time.

**Tasks:**
1.  **Investigate:** Manually query the database to confirm that historical data is being logged correctly by the collection service.
2.  **API Fix:** Review and fix the `/api/v1/status/history` endpoint. Ensure its database query correctly filters by time range and returns a valid JSON array of historical data points.
3.  **Frontend Fix:** Connect the history page's charting library to the API endpoint. Ensure the frontend correctly requests data for a default time range (e.g., last 24 hours).
4.  **Integration:** Ensure the data format returned by the API matches what the frontend charting library expects (e.g., correct timestamp format, data structure).

**Acceptance Criteria:**
*   [ ] Manually calling the `/api/v1/status/history?range=24h` endpoint (or similar) with a tool like `curl` returns a JSON array with multiple data points.
*   [ ] Loading the "History" page in the web browser no longer shows a blank state.
*   [ ] The page displays charts/graphs for all monitored metrics: CPU Load, Memory Usage, Disk Usage, GPU Load, Upload Rate, and Download Rate.
*   [ ] The charts correctly visualize the data trends over the selected time period.
*   [ ] The feature is robust and displays data collected over several hours/days.