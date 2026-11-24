DROP TABLE IF EXISTS reading_self_tests;
DROP TABLE IF EXISTS reading_run_memories;

ALTER TABLE reading_runs DROP COLUMN IF EXISTS phase;
ALTER TABLE reading_runs DROP COLUMN IF EXISTS synthesis_model_id;
ALTER TABLE reading_runs DROP COLUMN IF EXISTS extraction_model_id;

ALTER TABLE tenants DROP COLUMN IF EXISTS reading_synthesis_model_id;
ALTER TABLE tenants DROP COLUMN IF EXISTS reading_extraction_model_id;
