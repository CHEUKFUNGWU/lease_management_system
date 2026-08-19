-- 056_template_copy_lineage.sql — PRD S3-4 模板复制谱系
-- Copying a template produces a new version in the copy's own lineage
-- (new lineage starts at version 1, same-name copy continues the source
-- lineage at version+1); copied_from keeps the audit path explicit, the
-- same way fpna_plan_versions.prior_version_id does.

ALTER TABLE fin_statement_templates ADD COLUMN IF NOT EXISTS copied_from UUID REFERENCES fin_statement_templates(id);
CREATE INDEX IF NOT EXISTS idx_fin_statement_templates_copied_from
    ON fin_statement_templates(copied_from);
