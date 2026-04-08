---
id: devops-infra
name: DevOps & Infrastructure
triggers:
  keywords: [ci, cd, docker, kubernetes, deploy, pipeline, monitoring, terraform]
  feature_patterns: ["*deploy*", "*infra*", "*pipeline*", "*ci*", "*monitor*"]
applies_to: [architect, coder]
---

## When to apply

This specialist is relevant when the project involves CI/CD pipelines, containerization, deployment automation, infrastructure-as-code, or monitoring/observability. Look for features involving deployment, environments, containers, or operational concerns.

## Guidance for architect

- Define the deployment topology early: single server, container orchestration, serverless, edge
- Plan environment promotion: dev → staging → production with consistent configuration management
- Choose between pull-based (GitOps, ArgoCD) and push-based (CI/CD pipeline) deployment models
- Design health checks and readiness probes for every service
- Plan monitoring with the three pillars: metrics, logs, traces
- Define rollback strategy before the first deployment — automated vs. manual, full vs. partial
- Separate infrastructure concerns (networking, storage) from application concerns (config, secrets)

## Guidance for coder

- Write Dockerfiles with multi-stage builds — keep production images minimal
- Pin dependency versions in infrastructure definitions (Terraform providers, Docker base images, package versions)
- Use environment variables for configuration that changes between environments
- Make builds reproducible: lock files committed, deterministic build steps, no floating tags
- Write CI pipeline steps as idempotent operations — safe to retry on failure
- Include linting and security scanning in CI before deployment steps
- Configure log output as structured JSON for easy parsing by log aggregation tools
- Set resource limits (CPU, memory) for containers to prevent noisy-neighbor issues

## Review checklist

- [ ] Deployment topology documented
- [ ] Health checks and readiness probes defined
- [ ] Rollback procedure documented and tested
- [ ] CI pipeline includes lint, test, security scan stages
- [ ] Secrets managed via environment variables or secrets manager
- [ ] Docker images use multi-stage builds and minimal base images
- [ ] Monitoring covers metrics, logs, and traces
- [ ] Resource limits set for all containers
