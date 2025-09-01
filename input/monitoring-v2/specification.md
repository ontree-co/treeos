Of course. Based on your detailed input, here is a full specification document outlining the problems, requirements, and an action plan to refactor and enhance your home server monitoring application.

---

## **Specification: Home Server Monitoring App V2 Refactor**

### **1.0 Overview**

This document outlines the necessary changes to refactor the home server monitoring application. The primary goals are to improve performance by leveraging the new database backend, fix incorrect data measurements, implement new monitoring metrics (GPU load, split network rates), and repair the non-functional history feature. The project will involve modifications to the backend data collection service, the database schema, the API, and the frontend user interface.

### **2.0 Current System Analysis & Problem Statement**

The current implementation suffers from several issues, stemming from a mix of legacy code and newly introduced, but incomplete, features.

*   **P1: Outdated Frontend Loading Mechanism:** The frontend still uses an asynchronous loading mechanism designed to work around slow system probing. With a database now available, this is inefficient and unnecessary. The goal is to display the latest data instantly on page load by reading from the database.
*   **P2: Incomplete GPU Monitoring:** A placeholder for GPU load has been added, but the backend logic for reading, storing, and displaying this metric is not implemented.
*   **P3: Incorrect Network Monitoring:**
    *   The split of network statistics into "Upload" and "Download" is not correctly implemented.
    *   The current measurement is flawed. It shows constant network activity even when the server is idle. This strongly suggests it's reading a **cumulative byte counter** (total bytes since boot) rather than calculating the **rate** (bytes per second).
*   **P4: Non-Functional History Feature:** The history view is not displaying any data, despite the server running for days. This could be due to one or more of the following:
    *   The data collection service is not writing historical data to the database.
    *   The data is being written, but in a format the frontend cannot parse (e.g., incorrect timestamp format).
    *   The API endpoint for fetching history is broken or has a faulty query.
    *   The frontend is failing to request or render the historical data correctly.

### **3.0 Scope of Work & Requirements**

The work is divided into four main components: Backend/Data Collection, Database, API, and Frontend.

#### **3.1 Backend / Data Collection Service**

This service is responsible for periodically probing the system and writing the metrics to the database.

*   **Requirement 3.1.1: Implement GPU Load Measurement**
    *   Investigate and implement a reliable method to read GPU utilization percentage.
        *   **For NVIDIA GPUs:** Use the `nvidia-smi` command-line tool. A typical command would be `nvidia-smi --query-gpu=utilization.gpu --format=csv,noheader,nounits`.
        *   **For AMD GPUs:** Use tools like `radeontop` or read from the sysfs filesystem (e.g., `/sys/class/drm/card0/device/gpu_busy_percent`).
    *   The chosen method must be integrated into the data collection script. The output (a percentage value) must be parsed and stored.

*   **Requirement 3.1.2: Correct Network Rate Measurement**
    *   The service must be modified to calculate the **upload and download rates in bytes per second (B/s)**.
    *   This is **not** a single reading. It requires sampling the total bytes sent and received at two different points in time (t1 and t2) and calculating the difference over the time delta: `rate = (bytes_t2 - bytes_t1) / (t2_seconds - t1_seconds)`.
    *   **Recommendation:** Use a well-maintained system utility library like `psutil` (for Python), which has built-in functions (`psutil.net_io_counters()`) to handle this calculation correctly and in a cross-platform way.
    *   The service will now produce two distinct values: `upload_rate` and `download_rate`.

*   **Requirement 3.1.3: Ensure Consistent Data Logging**
    *   The main loop of the collection service must be verified to run at a consistent interval (e.g., every 30 seconds).
    *   Each run must collect all metrics (CPU, GPU, Memory, Disk, Upload Rate, Download Rate) and write them as a **single new row** into the database table, along with a current timestamp.

#### **3.2 Database (DB)**

The database schema must be updated to support the new metrics.

*   **Requirement 3.2.1: Modify Monitoring Table Schema**
    *   Let's assume the table is named `monitoring_stats`.
    *   **Remove** the old, generic `network_load` column.
    *   **Add** the following new columns:
        *   `gpu_load` (Type: `FLOAT` or `INTEGER`, to store percentage 0-100)
        *   `download_rate` (Type: `BIGINT`, to store bytes/second)
        *   `upload_rate` (Type: `BIGINT`, to store bytes/second)
    *   Ensure the `timestamp` column is of an appropriate type (`DATETIME`, `TIMESTAMP WITH TIME ZONE`, or `INTEGER` for Unix epoch) and is being populated correctly.

#### **3.3 API**

The API endpoints that serve data to the frontend need to be created or modified.

*   **Requirement 3.3.1: Create/Update 'Get Latest' Endpoint**
    *   An endpoint, e.g., `/api/v1/status/latest`, must be implemented.
    *   This endpoint will execute a single database query to fetch the most recent row from the `monitoring_stats` table (e.g., `SELECT * FROM monitoring_stats ORDER BY timestamp DESC LIMIT 1;`).
    *   It should return a JSON object containing all the latest metrics, including the new `gpu_load`, `download_rate`, and `upload_rate`.

*   **Requirement 3.3.2: Fix and Verify 'Get History' Endpoint**
    *   An endpoint, e.g., `/api/v1/status/history`, must be fixed.
    *   It should accept query parameters for a time range (e.g., `?start_time=<timestamp>&end_time=<timestamp>`).
    *   **Investigation Plan:**
        1.  **Verify Data Existence:** Manually query the database to confirm that multiple rows of data exist. Run `SELECT COUNT(*) FROM monitoring_stats;`. If the count is low, the problem is in the data collection service (see 3.1).
        2.  **Inspect the API Query:** Review the SQL query used by this endpoint. It is likely failing to filter by the time range correctly. Ensure timestamps are being compared properly.
        3.  **Test the Endpoint:** Call the API directly using a tool like `curl` or Postman to see what data (or error) is returned.
    *   The endpoint must return a JSON array of data points, each corresponding to a row in the database within the requested time range.

#### **3.4 Frontend (Web UI)**

The user interface must be updated to reflect all backend changes.

*   **Requirement 3.4.1: Remove Asynchronous Probing**
    *   On the main dashboard page, remove the old JavaScript code that initiated asynchronous requests to probe the system.
    *   Replace it with a single, simple API call on page load to the `/api/v1/status/latest` endpoint.
    *   The data from this single call will be used to populate all the monitoring gauges and text displays instantly.

*   **Requirement 3.4.2: Add New UI Components**
    *   Add a new visual element (e.g., a gauge, progress bar, or percentage text) to the dashboard to display the `gpu_load`.
    *   Modify the existing network component. Instead of one "Network" display, create two: "Download" and "Upload".
    *   These components should display the `download_rate` and `upload_rate` values. Include logic to format the values from B/s into a human-readable format (e.g., KB/s, MB/s).

*   **Requirement 3.4.3: Fix History Page**
    *   Connect the history page's charting library to the `/api/v1/status/history` endpoint.
    *   On page load, it should fetch data for a default range (e.g., the last 24 hours).
    *   Ensure the data received from the API is correctly parsed and fed into the charting library to render the graphs for all metrics over time.

### **4.0 Acceptance Criteria**

The project will be considered complete when all of the following criteria are met:

1.  **Instant Loading:** The main monitoring dashboard loads instantly, displaying the latest server stats without any visible "loading..." state for the data itself.
2.  **GPU Display:** The dashboard correctly displays the current GPU load as a percentage.
3.  **Accurate Network Rates:**
    *   The UI shows two separate stats: "Upload" and "Download".
    *   The rates are displayed in a user-friendly format (e.g., "1.2 MB/s").
    *   When the server is idle, both rates show 0 B/s or a value very close to zero.
4.  **Functional History:** The "History" page successfully loads and displays charts showing the trend of all monitored metrics (CPU, GPU, Memory, etc.) over the last 24 hours.
5.  **Data Integrity:** A manual check of the `monitoring_stats` table in the database shows new rows being added periodically with plausible data in all columns.