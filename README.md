# Nagios Notification to Better Stack Connector

This service tracks Nagios alerts and spins up Better Stack incidents accordingly.
Acking an incident in Nagios will update the Better Stack incident and vice versa.

## Configuration

### Database

Currently the only supported database is Azure CosmosDB. I plan to add sqlite and possibly others.

#### Azure CosmosDB

Create a nosql DB and provide the variables mentioned in the connector service environment variables.

### Better Stack

Generate an API key for the connector service, and provide it in the connector service environment variables.

Make an outgoing webhook that hits the connector service via POST at /api/better-stack-event.
It will send acks to Nagios via the Thruk api.

Take note of the notification policies you would like nagios to use, and provide it in the connector service environment variables.

### Nagios

Generate a Thruk API key for the connector service, and provide it in the connector service environment variables.
Make your notification commands pipe to the provided nagios-client.sh. It uses curl to hit this connector service.

```bash
command_name    notify-by-betterstack
command_line    /bin/bash $USER2$/notify-by-betterstack.sh -u 'https://your-connector-url.blah/api/nagios-event' -s 'nagios-site-name' -i '$SERVICEPROBLEMID$' -c '$SERVICEOUTPUT$' -n '$SERVICEDESC$' -h '$HOSTNAME$' -t '$NOTIFICATIONTYPE$'
```

### Connector Service

Provide the following environment variables to the process running the connector service.

```bash
# COSMOS DB
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

## TODO

- [ ] pass nagios ack comment to better stack
- [ ] add support for host notifications
- [ ] handle multiple notifications for same problem
- [ ] handle multiple fast crit to recovers from nagios....
- [ ] make unit file and stuff
