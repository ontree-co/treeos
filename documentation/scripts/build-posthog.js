#!/usr/bin/env node

const fs = require('fs');
const path = require('path');

// Get environment variables
const POSTHOG_KEY = process.env.PUBLIC_POSTHOG_KEY || '';
const POSTHOG_HOST = process.env.PUBLIC_POSTHOG_HOST || 'https://eu.i.posthog.com';

// Read template
const templatePath = path.join(__dirname, '..', 'static', 'posthog-init.js.template');
const outputPath = path.join(__dirname, '..', 'static', 'posthog-init.js');

let template = fs.readFileSync(templatePath, 'utf8');

// Replace placeholders
template = template.replace('__POSTHOG_KEY__', POSTHOG_KEY);
template = template.replace('__POSTHOG_HOST__', POSTHOG_HOST);

// Write output
fs.writeFileSync(outputPath, template);

console.log('PostHog script built successfully');
console.log(`  Key: ${POSTHOG_KEY ? POSTHOG_KEY.substring(0, 10) + '...' : '(not set)'}`);
console.log(`  Host: ${POSTHOG_HOST}`);