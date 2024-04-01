# Nagios Notifications to Better Stack Incident Connector

This service tracks Nagios alerts and spins up Better Stack incidents accordingly.

When a Host/Service goes down in Nagios, a new incident is created in Better Stack.

Acking an incident in Nagios will update the Better Stack incident and vice versa.

When a Host/Service comes back up in Nagios, the corresponding incident in Better Stack is resolved.

**Univerity of Oregon** Connector Status:
[![Better Stack Badge](https://uptime.betterstack.com/status-badges/v1/monitor/10qx4.svg)](https://uptime.betterstack.com/?utm_source=status_badge)

## Configuration

**Table of Contents**

- [Systemd](#systemd)
- [Database](#database)
- [Better Stack](#betterstack)
- [Nagios](#nagios)

### Systemd

Your systemd unit file should look something like this:

```
[Unit]
Description=Nagios Better Stack Connector
Wants=basic.target
After=basic.target network.target

[Service]
EnvironmentFile=/etc/nbsc/nbsc
ExecStart=/opt/nbsc/nbsc

[Install]
WantedBy=multi-user.target
```

The service is configured using environment variables.
In the above example, the service reads its environment variables from /etc/nbsc/nbsc. Which should look something like this:

```
# BETTER STACK
BETTER_STACK_API_KEY=12345asdfg
BETTER_STACK_DEFAULT_CONTACT_EMAIL=someone@acme.com

# NAGIOS
NAGIOS_THRUK_API_USER=someone
NAGIOS_THRUK_API_KEY=12345asdfg
NAGIOS_THRUK_BASE_URL=https://some-nagios-server.acme.com
NAGIOS_THRUK_SITE_NAME=some-nagios-site

# SQLITE
SQLITE_DB_PATH=/opt/nbsc/events.db
SQLITE_DB_BACKUP_DIR_PATH=/opt/nbsc/backups
SQLITE_DB_BACKUP_FREQUENCY_MINUTES=60
```

### Database

Currently the only supported database is SQLite.
The `database.DatabaseClient` interface is used to interact with the database, so it should be easy to add support for other databases.

SQLite require the following environment variables:

```
# where to store the live database, you'll need to create the parent directory
SQLITE_DB_PATH=/opt/nbsc/events.db

# directory to store backups, you'll need to create this directory
SQLITE_DB_BACKUP_DIR_PATH=/opt/nbsc/backups

# how often to backup the database
SQLITE_DB_BACKUP_FREQUENCY_MINUTES=60
```

### BetterStack

Generate an API key for the connector service, and provide it in the connector service environment variables, along with a default contact to label incident interactions with, like so:

```
# API key to authenticate with Better Stack
BETTER_STACK_API_KEY=12345asdfg

# Default contact email to use when creating, ACKing, or resolving incidents
BETTER_STACK_DEFAULT_CONTACT_EMAIL=someone@acme.com
```

Make an outgoing webhook that hits the connector service via POST at /api/better-stack-event.
It will send incident acks back to Nagios via the Thruk api.

Take note of the notification policies you would like nagios to use, and provide it in your nagios notification commands.

### Nagios

Generate a Thruk API key for the connector service, and provide it in the connector service environment variables, along with the base url for nagios, and site name, like so:

```
# Default interaction user for Thruk API
NAGIOS_THRUK_API_USER=someone

# API key to authenticate with Thruk
NAGIOS_THRUK_API_KEY=12345asdfg

# Base url for Thruk
NAGIOS_THRUK_BASE_URL=https://some-nagios-server.acme.com

# Nagios site name
NAGIOS_THRUK_SITE_NAME=some-nagios-site
```

Make your notification commands provided nbsc-client.py. It uses python3 with requests, argparse and json to relay the notification to the connector service.

The notification command should look something like this:

```
define command {
  command_name    notify-by-betterstack
  command_line    /usr/bin/python3 $USER2$/nbsc-client.py --url 'https://is-nagios-bsc-p.uoregon.edu/api/nagios-event' --site-name 'some-site' --problem-id '$SERVICEPROBLEMID$' --problem-content '$SERVICEOUTPUT$' --service-name '$SERVICEDESC$' --host-name '$HOSTNAME$' --notification-type '$NOTIFICATIONTYPE$' --policy-id '12345' --interacting-user '$SERVICEACKAUTHOR$'
}
```
