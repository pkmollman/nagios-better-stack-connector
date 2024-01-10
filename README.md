# Nagios Notification to Better Stack Connector

This service tracks Nagios alerts and spins up Better Stack incidents accordingly.
Acking an incident in Nagios will update the Better Stack incident and vice versa.

## Configuration

### Connector

Provide the following environment variables to the process running the connector service.

```bash
# COSMOD DB
AZURE_COSMOS_ENDPOINT=''
AZURE_COSMOS_KEY=''
AZURE_COSMOS_DATABASE=''
AZURE_COSMOS_CONTAINER=''

# BETTER STACK
BETTER_STACK_API_KEY=''

# NAGIOS
NAGIOS_THRUK_API_USER=''
NAGIOS_THRUK_API_KEY=''
NAGIOS_THRUK_BASE_URL=''
NAGIOS_THRUK_SITE_NAME=''
```

### Nagios

Generate a Thruk API key for the connector service, and provide it in the environment variables.
Make your notification commands pipe to the provided nagios-client.sh. It uses curl to hit this connector service.

```bash
bash client.sh -u "https://your-connector.blah/api/nagios-event" -s "site name" -i 123 -c "cause" -n "service name" -h "host name"
```

### Better Stack

Generate an API key for the connector service, and provide it in the environment variables.

Make an outgoing webhook that hits this service on /api/better-stack-event.
It will send acks to Nagios via the Thruk api.
