# Unreleased
### Changelog
* feat(docker): manage shared docker-compose env vars via a single conf/docker/.env file
  * Credentials, DB settings, SMTP, and log levels are now injected through `${VAR}` interpolation from `conf/docker/.env` instead of being hardcoded inline or spread across per-service env_file files.
  * Added `conf/docker/.env.example` template and gitignored the real `conf/docker/.env`.
  * The `infra` commands now fail with a clear message when `conf/docker/.env` is missing.
  * Copy step: `cp conf/docker/.env.example conf/docker/.env` then set the required values.

# v0.1.0 (2023.12.14.)
### Changelog
* cm-mayfly public (Based on Docker Compose)

### Feature
* pull: Download the Cloud-Migrator container images to your local image store
* run: Run Cloud-Migrator containers to drive the Cloud-Migrator system
* info: Display the status of Cloud-Migrator containers and the status of images
* stop: Stop the Cloud-Migrator system by stopping the Cloud-Migrator containers
* remove: Remove the Cloud-Migrator container (+ volumes, images)
