Output api-command.gif

Set FontSize 24
Set Width 1200
Set Height 600

#---
Type@75ms `auth0 api "tenants/settings"`
Sleep 200ms
Enter
Sleep 2000ms


Type@50ms `auth0 api "tenants/settings" | jq .enabled_locales`
Sleep 200ms
Enter
Sleep 2500ms

Type@75ms `auth0 api post connections --data '{"name":"github-conn","strategy":"github"}'`
Sleep 750ms
Enter
Sleep 2500ms