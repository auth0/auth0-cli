---
layout: default
parent: auth0 event-streams
has_toc: false
---
# auth0 event-streams create

Create a new event stream.

To create interactively, use `auth0 event-streams create` with no flags.

To create non-interactively, supply the event stream name, type, subscriptions and configuration through the flags. 
Only deployed actions can be used for action configuration

Use '--schema' to print the request payload schema and exit.
Use '--data' to supply the full JSON payload. It is sent as-is and validated by the API.

## Usage
```
auth0 event-streams create [flags]
```

## Examples

```
  auth0 event-streams create
  auth0 event-streams create --name my-event-stream --type eventbridge --subscriptions "user.created,user.updated" --configuration '{"aws_account_id":"325235643634","aws_region":"us-east-2"}'
  auth0 event-streams create --name my-event-stream --type webhook --subscriptions "user.created,user.deleted" --configuration '{"webhook_endpoint":"https://mywebhook.net","webhook_authorization":{"method":"bearer","token":"123456789"}}'
  auth0 event-streams create --name my-event-stream --type action --subscriptions "user.created" --configuration '{"action_id":"b053e4ca-8b14-4233-b615-bd3bdb353977"}'
  auth0 event-streams create -n my-event-stream -t webhook -s "user.created,user.deleted" -c '{"webhook_endpoint":"https://mywebhook.net","webhook_authorization":{"method":"bearer","token":"123456789"}}'
  auth0 event-streams create --schema
  auth0 event-streams create --data '{"name":"my-event-stream","subscriptions":[{"event_type":"user.created"}],"destination":{"type":"eventbridge","configuration":{"aws_account_id":"325235643634","aws_region":"us-east-2"}}}'
  auth0 event-streams create --data @event-stream.json
```


## Flags

```
  -c, --configuration string    Configuration of the Event Stream. Formatted as JSON. 
                                Webhook Example: {"webhook_endpoint":"https://my-webhook.net","webhook_authorization":{"method":"bearer","token":"123456789"}} 
                                Eventbridge Example: {"aws_account_id":"7832467231933","aws_region":"us-east-2"}
      --data string             JSON payload for the operation, as a JSON string or file path (@file.json). Can also be piped via stdin.
      --json                    Output in json format.
      --json-compact            Output in compact json format.
  -n, --name string             Name of the Event Stream.
      --schema                  Print the request payload schema for this command and exit. Use with --json or --json-compact for machine-readable output.
  -s, --subscriptions strings   Subscriptions of the Event Stream. Formatted as comma separated string. Eg. user.created,user.updated
  -t, --type string             Type of the Event Stream. Eg: webhook, eventbridge etc
```


## Inherited Flags

```
      --agent-mode      Output JSON, disable prompts and colors. Auto-enabled for AI agents; set AUTH0_AGENT_MODE=false to disable.
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
- [auth0 event-streams subscribe](auth0_event-streams_subscribe.md) - Subscribe to live events via Server-Sent Events (SSE)
- [auth0 event-streams trigger](auth0_event-streams_trigger.md) - Trigger a test event for an event stream
- [auth0 event-streams update](auth0_event-streams_update.md) - Update an event stream


