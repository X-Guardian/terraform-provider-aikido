# Notify Slack when a new issue is found
resource "aikido_webhook" "new_issues" {
  target_url = "https://hooks.slack.com/services/xxx/yyy/zzz"
  event_type = "issue.open.created"
}

# Alert on Zen attacks
resource "aikido_webhook" "attacks" {
  target_url = "https://hooks.slack.com/services/xxx/yyy/zzz"
  event_type = "zen.attack"
}
