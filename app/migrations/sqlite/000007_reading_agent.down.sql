DROP TABLE IF EXISTS reading_self_tests;
DROP TABLE IF EXISTS reading_run_memories;

ALTER TABLE reading_runs DROP COLUMN phase;
ALTER TABLE reading_runs DROP COLUMN synthesis_model_id;
ALTER TABLE reading_runs DROP COLUMN extraction_model_id;

ALTER TABLE tenants DROP COLUMN reading_synthesis_model_id;
ALTER TABLE tenants DROP COLUMN reading_extraction_model_id;
