BEGIN;
ALTER TABLE modules DROP COLUMN bug_id;
DROP TABLE IF EXISTS bugs;
COMMIT;
