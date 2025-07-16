---
sidebar_position: 3
---

# Monitoring

OnTree includes comprehensive system monitoring to help you track resource usage, identify bottlenecks, and ensure optimal performance of your applications.

## Dashboard Overview

The monitoring system displays four key metrics directly on the main dashboard:

- **CPU Usage** - Processor utilization percentage
- **Memory Usage** - RAM consumption
- **Disk Usage** - Storage utilization
- **Network Activity** - Transfer rates (upload/download)

Each metric card shows:
- Current value with percentage
- 24-hour sparkline graph
- Visual indicators for quick status assessment

## Real-Time Monitoring

### Update Intervals

Different metrics update at optimized intervals:

- **CPU**: Every 1 second (high variability)
- **Network**: Every 1 second (real-time activity)
- **Memory**: Every 60 seconds (slower changes)
- **Disk**: Every 60 seconds (gradual changes)

### Sparkline Graphs

Each metric displays a miniature graph showing the last 24 hours:

- **Time-aware scaling** - Accurate time representation
- **Gap detection** - Shows data collection interruptions
- **Auto-scaling** - Adjusts to value ranges
- **Click to expand** - Access detailed views

## Detailed Metric Views

Click any sparkline to open detailed analysis:

### Time Range Selection

Choose from multiple time windows:
- **1 Hour** - Fine-grained recent activity
- **6 Hours** - Short-term trends
- **24 Hours** - Daily patterns (default)
- **7 Days** - Weekly trends

### Chart Features

Detailed charts provide:
- **Large visualization** - 700x400 pixel graphs
- **Grid lines** - Easy value reading
- **Time labels** - Clear x-axis timestamps
- **Value labels** - Precise y-axis measurements
- **Filled area** - Visual weight for metrics

## Understanding Metrics

### CPU Usage

Shows total CPU utilization across all cores:

- **0-25%**: Light load, good headroom
- **25-50%**: Moderate load, healthy
- **50-75%**: Heavy load, monitor closely
- **75-100%**: Very heavy, may need optimization

**Common causes of high CPU**:
- Compilation or builds
- Media transcoding
- Database queries
- Runaway processes

### Memory Usage

Displays RAM consumption as percentage:

- **Includes**: Application memory, caches, buffers
- **Excludes**: Swap usage (monitored separately)
- **Healthy range**: 50-80% (Linux uses RAM for caching)

**Memory optimization tips**:
- Set container memory limits
- Monitor for memory leaks
- Use memory-efficient applications

### Disk Usage

Shows primary disk utilization:

- **Path**: Root filesystem (`/`)
- **Includes**: System, applications, data
- **Warning threshold**: 80% full
- **Critical threshold**: 90% full

**Disk management**:
- Regular cleanup of unused containers
- Archive old logs
- Use volume mounts for large data

### Network Activity

Displays current transfer rates:

- **Download rate**: Incoming traffic
- **Upload rate**: Outgoing traffic
- **Units**: Automatically scaled (B/s, KB/s, MB/s, GB/s)
- **Combined graph**: Total network activity

**Network patterns**:
- Spikes during updates/downloads
- Steady streams for media servers
- Periodic bursts for backups

## Performance Optimization

OnTree's monitoring system is optimized for minimal overhead:

### Caching

- **5-minute cache** for generated graphs
- **Intelligent invalidation** on new data
- **Reduced CPU usage** from rendering

### Efficient Queries

- **Batch data fetching** for multiple metrics
- **Indexed timestamps** for fast retrieval
- **Automatic data cleanup** after 7 days

### Resource Usage

The monitoring system itself uses:
- **< 1% CPU** for data collection
- **< 50MB RAM** for caching
- **< 100MB disk** for 7-day history

## Using Monitoring Data

### Identifying Issues

Watch for these patterns:

1. **CPU Spikes**
   - Sudden jumps indicate new processes
   - Sustained high usage suggests optimization needed

2. **Memory Leaks**
   - Gradually increasing memory without drops
   - Eventually leads to system slowdown

3. **Disk Fill**
   - Steady increase toward capacity
   - Can cause application failures

4. **Network Anomalies**
   - Unexpected high traffic
   - Could indicate issues or attacks

### Capacity Planning

Use historical data for planning:

- **Peak usage times** - Scale resources accordingly
- **Growth trends** - Predict future needs
- **Bottlenecks** - Identify limiting factors

### Performance Tuning

Monitoring helps optimize applications:

1. **Set resource limits** based on actual usage
2. **Schedule intensive tasks** during low-usage periods
3. **Identify inefficient** applications for replacement

## Integration with Apps

### Container Metrics

While system monitoring shows host metrics, container-specific monitoring is planned:

- Per-container CPU usage
- Individual memory consumption
- Container network traffic
- Storage per application

### Alerts and Notifications

Future features will include:

- Threshold-based alerts
- Email/webhook notifications
- Custom alert rules
- Integration with monitoring stacks

## Advanced Monitoring

### Exporting Data

Access raw monitoring data:

```bash
# SQLite database location
/data/ontree.db

# Query example
SELECT * FROM system_vital_logs 
WHERE timestamp > datetime('now', '-1 hour')
ORDER BY timestamp DESC;
```

### Custom Dashboards

Integrate with external tools:

- **Prometheus** - Export metrics endpoint
- **Grafana** - Rich visualization
- **InfluxDB** - Time-series storage

### API Access

Programmatic access to metrics:

```bash
# Get current CPU usage
curl http://localhost:8080/api/monitoring/cpu

# Get historical data
curl http://localhost:8080/api/monitoring/cpu?range=24h
```

## Configuration

### Enable/Disable Monitoring

In `config.toml`:
```toml
# Monitoring configuration
monitoring_enabled = true
monitoring_interval = 60  # seconds
monitoring_retention = 7  # days
```

Via environment:
```bash
MONITORING_ENABLED=false ontree-server
```

### Performance Tuning

Adjust monitoring for your needs:

```toml
# Reduce frequency for low-end systems
monitoring_interval = 300  # 5 minutes

# Increase retention for long-term analysis
monitoring_retention = 30  # 30 days

# Disable specific metrics
disable_network_monitoring = true
```

## Troubleshooting

### No Data Showing

1. **Check monitoring is enabled** in configuration
2. **Verify data collection** in logs
3. **Check database permissions**
4. **Ensure sufficient disk space**

### Incorrect Values

1. **Verify system tools** are installed (iostat, vmstat)
2. **Check container permissions** for /proc access
3. **Compare with system tools** for validation

### Performance Impact

If monitoring impacts performance:

1. **Increase collection interval**
2. **Reduce retention period**
3. **Disable unnecessary metrics**
4. **Check for resource constraints**

## Best Practices

### Regular Review

- **Daily**: Check for anomalies
- **Weekly**: Review trends
- **Monthly**: Capacity planning

### Baseline Establishment

- Document normal operating ranges
- Note typical peak times
- Track seasonal variations

### Proactive Monitoring

- Set up alerts for thresholds
- Plan maintenance windows
- Scale before hitting limits

The monitoring system is your window into OnTree's performance, helping you maintain a healthy, efficient environment for all your applications.