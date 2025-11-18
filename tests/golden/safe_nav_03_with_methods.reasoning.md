---
title: "🧭 Safe navigation with method calls on optional receivers"
category: "Operators"
category_order: 90
subcategory: "Safe Navigation"
test_id: "safe_nav_03_with_methods"
order: 3
complexity: "advanced"
feature: "🧭 safe-navigation"
phase: "Phase 2.7"
status: "implemented"
description: "Validates safe navigation operator with method invocations on potentially nil receivers"
summary: "?. with method calls"
code_reduction: 65
feature_file: "null-safety.md"
related_tests:
  - "safe_nav_01_basic"
tags:
  - "safe-navigation"
  - "methods"
keywords:
  - "?. operator"
  - "method calls"
  - "nil receivers"
---

# Test Reasoning: safe_nav_03_with_methods

Safe navigation with methods prevents panics when calling methods on potentially nil receivers.

**Last Updated**: 2025-11-17
