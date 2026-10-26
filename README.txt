Agiparel v5.2 PLANNING ARTIFACTS ONLY — NO EXECUTABLE CODE

Documents update the normative requirements and backlog, preserving full v5.1 content.

To install into existing repository after review:
  cd ~/agiparel
  unzip -o ~/Downloads/agiparel_plans_v5.2.zip -d .
  git add docs/Agiparel_Master_PRD_ARD_v5.2.md docs/Agiparel_Master_Epics_Stories_Tasks_v5.2.md docs/Agiparel_Implementation_Plan_v5.2.md
  git diff --cached --stat

This ZIP does not alter services, compose or production data.

DEV test convention (no DEV-TENANT):
  AGIPAREL_OPERATOR_USER=dev
  AGIPAREL_OPERATOR_PASSWORD=1212
  AGIPAREL_OPERATOR_TENANT=dev
