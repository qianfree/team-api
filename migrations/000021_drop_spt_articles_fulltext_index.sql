-- +goose Up
-- 帮助中心文章搜索从 to_tsvector 全文检索改为 ILIKE 模糊匹配：
-- PostgreSQL 'simple' 分词配置不做中文分词，连续中文被切成单个长 token，
-- 中文关键词几乎无法命中（帮助文档以中文内容为主）。该 GIN 索引不再被查询使用，删除以释放空间。

DROP INDEX IF EXISTS idx_spt_articles_search;

-- +goose Down
-- 恢复全文检索索引（仅回滚迁移时使用，此时搜索已退回 to_tsvector 方案）
CREATE INDEX idx_spt_articles_search ON spt_articles USING gin (to_tsvector('simple'::regconfig, (((COALESCE(title, ''::character varying))::text || ' '::text) || COALESCE(content, ''::text))));
