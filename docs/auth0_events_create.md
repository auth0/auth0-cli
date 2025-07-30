---
layout: default
parent: auth0 events
has_toc: false
---
# auth0 events create

Create a new event stream.

To create interactively, use `auth0 events create` with no flags.

To create non-interactively, supply the event stream name, type, subscriptions and configuration through the flags.

## Usage
```
auth0 events create [flags]
```

## Examples

```
  auth0 events create
  auth0 events create --name my-event-stream --type eventbridge --subscriptions "user.created,user.updated" --configuration '{"aws_account_id":"325235643634","aws_region":"us-east-2"}'
  auth0 events create --name my-event-stream --type webhook --subscriptions "user.created,user.deleted" --configuration '{"webhook_endpoint":"https://mywebhook.net","webhook_authorization":{"method":"bearer","token":"123456789"}}'
  auth0 events create -n my-event-stream -t webhook -s "user.created,user.deleted" -c '{"webhook_endpoint":"https://mywebhook.net","webhook_authorization":{"method":"bearer","token":"123456789"}}'
```


## Flags

```
  -c, --configuration string    Configuration of the Event Stream. Formatted as JSON. 
                                Webhook Example: {"webhook_endpoint":"https://my-webhook.net","webhook_authorization":{"method":"bearer","token":"123456789"}} 
                                Eventbridge Example: {"aws_account_id":"7832467231933","aws_region":"us-east-2"}
      --json                    Output in json format.
  -n, --name string             Name of the Event Stream.
  -s, --subscriptions strings   Subscriptions of the Event Stream. Formatted as comma separated string. Eg. user.created,user.updated
  -t, --type string             Type of the Event Stream. Eg: webhook, eventbridge etc
```


## Inherited Flags

```
      --debug           Enable debug mode.
      --no-color        Disable colors.
      --no-input        Disable interactivity.
      --tenant string   Specific tenant to use.
```


## Related Commands

- [auth0 events create](auth0_events_create.md) - Create a new event stream
- [auth0 events delete](auth0_events_delete.md) - Delete an event stream
- [auth0 events deliveries](auth0_events_deliveries.md) - Manage event stream deliveries
- [auth0 events list](auth0_events_list.md) - List your event streams
- [auth0 events redeliver](auth0_events_redeliver.md) - Retry one or more event deliveries for a given stream
- [auth0 events redeliver-many](auth0_events_redeliver-many.md) - Bulk retry failed event deliveries using filters
- [auth0 events show](auth0_events_show.md) - Show an event stream
- [auth0 events stats](auth0_events_stats.md) - View delivery stats for an event stream
- [auth0 events trigger](auth0_events_trigger.md) - Trigger a test event for an event stream
- [auth0 events update](auth0_events_update.md) - Update an event


