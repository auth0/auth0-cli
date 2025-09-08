---
layout: default
parent: auth0 event-streams
has_toc: false
---
# auth0 event-streams update

Update an event.

To update interactively, use `auth0 events update` with no arguments.

To update non-interactively, supply the event id, name, status, subscriptions and configuration through the flags. An event stream type CANNOT be updated hence the configuration should match the schema based on the type of event stream. Configuration for `eventbridge` streams cannot be updated.

## Usage
```
auth0 event-streams update [flags]
```

## Examples

```
  auth0 events update <event-id>
  auth0 events update <event-id> --name my-event-stream
  auth0 events update <event-id> --name my-event-stream --status enabled
  auth0 events update <event-id> --name my-event-stream --status enabled --subscriptions "user.created,user.updated"
  auth0 events update <event-id> --name my-event-stream --status disabled --subscriptions "user.deleted" --configuration '{"aws_account_id":"325235643634","aws_region":"us-east-2"}'
  auth0 events update <event-id> --name my-event-stream --status enabled --subscriptions "user.created" --configuration '{"webhook_endpoint":"https://my-new-webhook.net","webhook_authorization":{"method":"bearer","token":"0909090909"}}
  auth0 events update <event-id> -n my-event-stream --status enabled -s "user.created" -c '{"webhook_endpoint":"https://my-new-webhook.net","webhook_authorization":{"method":"bearer","token":"987654321"}}
```


## Flags

```
  -c, --configuration string    Configuration of the Event Stream. Formatted as JSON. 
                                Webhook Example: {"webhook_endpoint":"https://my-webhook.net","webhook_authorization":{"method":"bearer","token":"123456789"}} 
                                Eventbridge Example: {"aws_account_id":"7832467231933","aws_region":"us-east-2"}
      --json                    Output in json format.
  -n, --name string             Name of the Event Stream.
      --status string           Status of the Event Stream. (enabled/disabled)
  -s, --subscriptions strings   Subscriptions of the Event Stream. Formatted as comma separated string. Eg. user.created,user.updated
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 event-streams create](auth0_event-streams_create.md) - Create a new event stream
- [auth0 event-streams delete](auth0_event-streams_delete.md) - Delete an event stream
- [auth0 event-streams deliveries](auth0_event-streams_deliveries.md) - Manage event stream deliveries
- [auth0 event-streams list](auth0_event-streams_list.md) - List your event streams
- [auth0 event-streams redeliver](auth0_event-streams_redeliver.md) - Retry one or more event deliveries for a given stream
- [auth0 event-streams redeliver-many](auth0_event-streams_redeliver-many.md) - Bulk retry failed event deliveries using filters
- [auth0 event-streams show](auth0_event-streams_show.md) - Show an event stream
- [auth0 event-streams stats](auth0_event-streams_stats.md) - View delivery stats for an event stream
- [auth0 event-streams trigger](auth0_event-streams_trigger.md) - Trigger a test event for an event stream
- [auth0 event-streams update](auth0_event-streams_update.md) - Update an event


