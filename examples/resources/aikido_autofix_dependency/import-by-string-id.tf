# Dependency AutoFix settings are workspace-wide, so the import ID is ignored.
# The conventional value is "dependency".
# Note: importing fails if AutoFix is disabled for the whole workspace in Aikido.
import {
  to = aikido_autofix_dependency.this
  id = "dependency"
}
