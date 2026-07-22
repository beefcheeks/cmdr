-- Enlistment efforts: group cross-repo enlistments under a leader-side
-- .agents/enlistments/<effort> folder and track resume/amend metadata.
ALTER TABLE delegations ADD COLUMN effort TEXT NOT NULL DEFAULT '';
ALTER TABLE delegations ADD COLUMN slug TEXT NOT NULL DEFAULT '';
ALTER TABLE delegations ADD COLUMN leader_cwd TEXT NOT NULL DEFAULT '';
ALTER TABLE delegations ADD COLUMN artifact_path TEXT NOT NULL DEFAULT '';
ALTER TABLE delegations ADD COLUMN debrief_path TEXT NOT NULL DEFAULT '';
