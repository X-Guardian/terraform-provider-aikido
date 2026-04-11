resource "aikido_custom_rule" "no_hardcoded_secrets" {
  semgrep_rule = <<-EOT
    rules:
      - id: no-hardcoded-api-key
        patterns:
          - pattern: $KEY = "..."
          - metavariable-regex:
              metavariable: $KEY
              regex: (api_key|secret|token)
        message: Hardcoded secret detected
        severity: ERROR
        languages: [javascript, typescript]
  EOT

  issue_title = "Hardcoded API key detected"
  tldr        = "A hardcoded API key or secret was found in the source code."
  how_to_fix  = "Use environment variables or a secrets manager instead of hardcoding credentials."
  priority    = 85
  language    = "JS"
}
