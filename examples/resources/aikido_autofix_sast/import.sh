# SAST and IaC AutoFix settings are workspace-wide, so the import ID is ignored.
# The conventional value is "sast".
# Note: importing fails if AutoFix is disabled for the whole workspace in Aikido.
terraform import aikido_autofix_sast.this "sast"
