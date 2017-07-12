*Cron for Docker*

Schedule jobs inside containers with Docker labels.

Following labels are supported:
`cron.<job-id>.schedule=*/5 * * * * *` - cron expression
`cron.<job-id>.command=/bin/sh -c echo hello` - command to run

## Installation
To run dron: `docker run -d -v /var/run/docker.sock:/var/run/docker.sock
quay.io/stepanstipl/dron:latest`

## TODO:
- Configure prefix
- Publish this on GitHub
- Tests
