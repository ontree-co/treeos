# Cache Package

Simple in-memory cache implementation with TTL support for monitoring performance optimization.

## Features
- Thread-safe in-memory caching
- Configurable TTL per cache instance
- Automatic cleanup of expired entries
- Simple Get/Set interface

## Usage
Used by monitoring handlers to cache generated sparkline SVGs for 5 minutes, reducing repeated calculations.