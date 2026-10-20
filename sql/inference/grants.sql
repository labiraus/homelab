-- Run explicitly as the migration owner; provision this login/password separately.
REVOKE ALL ON SCHEMA inference FROM PUBLIC;
GRANT USAGE ON SCHEMA inference TO inference_runtime;
GRANT SELECT ON inference.schema_version,inference.attempt TO inference_runtime;
GRANT INSERT ON inference.attempt TO inference_runtime;
GRANT UPDATE(settled_micros,input_tokens,output_tokens,finished_at) ON inference.attempt TO inference_runtime;
